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
import {dirname, join} from 'node:path';
import test from 'node:test';
import vm from 'node:vm';
import {fileURLToPath} from 'node:url';

const frontendRoot = join(dirname(fileURLToPath(import.meta.url)), '..');

/**
 * Read one embedded frontend source.
 * @param {...string} parts - Path below the frontend root.
 * @returns {string} Source text.
 */
function source(...parts) {
    return readFileSync(join(frontendRoot, ...parts), 'utf8');
}

test('administrator notification composition is isolated from the message center', () => {
    const main = source('js', 'main.js');
    const messages = source('js', 'messages.js');
    const composer = source('js', 'notification-composer.js');
    assert.match(main, /initMessageCenter, openMessageCenter} from '\.\/messages\.js'/);
    assert.match(main, /initNotificationComposer, openNotificationComposer} from '\.\/notification-composer\.js'/);
    assert.match(main, /initNotificationComposer\(\)/);
    assert.doesNotMatch(messages, /message-compose|SendNotificationRequest|UserSearchResponse|cachedIsManager/);
    assert.match(composer, /export function initNotificationComposer/);
    assert.match(composer, /export function openNotificationComposer/);
    assert.match(composer, /if \(!cachedIsManager \|\| sending\) return/);
});

test('notification composition keeps bounded typed delivery and accessible recipient suggestions', () => {
    const composer = source('js', 'notification-composer.js');
    assert.match(composer, /makeCustomSelect\(severityOptions\(\), 'info'\)/);
    assert.match(composer, /createToggle\(false, handleBroadcastToggleChange\)/);
    assert.match(composer, /\/api\/messages\/admin\/users\?q=/);
    assert.match(composer, /recipientSuggestionVersion/);
    assert.match(composer, /role: 'option'/);
    assert.match(composer, /SendNotificationRequest/);
    assert.match(composer, /SendNotificationResponse/);
    assert.match(composer, /localizedResponseError\(response, 'messages\.sendFailed'\)/);
    assert.doesNotMatch(composer, /innerHTML\s*=\s*username/);
});

test('session targets reset on recipient changes and discard stale pages with reversible visibility', async () => {
    const requests = [], animations = [], timers = new Map();
    let nextTimer = 0, value = '', options = [];
    const select = {
        querySelector: () => ({
            setAttribute() {
            }
        }),
        getValue: () => value,
        setValue: next => {
            value = next;
        },
        setOptions: next => {
            options = next;
            if (!options.some(item => item.value === value)) value = '';
        },
    };
    const context = vm.createContext({
        AbortController, encodeURIComponent,
        makeCustomSelect: () => select,
        expandElement: () => animations.push('expand'), collapseElement: () => animations.push('collapse'),
        SessionList: {}, t: key => key, formatTimestamp: value => String(value),
        fetchProto: (url, type, options) => new Promise(resolve => requests.push({url, ...options, resolve})),
        setTimeout: fn => {
            timers.set(++nextTimer, fn);
            return nextTimer;
        },
        clearTimeout: id => timers.delete(id),
    });
    vm.runInContext(source('js', 'notification-sessions.js').replace(/^import .*;\r?\n/gm, '').replaceAll('export ', ''), context);
    const field = {}, status = {};
    const target = context.createNotificationSessions(field, {
        replaceChildren() {
        }
    }, status);
    const flush = () => new Promise(resolve => setImmediate(resolve));
    const start = () => {
        const pending = [...timers.values()];
        timers.clear();
        pending.forEach(fn => fn());
    };
    target.update(['alice'], true);
    start();
    assert.equal(target.value(['alice']), '');
    target.update(['bob'], true);
    assert.equal(requests[0].signal.aborted, true);
    start();
    requests[0].resolve({response: {ok: true}, data: {sessions: [{public_id: 'stale'}]}});
    requests[1].resolve({response: {ok: true}, data: {sessions: [{public_id: 'bob-first'}], next_cursor: 'next'}});
    await flush();
    assert.match(requests[2].url, /cursor=next/);
    requests[2].resolve({response: {ok: true}, data: {sessions: [{public_id: 'bob-second'}]}});
    await flush();
    assert.deepEqual(Array.from(options, item => item.value), ['', 'bob-first', 'bob-second']);
    select.setValue('bob-second');
    assert.equal(target.value(['bob']), 'bob-second');
    assert.equal(target.value(['alice']), '');
    target.translate();
    assert.equal(target.value(['bob']), 'bob-second');
    target.update(['bob', 'alice'], true);
    assert.equal(field.inert, true);
    assert.equal(target.value(['bob']), '');
    target.update(['bob'], true);
    target.update(['bob'], false);
    assert.deepEqual(animations, ['expand', 'collapse', 'expand', 'collapse']);
});
