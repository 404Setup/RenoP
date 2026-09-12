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
const selector = process.env.RENOP_TEST_SELECT_SELECTOR || '[data-mail-select="template_style"] button';

test('shared select supports keyboard selection, Escape, and focus restoration', {skip: !endpoint}, async () => {
    const browser = await connectBrowserPage(endpoint);
    const {evaluate, key} = browser;
    try {
        await evaluate('window.selectUnderTest = document.querySelector(' + JSON.stringify(selector) + '); selectUnderTest.focus()');
        const sectionPresent = await evaluate('window.sectionUnderTest = selectUnderTest.closest(".cfg-section"); Boolean(sectionUnderTest)');
        if (sectionPresent) {
            await evaluate('window.sectionHeader = sectionUnderTest.querySelector(".cfg-section-header"); if(sectionUnderTest.classList.contains("is-collapsed")) sectionHeader.click(); sectionHeader.focus()');
            await key('Enter');
            assert.equal(await evaluate('sectionUnderTest.querySelector(".cfg-section-body").inert'), true);
            await evaluate('selectUnderTest.focus()');
            assert.equal(await evaluate('document.activeElement === sectionHeader'), true);
            await key('Enter');
            assert.equal(await evaluate('sectionUnderTest.querySelector(".cfg-section-body").inert'), false);
            await evaluate('selectUnderTest.focus()');
        }
        const original = await evaluate('selectUnderTest.textContent.trim()');
        assert.equal(await evaluate('document.getElementById(selectUnderTest.getAttribute("aria-controls"))'), null);
        await key('ArrowDown');
        assert.equal(await evaluate('document.activeElement.getAttribute("role")'), 'option');
        await key('ArrowDown');
        const expected = await evaluate('document.activeElement.textContent.trim()');
        await key('Enter');
        assert.equal(await evaluate('selectUnderTest.textContent.trim()'), expected);
        assert.equal(await evaluate('document.activeElement === selectUnderTest'), true);
        await key('ArrowDown');
        await key('Escape');
        assert.equal(await evaluate('selectUnderTest.getAttribute("aria-expanded")'), 'false');
        assert.equal(await evaluate('document.activeElement === selectUnderTest'), true);
        assert.equal(await evaluate('document.getElementById(selectUnderTest.getAttribute("aria-controls"))'), null);
        await evaluate('window.selectWrap = selectUnderTest.closest(".custom-select-wrapper"); window.selectParent = selectWrap.parentNode; window.selectNext = selectWrap.nextSibling; selectWrap.remove()');
        await new Promise(resolve => setTimeout(resolve, 200));
        await evaluate('selectParent.insertBefore(selectWrap, selectNext); selectUnderTest.focus()');
        await key('ArrowDown');
        await evaluate('Array.from(document.getElementById(selectUnderTest.getAttribute("aria-controls")).children).find(item => item.textContent.trim() === ' + JSON.stringify(original) + ').click()');
        assert.equal(await evaluate('selectUnderTest.textContent.trim()'), original);
    } finally {
        browser.close();
    }
});
