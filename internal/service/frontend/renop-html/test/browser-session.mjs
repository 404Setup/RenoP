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

/** Connect to an explicitly selected local browser page for optional UI contracts. */
export async function connectBrowserPage(endpoint) {
    const pages = await (await fetch(endpoint + '/json/list')).json();
    const page = pages.find(value => value.type === 'page' && value.url.startsWith(process.env.RENOP_TEST_BROWSER_URL || 'http://127.0.0.1:18080'));
    assert.ok(page, 'Open the configured test page.');
    const socket = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
    let next = 0;
    const pending = new Map();
    socket.onmessage = event => {
        const value = JSON.parse(event.data);
        if (!value.id) return;
        const entry = pending.get(value.id);
        if (!entry) return;
        pending.delete(value.id);
        clearTimeout(entry.timer);
        value.error ? entry.reject(value.error) : entry.resolve(value.result);
    };
    const call = (method, params = {}) => new Promise((resolve, reject) => {
        const id = ++next;
        pending.set(id, {resolve, reject, timer: setTimeout(() => reject(new Error('Browser command timed out')), 10000)});
        socket.send(JSON.stringify({id, method, params}));
    });
    const evaluate = async expression => {
        const value = await call('Runtime.evaluate', {expression, returnByValue: true, awaitPromise: true});
        assert.equal(value.exceptionDetails, undefined);
        return value.result.value;
    };
    const key = async value => {
        await call('Input.dispatchKeyEvent', {type: 'keyDown', key: value});
        await call('Input.dispatchKeyEvent', {type: 'keyUp', key: value});
    };
    return {call, evaluate, key, close: () => {
        for (const entry of pending.values()) clearTimeout(entry.timer);
        pending.clear();
        socket.close();
    }};
}
