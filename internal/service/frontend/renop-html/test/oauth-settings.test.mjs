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

const source = readFileSync(new URL('../js/settings/oauth.js', import.meta.url), 'utf8');

function createDOMContext() {
    const node = (tag, attrs = {}, ...children) => {
        const elem = {
            tag,
            className: attrs.class || '',
            classList: {
                add(c) { elem.className += ' ' + c; },
                remove(c) { elem.className = elem.className.replace(c, '').trim(); },
                toggle(c, v) { if (v) this.add(c); else this.remove(c); }
            },
            dataset: {},
            children: children.flat(),
            listeners: {},
            appendChild(child) { elem.children.push(child); return child; },
            append(...items) { elem.children.push(...items); },
            replaceChildren(...items) { elem.children = items; },
            before(item) { elem.children.unshift(item); },
            addEventListener(name, fn) { elem.listeners[name] = fn; },
            setAttribute(name, val) { elem[name] = val; },
            querySelector(sel) {
                if (sel === '.cfg-fields') {
                    if (!elem.fields) {
                        elem.fields = node('div', {class: 'cfg-fields'});
                        elem.children.push(elem.fields);
                    }
                    return elem.fields;
                }
                if (sel === '.cfg-section-title') return elem.title || (elem.title = node('div', {class: 'cfg-section-title'}));
                if (sel === '.cfg-section-subtitle') return elem.sub || (elem.sub = node('div', {class: 'cfg-section-subtitle'}));
                if (sel === '.cfg-section-icon') return elem.icon || (elem.icon = node('div', {class: 'cfg-section-icon'}));
                if (sel === '.cfg-section-chevron') return elem.chev || (elem.chev = node('div', {class: 'cfg-section-chevron'}));
                if (sel === 'button') return node('button');
                return null;
            }
        };
        return elem;
    };

    let presetSelectOptions = [];
    let presetSelectChange = null;
    let selectOptions = [];
    let selectChange = null;

    const context = {
        el: node,
        t: (k, params) => k + (params ? JSON.stringify(params) : ''),
        makeCustomSelect: (options, current, onChange) => {
            const isPresetSelect = options.some(o => ['github', 'google', 'microsoft', 'gitlab', 'cloudflare', 'stackexchange', 'custom'].includes(o.value || o));
            if (isPresetSelect) {
                presetSelectOptions = options;
                presetSelectChange = onChange;
            }
            selectOptions = options;
            selectChange = onChange;
            const selNode = node('div', {class: 'custom-select'});
            selNode.setOptions = (nextOptions) => {
                if (isPresetSelect || nextOptions.some(o => ['github', 'google', 'microsoft', 'gitlab', 'cloudflare', 'stackexchange', 'custom'].includes(o.value || o))) {
                    presetSelectOptions = nextOptions;
                }
                selectOptions = nextOptions;
            };
            return selNode;
        },
        createSection: (_icon, title) => {
            const sec = node('div', {class: 'cfg-section'});
            sec.querySelector('.cfg-fields');
            return sec;
        },
        createActionButton: (label, action) => {
            const btn = node('button', {}, label);
            btn.action = action;
            return btn;
        },
        createCallout: () => node('div', {class: 'callout'}),
        createFieldRow: (_label, _hint, ctrl) => node('div', {class: 'row'}, ctrl),
        createIcon: name => node('icon', {name}),
        createSubHeader: () => node('div', {class: 'sub-header'}),
        createToggle: () => node('toggle'),
        createToggleRow: () => node('toggle-row'),
        buildInput: (type, val, placeholder, onChange) => {
            const inp = node('input', {type});
            inp.value = val || '';
            inp.addEventListener('input', e => onChange(e));
            return inp;
        },
        collapseElement: async () => {},
        expandElement: async () => {},
        writeClipboardText: async () => {},
        createSettingsGuide: () => node('div', {class: 'guide'}),
        window: {
            location: {origin: 'https://renop.test'},
            showConfirm: async () => true,
            requestAnimationFrame: fn => fn()
        },
        document: {
            createTextNode: t => t
        },
        structuredClone,
        getSelectOptions: () => presetSelectOptions.length ? presetSelectOptions : selectOptions,
        triggerSelectChange: (val) => {
            if (presetSelectChange) presetSelectChange(val);
            else if (selectChange) selectChange(val);
        }
    };
    return context;
}

