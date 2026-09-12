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
import {installBackNavigation, navigateBack} from '../js/back-navigation.js';

/** Model browser history entries and the URL changes used by the application. */
function browser(referrer = '') {
    const entries = [{url: '/account/tickets', state: null}];
    let index = 0;
    const target = {
        document: {referrer}, location: new URL('https://renop.test/account/tickets'),
        PopStateEvent: class {}, dispatchEvent() {}, backCount: 0,
        history: {
            get state() { return entries[index].state; },
            get length() { return entries.length + (referrer ? 1 : 0); },
            pushState(state, title, url) {
                entries.splice(++index, Infinity, {state, url});
                target.location = new URL(url, target.location);
            },
            replaceState(state, title, url = target.location.href) {
                entries[index] = {state, url};
                target.location = new URL(url, target.location);
            },
            back() {
                target.backCount++;
                if (index > 0) target.location = new URL(entries[--index].url, target.location);
            },
        },
    };
    return target;
}

test('Back preserves route state, follows previous entries, and handles history branching', () => {
    const target = browser();
    installBackNavigation(target);
    installBackNavigation(target);
    target.history.pushState({renopProfileReturnPath: '/account/tickets'}, '', '/user/alice');
    target.history.replaceState({...target.history.state, panel: 'teams'}, '', '/user/alice');
    assert.equal(target.history.state.renopProfileReturnPath, '/account/tickets');
    navigateBack(target);
    assert.equal(target.location.pathname, '/account/tickets');
    target.history.pushState(null, '', '/team/demo');
    navigateBack(target);
    assert.equal(target.location.pathname, '/account/tickets');
    navigateBack(target);
    assert.equal(target.location.pathname, '/');
    assert.equal(target.backCount, 2);
});

test('direct and external entries fall back home; same-origin document navigation can go back', () => {
    for (const referrer of ['', 'https://external.test/', 'https://renop.test/account/tickets']) {
        const target = browser(referrer);
        installBackNavigation(target);
        navigateBack(target);
        assert.equal(target.location.pathname, '/');
        assert.equal(target.backCount, 0);
    }
    const target = browser('https://renop.test/files/');
    installBackNavigation(target);
    navigateBack(target);
    assert.equal(target.backCount, 1);
});
