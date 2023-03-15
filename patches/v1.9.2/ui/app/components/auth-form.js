import Ember from 'ember';
import { next } from '@ember/runloop';
import { inject as service } from '@ember/service';
import { match, alias, or } from '@ember/object/computed';
import { assign } from '@ember/polyfills';
import { dasherize } from '@ember/string';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { supportedAuthBackends } from 'vault/helpers/supported-auth-backends';
import { task, timeout } from 'ember-concurrency';
const BACKENDS = supportedAuthBackends();

/**
 * @module AuthForm
 * The `AuthForm` is used to sign users into Vault.
 *
 * @example ```js
 * // All properties are passed in via query params.
 *   <AuthForm @wrappedToken={{wrappedToken}} @cluster={{model}} @namespace={{namespaceQueryParam}} @redirectTo={{redirectTo}} @selectedAuth={{authMethod}}/>```
 *
 * @param wrappedToken=null {String} - The auth method that is currently selected in the dropdown.
 * @param cluster=null {Object} - The auth method that is currently selected in the dropdown. This corresponds to an Ember Model.
 * @param namespace=null {String} - The currently active namespace.
 * @param redirectTo=null {String} - The name of the route to redirect to.
 * @param selectedAuth=null {String} - The auth method that is currently selected in the dropdown.
 */

const DEFAULTS = {
    token: null,
    username: null,
    password: null,
    customPath: null,
};

