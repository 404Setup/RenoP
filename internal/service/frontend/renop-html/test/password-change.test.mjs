/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('../js/password-change.js', import.meta.url), 'utf8')
    .replace(/^import .*;\r?\n/gm, '').replaceAll('export ', '');
const settle = () => new Promise(resolve => setImmediate(resolve));

function passwordUI(security) {
    const elements = new Map(), requests = [], listeners = new Map();
    let dialog, selectFactor, action;
    const context = vm.createContext({
        AbortController, DOMException, TextEncoder,
        window: {
            location: {pathname: '/account'},
            addEventListener: (type, listener) => listeners.set(type, listener),
            removeEventListener: type => listeners.delete(type),
        },
        document: {getElementById: id => elements.get(id)},
        createJSONClient: () => async (path, options) => {
            requests.push({path, options});
            if (path === 'profile/security') return security;
            if (path === 'profile/password/passkey/begin') return {challenge_id: 'challenge', options: {publicKey: {}}};
            if (path === 'password-reset/request') return {id: 'mail-job', ticket: 'private-ticket'};
            assert.fail(`unexpected request ${path}`);
        },
        putProto: async (path, _requestType, body, _responseType, options) => {
            requests.push({path, body, options});
            return {response: {ok: true}};
        },
        requestPasskeyAssertion: async () => ({id: 'credential', response: {signature: 'signed'}}),
        verifyProfileEmail: async (_email, receipt, options) => {
            assert.equal(receipt.ticket, 'private-ticket');
            return (await options.confirm('email-proof', new AbortController().signal)).ok;
        },
        runButtonAction: (_button, run) => action = run(),
        el: (tag, attributes = {}, ...children) => {
            const node = {tag, ...attributes, children, handlers: {}, value: '',
                addEventListener(type, fn) { this.handlers[type] = fn; },
                requestSubmit() { this.handlers.submit({preventDefault() {}}); }};
            if (node.id) elements.set(node.id, node);
            return node;
        },
        makeCustomSelect: (_choices, _current, select) => {
            selectFactor = select;
            return {querySelector: () => ({setAttribute() {}})};
        },
        RenopDialog: {show: options => new Promise(resolve => {
            dialog = options;
            elements.set(options.id, {close: result => { options.onClose(); resolve(result); }});
            elements.set('password-change-verify', {});
        })},
        StatusOk: {}, UpdatePasswordRequest: {}, t: key => key,
        localizedResponseError: async () => new Error('rejected'),
        caughtErrorMessage: () => 'localized-error', passkeyErrorMessage: () => 'passkey-error',
    });
    vm.runInContext(source, context);
    return {context, requests, elements, listeners,
        get dialog() { return dialog; }, select: value => selectFactor(value), submit: async () => {
            dialog.body.requestSubmit();
            await action;
        }};
}

test('password changes submit the selected live second-factor proof', async t => {
    for (const factor of ['totp', 'passkey', 'email']) await t.test(factor, async () => {
        const ui = passwordUI({totp_enabled: true, passkey_second_factor: true,
            email: 'primary@example.com', email_verification_required: true});
        const result = ui.context.changeProfilePassword('new-password');
        await settle();
        assert.equal(ui.requests.filter(item => item.body).length, 0, 'opening verification must not change the password');
        ui.select(factor);
        ui.elements.get('password-change-code').value = '123456';
        await ui.submit();
        assert.equal(await result, true);
        const submitted = ui.requests.find(item => item.body)?.body;
        assert.equal(submitted.new_password, 'new-password');
        assert.equal(submitted.factor, factor);
        if (factor === 'totp') assert.equal(submitted.totp_code, '123456');
        if (factor === 'passkey') {
            assert.equal(submitted.challenge_id, 'challenge');
            assert.equal(JSON.parse(new TextDecoder().decode(submitted.passkey_credential)).id, 'credential');
        }
        if (factor === 'email') {
            assert.equal(submitted.email_code, 'email-proof');
            assert.equal(ui.requests.find(item => item.path === 'password-reset/request').options.json.email, 'primary@example.com');
        }
        assert.equal(ui.elements.get('password-change-code').value, '');
        assert.equal(ui.listeners.size, 0);
    });
});

test('leaving password verification cancels without submitting a mutation', async () => {
    const ui = passwordUI({totp_enabled: true});
    const result = ui.context.changeProfilePassword('new-password');
    await settle();
    ui.listeners.get('popstate')();
    assert.equal(await result, false);
    assert.equal(ui.requests.filter(item => item.body).length, 0);
    assert.equal(ui.listeners.size, 0);
});
