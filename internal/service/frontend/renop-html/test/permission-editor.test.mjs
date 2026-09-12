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
import test from 'node:test';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

test('permission drafts stay isolated, preserve edits during discovery, and cancel stale loads', async () => {
    const loads = [];
    const node = (tag, attrs = {}, ...children) => ({tag, attrs, children, listeners: {},
        append(...values) { this.children.push(...values); },
        appendChild(value) { this.children.push(value); },
        replaceChildren(...values) { this.children = values; },
        setAttribute(key, value) { this.attrs[key] = value; },
        addEventListener(event, listener) { this.listeners[event] = listener; },
    });
    const context = vm.createContext({t: key => key, el: node, MavenRepositoriesResponse: {},
        createRolesGroup: title => node('group', {title}), createEmptyState: value => node('empty', value),
        createRoleChip: (value, options) => node('chip', {value, ...options}),
        fetchProto: () => new Promise(resolve => loads.push(resolve)),
    });
    const source = readFileSync(new URL('../js/permission-editor.js', import.meta.url), 'utf8');
    vm.runInContext(source.replace(/^import .*;$/gm, '').replace(/^export /gm, ''), context);
    const changes = [];
    const first = context.createPermissionEditor(['base', 'canview:archived'], values => changes.push([...values]));
    const second = context.createPermissionEditor(['canmoderate:*']);
    const find = (element, value) => element.attrs?.value === value ? element : element.children?.map(child => typeof child === 'object' ? find(child, value) : null).find(Boolean);
    const admin = find(first.element, 'admin');
    admin.listeners.change({detail: {checked: true}});
    assert.deepEqual([...first.values()], ['base', 'canview:archived', 'admin']);
    assert.deepEqual([...second.values()], ['canmoderate:*']);
    loads[0]({response: {ok: true}, data: {repositories: {beta: {}, alpha: {}}}});
    await first.ready;
    assert.equal(find(first.element, 'canview:archived').attrs.checked, true);
    assert.equal(find(first.element, 'admin').attrs.checked, false); // The mock does not implement the chip's internal state.
    assert.ok(first.values().includes('admin'), 'late results cannot reset an edited draft');
    const secondList = second.element.children[1].children[0];
    const previous = secondList.children;
    second.dispose();
    loads[1]({response: {ok: true}, data: {repositories: {stale: {}}}});
    await second.ready;
    assert.equal(secondList.children, previous);
    assert.equal(changes.length, 1);
    assert.notEqual(admin.attrs['for-id'], find(second.element, 'admin').attrs['for-id']);
});
