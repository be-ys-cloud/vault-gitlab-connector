import { helper as buildHelper } from '@ember/component/helper';

const SUPPORTED_AUTH_BACKENDS = [
    {
        type: 'token',
        typeDisplay: 'Token',
        description: 'Token authentication.',
        tokenPath: 'id',
        displayNamePath: 'display_name',
        formAttributes: ['token'],
    },
    {
        type: 'gitlab',
        typeDisplay: 'Gitlab',
        description: 'Gitlab authentication.',
        tokenPath: 'client_token',
        displayNamePath: ['metadata.username'],
        formAttributes: ['token', 'username', 'password'],
        showOrder: 1,
    },

];

export function supportedAuthBackends() {
    return SUPPORTED_AUTH_BACKENDS.sort((a, b) => (1 / b['showOrder'] || 0) - (1 / a['showOrder'] || 0));
}

export default buildHelper(supportedAuthBackends);
