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

test('JSON clients preserve public login failures, encode bodies, and retain caller cancellation', async () => {
    const requests = [], logouts = [];
    let status = 200;
    const context = vm.createContext({
        AbortSignal, document: {documentElement: {lang: 'ja-JP'}},
        logout: reason => logouts.push(reason),
        localizedResponseError: async response => Object.assign(new Error('localized'), {status: response.status}),
        captchaFetch: async (url, options) => {
            requests.push({url, options});
            return {ok: status < 400, status, json: async () => ({saved: true})};
        }
    });
    const source = readFileSync(new URL('../js/api.js', import.meta.url), 'utf8');
    vm.runInContext(source.replace(/^import .*;$/gm, '').replace(/^export /gm, ''), context);
    const privateClient = context.createJSONClient('/private', 'failed');
    const publicClient = context.createJSONClient('/public', 'failed', {publicRequest: true, timeoutMS: 15000});
    const controller = new AbortController();
    await publicClient('/code', {json: {email: 'alice@example.com'}, signal: controller.signal});
    const options = requests[0].options;
    assert.equal(requests[0].url, '/public/code');
    assert.equal(options.method, 'POST');
    assert.equal(options.credentials, 'include');
    assert.equal(options.cache, 'no-store');
    assert.equal(options.headers['Accept-Language'], 'ja-JP');
    assert.equal(options.headers['Content-Type'], 'application/json');
    assert.deepEqual(JSON.parse(options.body), {email: 'alice@example.com'});
    controller.abort();
    assert.equal(options.signal.aborted, true);
    status = 401;
    await assert.rejects(publicClient('/confirm'), {message: 'localized', status: 401});
    assert.deepEqual(logouts, []);
    status = 403;
    await assert.rejects(privateClient('/permission'), {message: 'localized', status: 403});
    assert.deepEqual(logouts, []);
    status = 204;
    assert.equal(await privateClient('/delete', {method: 'DELETE'}), null);
    status = 401;
    await assert.rejects(privateClient('/expired'), {message: 'Unauthorized'});
    assert.deepEqual(logouts, ['kicked']);
});

test('UI actions suppress double clicks and cancellation success while restoring failed buttons', async () => {
    const alerts = [];
    const context = vm.createContext({
        showAlert: (...args) => alerts.push(args), t: key => key,
        caughtErrorMessage: (_error, key) => key,
    });
    const source = readFileSync(new URL('../js/components/button.js', import.meta.url), 'utf8');
    vm.runInContext(source.replace(/^import .*;$/gm, '').replace(/^export /gm, ''), context);
    const button = {disabled: false}, feedback = {errorKey: 'failed', successKey: 'saved'};
    let calls = 0, release;
    const action = () => { calls++; return new Promise(resolve => { release = resolve; }); };
    const pending = context.runUIAction(button, action, feedback);
    await context.runUIAction(button, action, feedback);
    assert.equal(calls, 1);
    assert.equal(button.disabled, true);
    release(false);
    await pending;
    assert.equal(button.disabled, false);
    assert.deepEqual(alerts, []);
    await context.runUIAction(button, () => { throw new Error('private runtime details'); }, feedback);
    assert.deepEqual(alerts, [['failed', 'error']]);
    assert.equal(button.disabled, false);
    await context.runUIAction(button, () => undefined, feedback);
    assert.deepEqual(alerts.at(-1), ['saved', 'success']);
});