test('renderOAuthSettings renders delete button for GitHub and supports removing and re-adding', async () => {
    const ctx = createDOMContext();
    const sandbox = vm.createContext(ctx);

    // Strip imports and run renderOAuthSettings
    const code = source
        .replace(/import\s+[\s\S]*?from\s+['"][^'"]+['"];?/g, '')
        .replace('export function renderOAuthSettings', 'function renderOAuthSettings') +
        '; renderOAuthSettings';

    const renderOAuthSettings = vm.runInNewContext(code, sandbox);

    const container = ctx.el('div');
    const data = {
        providers: [
            {id: 'github', type: 'github', name: 'GitHub', enabled: true, client_id: 'gh-123'}
        ],
        presets: [
            {type: 'github', name: 'GitHub'},
            {type: 'google', name: 'Google'},
            {type: 'gitlab', name: 'GitLab'}
        ]
    };
    let changedCount = 0;
    renderOAuthSettings(container, data, () => changedCount++);

    // 1. Initial state: GitHub is in providers, so presetSelect should not offer GitHub
    let options = ctx.getSelectOptions();
    assert.ok(!options.some(o => o.value === 'github'), 'GitHub should not be in preset choices when already configured');

    // 2. Look for the remove button in GitHub provider
    const allButtons = [];
    function findButtons(n) {
        if (n.tag === 'button') allButtons.push(n);
        if (n.children) n.children.forEach(c => typeof c === 'object' && findButtons(c));
    }
    findButtons(container);

    const removeBtn = allButtons.find(b => b.children.some(c => typeof c === 'string' && c.startsWith('oauth.remove')));
    assert.ok(removeBtn, 'GitHub provider card must have a remove button');

    // 3. Click remove
    await removeBtn.listeners['click']();
    assert.equal(data.providers.length, 0, 'GitHub should be removed from providers');
    assert.equal(changedCount, 1, 'Form should be marked changed');

    // 4. After deletion, preset choices must now include GitHub
    options = ctx.getSelectOptions();
    assert.ok(options.some(o => o.value === 'github'), 'GitHub should now be available in preset choices');

    // 5. Select GitHub preset and click Add
    ctx.triggerSelectChange('github');
    const addBtn = allButtons.find(b => b.action);
    assert.ok(addBtn, 'Add button should exist');
    await addBtn.action();

    assert.equal(data.providers.length, 1, 'GitHub should be added back');
    assert.equal(data.providers[0].type, 'github');
    assert.equal(data.providers[0].id, 'github');
    assert.equal(data.providers[0].callback_url, 'https://renop.test/api/auth/github/callback');
    assert.equal(changedCount, 2, 'Form should be marked changed again');

    // 6. After re-adding, presets choices should no longer have GitHub
    options = ctx.getSelectOptions();
    assert.ok(!options.some(o => o.value === 'github'), 'GitHub should not be in preset choices once added');
});

test('OAuth settings save payload includes tombstone when GitHub is deleted and fetch filters unconfigured GitHub', () => {
    const settingsSource = readFileSync(new URL('../js/settings.js', import.meta.url), 'utf8');

    // Verify fetchDomainSettings filters unconfigured GitHub
    assert.match(settingsSource, /result\.data\.providers\.filter\(p => p\.type !== 'github' \|\| p\.client_id \|\| p\.enabled \|\| p\.client_secret_configured\)/);

    // Verify saveDomainSettings sends cleared GitHub when omitted
    assert.match(settingsSource, /const hasGitHub = submitted\.providers\.some\(p => p\.type === 'github'\)/);
    assert.match(settingsSource, /clear_client_secret: true/);
    assert.match(settingsSource, /clear_revocation_secret: true/);
});

