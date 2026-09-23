/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {createJSONClient, putProto} from './api.js';
import {RenopDialog, runButtonAction} from './components.js';
import {passkeyErrorMessage, requestPasskeyAssertion} from './fido-utils.js';
import {verifyProfileEmail} from './profile-email-verification.js';
import {caughtErrorMessage, localizedResponseError} from './response-errors.js';
import {StatusOk, UpdatePasswordRequest} from './proto/index.js';
import {t} from './i18n.js';
import {el} from '@renop/ui/dom';
import {makeCustomSelect} from '@renop/ui/custom-select';

const request = createJSONClient('/api/auth/', 'profile.updatePasswordFailed');

/** Change a password using the account's live second-factor policy and a fresh proof. */
export async function changeProfilePassword(newPassword) {
    const controller = new AbortController(), route = window.location.pathname;
    const active = () => !controller.signal.aborted && window.location.pathname === route;
    const close = () => {
        controller.abort();
        document.getElementById('password-change-dialog')?.close(false);
    };
    const submit = async (proof, signal = controller.signal) => {
        const {response} = await putProto('/api/auth/profile/password', UpdatePasswordRequest,
            {new_password: newPassword, ...proof}, StatusOk, {signal});
        return response;
    };
    window.addEventListener('pagehide', close);
    window.addEventListener('popstate', close);
    window.addEventListener('authChanged', close);
    try {
        const security = await request('profile/security', {signal: controller.signal});
        if (!active()) return false;
        if (!security.totp_enabled && !security.passkey_second_factor) {
            const response = await submit({});
            if (!response.ok) throw await localizedResponseError(response, 'profile.updatePasswordFailed');
            return active();
        }
        const choices = [];
        if (security.totp_enabled) choices.push({value: 'totp', label: t('mfa.authenticator')});
        if (security.passkey_second_factor) choices.push({value: 'passkey', label: t('mfa.usePasskey')});
        if (security.email && security.email_verification_required) choices.push({
            value: 'email',
            label: t('profile.passwordEmailMethod')
        });
        let factor = choices[0].value;
        const code = el('input', {
            id: 'password-change-code', type: 'text', inputmode: 'numeric',
            autocomplete: 'one-time-code', pattern: '[0-9]{6}', minlength: '6', maxlength: '6'
        });
        const codeField = el('div', {class: 'account-field'}, el('label', {for: code.id}, t('mfa.code')), code);
        const error = el('p', {class: 'account-form-error', role: 'alert', hidden: true});
        const update = () => {
            codeField.hidden = factor !== 'totp';
            code.required = factor === 'totp';
            codeField.inert = factor !== 'totp';
            code.value = '';
            error.hidden = true;
        };
        const picker = makeCustomSelect(choices, factor, value => {
            factor = value;
            update();
        });
        picker.querySelector('button')?.setAttribute('aria-label', t('profile.passwordSecondFactor'));
        const form = el('form', {class: 'account-verification'},
            el('p', {}, t('profile.passwordSecondFactor')), picker, codeField, error);
        update();
        form.addEventListener('submit', event => {
            event.preventDefault();
            if (!active()) return;
            const method = factor;
            void runButtonAction(document.getElementById('password-change-verify'), async () => {
                form.inert = true;
                error.hidden = true;
                try {
                    if (method === 'email') {
                        const receipt = await request('password-reset/request', {
                            method: 'POST', json: {email: security.email}, signal: controller.signal,
                        });
                        if (!active()) return;
                        const changed = await verifyProfileEmail(security.email, receipt, {
                            titleKey: 'profile.changePasswordTitle', descriptionKey: 'profile.passwordEmailSent',
                            errorKey: 'profile.updatePasswordFailed',
                            confirm: (emailCode, signal) => submit({factor: 'email', email_code: emailCode}, signal),
                        });
                        if (changed && active()) document.getElementById('password-change-dialog')?.close(true);
                        return;
                    }
                    let proof = {factor: 'totp', totp_code: code.value};
                    if (method === 'passkey') {
                        const challenge = await request('profile/password/passkey/begin', {
                            method: 'POST', json: {}, signal: controller.signal,
                        });
                        if (!active()) return;
                        const credential = await requestPasskeyAssertion(challenge.options, {
                            signal: controller.signal, button: document.getElementById('password-change-verify'),
                        });
                        proof = {
                            factor: 'passkey', challenge_id: challenge.challenge_id,
                            passkey_credential: new TextEncoder().encode(JSON.stringify(credential))
                        };
                    }
                    const response = await submit(proof);
                    code.value = '';
                    if (!response.ok) throw await localizedResponseError(response, 'profile.updatePasswordFailed');
                    if (active()) document.getElementById('password-change-dialog')?.close(true);
                } catch (failure) {
                    if (!active()) return;
                    error.textContent = failure instanceof DOMException ? passkeyErrorMessage(failure)
                        : caughtErrorMessage(failure, 'profile.updatePasswordFailed');
                    error.hidden = false;
                } finally {
                    form.inert = false;
                }
            });
        });
        return await RenopDialog.show({
            id: 'password-change-dialog', title: t('profile.changePasswordTitle'), icon: 'fileKey',
            maxWidth: '480px', body: form,
            footer: [
                {text: t('common.cancel'), className: 'action-btn', onClick: close},
                {
                    id: 'password-change-verify',
                    text: t('mfa.verify'),
                    className: 'action-btn primary-btn',
                    onClick: () => form.requestSubmit()
                },
            ],
            onClose: () => {
                controller.abort();
                code.value = '';
            },
        }) === true;
    } catch (error) {
        if (!active()) return false;
        throw error;
    } finally {
        newPassword = '';
        controller.abort();
        window.removeEventListener('pagehide', close);
        window.removeEventListener('popstate', close);
        window.removeEventListener('authChanged', close);
    }
}
