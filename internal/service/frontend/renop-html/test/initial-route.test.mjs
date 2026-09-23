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

test('saved manager tabs apply only at the root, preserving direct repository and public resource links', () => {
    const source = readFileSync(new URL('../js/main.js', import.meta.url), 'utf8');
    const start = source.indexOf('        let savedTab =');
    const block = source.slice(start, source.indexOf('        await switchTab(savedTab);', start));
    for (const [pathname, profileRoute, accountTab, expected] of [
        ['/', null, '', 'settings'],
        ['/files/deep/path/', null, '', 'overview'],
        ['/team/demo', null, '', 'overview'],
        ['/user/alice', {}, '', 'profile'],
        ['/account/tickets', null, 'tickets', 'tickets'],
    ]) {
        let loadingRemoved = false;
        const context = {
            window: {location: {pathname}}, profileRoute, accountTab,
            document: {
                getElementById: () => ({
                    remove() {
                        loadingRemoved = true;
                    }
                })
            },
            localStorage: {
                getItem: () => 'settings', setItem() {
                }
            }, cachedIsLoggedIn: true, cachedIsManager: true,
            isAccountTab: tab => tab === 'tickets', isManagerTab: tab => tab === 'settings'
        };
        assert.equal(vm.runInNewContext(block + '\nsavedTab', context), expected, pathname);
        assert.equal(loadingRemoved, true);
    }
});

test('Back restores the root tab recorded in that history entry', () => {
    const source = readFileSync(new URL('../js/main.js', import.meta.url), 'utf8');
    const start = source.indexOf("window.addEventListener('popstate',");
    const handler = source.slice(start, source.indexOf('\n});', start) + 4);
    for (const [pathname, renopTab, expected] of [
        ['/', 'settings', 'settings'], ['/', 'dashboard', 'dashboard'],
        ['/', 'unknown', 'overview'], ['/', undefined, 'overview'],
        ['/files/package', 'settings', 'overview'],
    ]) {
        let selected;
        vm.runInNewContext(handler, {
            window: {
                location: {pathname},
                history: {state: {renopTab}},
                addEventListener: (_name, callback) => callback()
            },
            profileRouteFromPath: () => null, publicMavenDomainRouteFromPath: () => false,
            publicSuperTeamRouteFromPath: () => false, accountTabFromPath: () => '',
            isManagerTab: value => ['settings', 'dashboard', 'users', 'repositories'].includes(value),
            switchTab: tab => {
                selected = tab;
            },
        });
        assert.equal(selected, expected);
    }
});
