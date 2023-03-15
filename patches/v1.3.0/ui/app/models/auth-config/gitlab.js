import { computed } from '@ember/object';
import DS from 'ember-data';

import AuthConfig from '../auth-config';
import fieldToAttrs from 'vault/utils/field-to-attrs';

const { attr } = DS;

export default AuthConfig.extend({
    baseURL: attr('string', {
        label: 'Base URL',
    }),
    minAccessLevel: attr('string', {
        label: 'Minimal Access Level',
        defaultValue: 'developer',
        possibleValues: ['none', 'guest', 'reporter', 'developer', 'maintainer', 'owner']
    }),
    appID: attr('string', {
        label: 'Oauth Application ID',
    }),
    appSecret: attr('string', {
        label: 'Oauth Application Secret',
    }),
    callbackURL: attr('string', {
        label: 'Oauth Callback URL',
    }),
    ciToken: attr('string', {
        label: 'CI token',
    }),

    fieldGroups: computed(function() {
        const groups = [{
            'Gitlab Options': ['baseURL', 'minAccessLevel', 'appID', 'appSecret', 'callbackURL', 'ciToken'],
        }, ];

        return fieldToAttrs(this, groups);
    }),

});