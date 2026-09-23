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

test('demo credentials only fill empty fields and stale mode responses cannot restore them', async () => {
    const fields = new Map([
        ['username', {value: ''}], ['password', {value: ''}],
        ['demo-banner', {hidden: true, textContent: ''}], ['login-form', {
            addEventListener() {
            }
        }],
    ]);
    const pending = [];
    const context = vm.createContext({
        document: {getElementById: id => fields.get(id)},
        window: {
            addEventListener() {
            }
        }, queueMicrotask,
        t: key => key, DemoInfo: {},
        fetchProto: () => new Promise(resolve => pending.push(resolve)),
    });
    vm.runInContext(readFileSync(new URL('../js/demo-mode.js', import.meta.url), 'utf8')
        .replace(/^import .*;\r?\n/gm, '').replaceAll('export ', ''), context);
    const mode = {enabled: true, temporary: false, username: 'admin', password: '12345678'};
    let load = context.initializeDemoMode();
    pending.shift()({response: {ok: true}, data: mode});
    await load;
    assert.equal(fields.get('username').value, 'admin');
    assert.equal(fields.get('password').value, '12345678');
    assert.equal(fields.get('demo-banner').hidden, false);
    fields.get('username').value = 'typed-account';
    fields.get('password').value = '';
    context.fillDemoCredentials();
    assert.equal(fields.get('password').value, '', 'never combine a typed identity with the demo password');
    const stale = context.initializeDemoMode();
    load = context.initializeDemoMode();
    pending.pop()({response: {ok: true}, data: {enabled: false}});
    await load;
    pending.shift()({response: {ok: true}, data: mode});
    await stale;
    fields.get('username').value = '';
    context.fillDemoCredentials();
    assert.equal(fields.get('username').value, '');
    assert.equal(fields.get('password').value, '');
    assert.equal(fields.get('demo-banner').hidden, true);
});
