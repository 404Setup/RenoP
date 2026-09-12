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
import {connectBrowserPage} from './browser-session.mjs';

const endpoint = process.env.RENOP_TEST_BROWSER_CDP;

test('token target cards confirm independently, cancel drafts, and retain parent focus', {
    skip: !endpoint || process.env.RENOP_TEST_TOKEN_DIALOG !== '1',
}, async () => {
    const browser = await connectBrowserPage(endpoint);
    const {evaluate, key} = browser;
    try {
        assert.equal(await evaluate(`Boolean(document.getElementById('profile-api-token-create-dialog'))`), true);
        await evaluate(`window.testScope = document.querySelector('input[value="repository:read"]');
            window.testScopeChecked = testScope.checked;
            if (!testScope.checked) testScope.click();
            window.testEdit = testScope.closest('.profile-api-token-scope-entry').querySelector('button');
            window.testSummary = testScope.closest('.profile-api-token-scope-entry').querySelector('[data-api-token-target-count]');
            window.testOriginalTargets = testSummary.title ? testSummary.title.split(', ') : [];
            testEdit.focus(); testEdit.click();`);
        await evaluate(`new Promise(resolve => setTimeout(resolve, 250))`);
        await evaluate(`window.testDialog = document.getElementById('profile-api-token-target-dialog');
            window.testTarget = testDialog.querySelector('textarea'); testTarget.value = 'regression-exact-target';
            testDialog.querySelector('button.pill-btn').click();
            testDialog.querySelectorAll('.profile-api-token-target-cards input').forEach(input => {
                if (input.checked && input.value !== 'regression-exact-target') input.click();
            }); testDialog.querySelector('.primary-btn').click();`);
        await evaluate(`new Promise(resolve => setTimeout(resolve, 250))`);
        assert.equal(await evaluate(`testSummary.title`), 'regression-exact-target');
        assert.equal(await evaluate(`document.activeElement === testEdit`), true);
        assert.equal(await evaluate(`document.getElementById('profile-api-token-create-dialog').inert`), false);
        await evaluate(`testEdit.focus();testEdit.click();`);
        await evaluate(`new Promise(resolve => setTimeout(resolve, 250))`);
        await evaluate(`document.querySelector('#profile-api-token-target-dialog input[type=radio]').click()`);
        await key('Escape');
        await evaluate(`new Promise(resolve => setTimeout(resolve, 250))`);
        assert.equal(await evaluate(`Boolean(document.getElementById('profile-api-token-target-dialog'))`), false);
        assert.equal(await evaluate(`testSummary.title`), 'regression-exact-target');
        assert.equal(await evaluate(`document.activeElement === testEdit`), true);
    } finally {
        await evaluate(`document.getElementById('profile-api-token-target-dialog')?.close(false);
            new Promise(resolve => setTimeout(resolve, 250));`);
        await evaluate(`if (window.testEdit?.isConnected && window.testOriginalTargets) {
            testEdit.focus();testEdit.click();
            const dialog = document.getElementById('profile-api-token-target-dialog');
            if (testOriginalTargets.length) {
                dialog.querySelector('textarea').value = testOriginalTargets.join(', ');
                dialog.querySelector('button.pill-btn').click();
                dialog.querySelectorAll('.profile-api-token-target-cards input').forEach(input => {
                    if (input.checked !== testOriginalTargets.includes(input.value)) input.click();
                });
            } else dialog.querySelector('input[type=radio]').click();
            dialog.querySelector('.primary-btn').click();
            if (testScope.checked !== testScopeChecked) testScope.click();
        }`);
        browser.close();
    }
});