export default Component.extend(DEFAULTS, {
    router: service(),
    auth: service(),
    flashMessages: service(),
    store: service(),
    csp: service('csp-event'),

    //  passed in via a query param
    selectedAuth: null,
    methods: null,
    cluster: null,
    redirectTo: null,
    namespace: null,
    wrappedToken: null,
    // internal
    oldNamespace: null,

    didReceiveAttrs() {
        this._super(...arguments);
        let {
            wrappedToken: token,
            oldWrappedToken: oldToken,
            oldNamespace: oldNS,
            namespace: ns,
            selectedAuth: newMethod,
            oldSelectedAuth: oldMethod,
        } = this;

        next(() => {
            if (!token && (oldNS === null || oldNS !== ns)) {
                this.fetchMethods.perform();
            }
            this.set('oldNamespace', ns);
            // we only want to trigger this once
            if (token && !oldToken) {
                this.unwrapToken.perform(token);
                this.set('oldWrappedToken', token);
            }
            if (oldMethod && oldMethod !== newMethod) {
                this.resetDefaults();
            }
            this.set('oldSelectedAuth', newMethod);
        });
    },

    didRender() {
        this._super(...arguments);
        // on very narrow viewports the active tab may be overflowed, so we scroll it into view here
        let activeEle = this.element.querySelector('li.is-active');
        if (activeEle) {
            activeEle.scrollIntoView();
        }

        next(() => {
            let firstMethod = this.firstMethod();
            // set `with` to the first method
            if (
                !this.wrappedToken &&
                ((this.fetchMethods.isIdle && firstMethod && !this.selectedAuth) ||
                    (this.selectedAuth && !this.selectedAuthBackend))
            ) {
                this.set('selectedAuth', firstMethod);
            }
        });
    },

    firstMethod() {
        let firstMethod = this.methodsToShow.firstObject;
        if (!firstMethod) return;
        // prefer backends with a path over those with a type
        return firstMethod.path || firstMethod.type;
    },

    resetDefaults() {
        this.setProperties(DEFAULTS);
    },

    selectedAuthIsPath: match('selectedAuth', /\/$/),
    selectedAuthBackend: computed(
        'wrappedToken',
        'methods',
        'methods.[]',
        'selectedAuth',
        'selectedAuthIsPath',
        function() {
            let { wrappedToken, methods, selectedAuth, selectedAuthIsPath: keyIsPath } = this;
            if (!methods && !wrappedToken) {
                return {};
            }
            if (keyIsPath) {
                return methods.findBy('path', selectedAuth);
            }
            return BACKENDS.findBy('type', selectedAuth);
        }
    ),

    providerName: computed('selectedAuthBackend.type', function() {
        if (!this.selectedAuthBackend) {
            return;
        }
        let type = this.selectedAuthBackend.type || 'token';
        type = type.toLowerCase();
        let templateName = dasherize(type);
        return templateName;
    }),

    hasCSPError: alias('csp.connectionViolations.firstObject'),

    cspErrorText: `This is a standby Vault node but can't communicate with the active node via request forwarding. Sign in at the active node to use the Vault UI.`,

    allSupportedMethods: computed('methodsToShow', 'hasMethodsWithPath', function() {
        let hasMethodsWithPath = this.hasMethodsWithPath;
        let methodsToShow = this.methodsToShow;
        return hasMethodsWithPath ? methodsToShow.concat(BACKENDS) : methodsToShow;
    }),

    hasMethodsWithPath: computed('methodsToShow', function() {
        return this.methodsToShow.isAny('path');
    }),
    methodsToShow: computed('methods', function() {
        let methods = this.methods || [];
        let shownMethods = methods.filter(m => BACKENDS.find(b => b.type.toLowerCase() === m.type.toLowerCase()));
        return shownMethods.length ? shownMethods : BACKENDS;
    }),

    unwrapToken: task(function*(token) {
        // will be using the Token Auth Method, so set it here
        this.set('selectedAuth', 'token');
        let adapter = this.store.adapterFor('tools');
        try {
            let response = yield adapter.toolAction('unwrap', null, { clientToken: token });
            if (response.data.authType) {
                this.set('selectedAuth', response.data.authType);
                this.set('selectedAuthBackend', BACKENDS.findBy('type', response.data.authType));
                this.set('token', response.data.token);
            } else  {
                this.set('selectedAuth', 'token');
                this.set('token', response.auth.client_token);
            }

            this.send('doSubmit');
        } catch (e) {
            this.set('error', `Token unwrap failed: ${e.errors[0]}`);
        }
    }).withTestWaiter(),

    fetchMethods: task(function*() {
        let store = this.store;
        try {
            let methods = yield store.findAll('auth-method', {
                adapterOptions: {
                    unauthenticated: true,
                },
            });
            this.set(
                'methods',
                methods.map(m => {
                    const method = m.serialize({ includeId: true });
                    return {
                        ...method,
                        mountDescription: method.description,
                    };
                })
            );
            next(() => {
                store.unloadAll('auth-method');
            });
        } catch (e) {
            this.set('error', `There was an error fetching Auth Methods: ${e.errors[0]}`);
        }
    }).withTestWaiter(),

    showLoading: or('isLoading', 'authenticate.isRunning', 'fetchMethods.isRunning', 'unwrapToken.isRunning'),

    handleError(e, prefixMessage = true) {
        this.set('loading', false);
        let errors;
        if (e.errors) {
            errors = e.errors.map(error => {
                if (error.detail) {
                    return error.detail;
                }
                return error;
            });
        } else {
            errors = [e];
        }
        let message = prefixMessage ? 'Authentication failed: ' : '';
        this.set('error', `${message}${errors.join('.')}`);
    },

    authenticate: task(function*(backendType, data) {
        let clusterId = this.cluster.id;
        try {
            if (backendType === 'okta') {
                this.delayAuthMessageReminder.perform();
            }
            let authResponse = yield this.auth.authenticate({ clusterId, backend: backendType, data });

            let { isRoot, namespace } = authResponse;
            let transition;
            let { redirectTo } = this;
            if (redirectTo) {
                // reset the value on the controller because it's bound here
                this.set('redirectTo', '');
                // here we don't need the namespace because it will be encoded in redirectTo
                transition = this.router.transitionTo(redirectTo);
            } else {
                transition = this.router.transitionTo('vault.cluster', { queryParams: { namespace } });
            }
            // returning this w/then because if we keep it
            // in the task, it will get cancelled when the component in un-rendered
            yield transition.followRedirects().then(() => {
                if (isRoot) {
                    this.flashMessages.warning(
                        'You have logged in with a root token. As a security precaution, this root token will not be stored by your browser and you will need to re-authenticate after the window is closed or refreshed.'
                    );
                }
            });
        } catch (e) {
            this.handleError(e);
        }
    }).withTestWaiter(),

    delayAuthMessageReminder: task(function*() {
        if (Ember.testing) {
            this.showLoading = true;
            yield timeout(0);
            return;
        }
        yield timeout(5000);
    }),

    actions: {
        doSubmit() {
            let passedData, e;
            if (arguments.length > 1) {
                [passedData, e] = arguments;
            } else {
                [e] = arguments;
            }
            if (e) {
                e.preventDefault();
            }
            let data = {};
            this.setProperties({
                error: null,
            });
            let backend = this.selectedAuthBackend || {};
            let backendMeta = BACKENDS.find(
                b => (b.type || '').toLowerCase() === (backend.type || '').toLowerCase()
            );
            let attributes = (backendMeta || {}).formAttributes || [];

            data = assign(data, this.getProperties(...attributes));
            if (passedData) {
                data = assign(data, passedData);
            }
            if (this.customPath || backend.id) {
                data.path = this.customPath || backend.id;
            }
            return this.authenticate.unlinked().perform(backend.type, data);
        },
        handleError(e) {
            if (e) {
                this.handleError(e, false);
            } else {
                this.set('error', null);
            }
        },
    },
});