test('renderOAuthSettings hides added non-custom presets from dropdown and keeps custom OAuth 2.0', async () => {
    const ctx = createDOMContext();
    const sandbox = vm.createContext(ctx);

    const code = source
        .replace(/import\s+[\s\S]*?from\s+['"][^'"]+['"];?/g, '')
        .replace('export function renderOAuthSettings', 'function renderOAuthSettings') +
        '; renderOAuthSettings';

    const renderOAuthSettings = vm.runInNewContext(code, sandbox);

    const container = ctx.el('div');
    const data = {
        providers: [
            {id: 'google', type: 'google', name: 'Google', enabled: true}
        ],
        presets: [
            {type: 'google', name: 'Google'},
            {type: 'microsoft', name: 'Microsoft'},
            {type: 'custom', name: 'OAuth 2.0'}
        ]
    };
    renderOAuthSettings(container, data, () => {});

    // 1. Initial state: Google is already in providers, so it should NOT be in options
    let options = ctx.getSelectOptions();
    assert.ok(!options.some(o => o.value === 'google'), 'Google should be filtered out from presets when already added');
    assert.ok(options.some(o => o.value === 'microsoft'), 'Microsoft should be available in presets');
    assert.ok(options.some(o => o.value === 'custom'), 'OAuth 2.0 (custom) should always be available');

    // 2. Add Microsoft
    ctx.triggerSelectChange('microsoft');
    const allButtons = [];
    function findButtons(n) {
        if (n.tag === 'button') allButtons.push(n);
        if (n.children) n.children.forEach(c => typeof c === 'object' && findButtons(c));
    }
    findButtons(container);
    const addBtn = allButtons.find(b => b.action);
    await addBtn.action();

    assert.equal(data.providers.length, 2);
    assert.ok(data.providers.some(p => p.type === 'microsoft'));

    // 3. Now both Google and Microsoft are added, so neither should be in options; custom must still be present
    options = ctx.getSelectOptions();
    assert.ok(!options.some(o => o.value === 'google'), 'Google should still be filtered out');
    assert.ok(!options.some(o => o.value === 'microsoft'), 'Microsoft should now also be filtered out');
    assert.ok(options.some(o => o.value === 'custom'), 'OAuth 2.0 (custom) should still be available');

    // 4. Add custom provider
    ctx.triggerSelectChange('custom');
    await addBtn.action();
    assert.equal(data.providers.length, 3);
    assert.ok(data.providers.some(p => p.type === 'custom'));

    // Custom should STILL be available after being added
    options = ctx.getSelectOptions();
    assert.ok(options.some(o => o.value === 'custom'), 'OAuth 2.0 (custom) should remain in options even after adding');

    // 5. Remove Google provider card
    allButtons.length = 0;
    findButtons(container);
    const removeBtns = allButtons.filter(b => b.children.some(c => typeof c === 'string' && c.startsWith('oauth.remove')));
    assert.equal(removeBtns.length, 3);
    // Click first remove button (Google)
    await removeBtns[0].listeners['click']();
    assert.equal(data.providers.length, 2);
    assert.ok(!data.providers.some(p => p.type === 'google'));

    // Google should now reappear in preset options
    options = ctx.getSelectOptions();
    assert.ok(options.some(o => o.value === 'google'), 'Google should reappear in presets after removal');
});

test('renderOAuthSettings disables add button when limit of 12 is reached', async () => {
    const ctx = createDOMContext();
    const sandbox = vm.createContext(ctx);

    const code = source
        .replace(/import\s+[\s\S]*?from\s+['"][^'"]+['"];?/g, '')
        .replace('export function renderOAuthSettings', 'function renderOAuthSettings') +
        '; renderOAuthSettings';

    const renderOAuthSettings = vm.runInNewContext(code, sandbox);

    const container = ctx.el('div');
    const providers = [];
    for (let i = 0; i < 12; i++) {
        providers.push({id: 'custom-' + i, type: 'custom', name: 'Custom ' + i, enabled: true});
    }
    const data = {
        providers,
        presets: [
            {type: 'custom', name: 'OAuth 2.0'}
        ]
    };
    renderOAuthSettings(container, data, () => {});

    const allButtons = [];
    function findButtons(n) {
        if (n.tag === 'button') allButtons.push(n);
        if (n.children) n.children.forEach(c => typeof c === 'object' && findButtons(c));
    }
    findButtons(container);
    const addBtn = allButtons.find(b => b.action);
    assert.ok(addBtn, 'Add button should exist');
    assert.equal(addBtn.disabled, true, 'Add button must be disabled when 12 providers are configured');
});


