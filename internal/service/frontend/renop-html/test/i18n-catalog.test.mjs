/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {renderHTML} from '../../../../../scripts/render-shell-test.mjs';
import assert from 'node:assert/strict';
import {mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {dirname, join, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import test from 'node:test';
import vm from 'node:vm';

import {generateI18nCatalog, scanI18nCatalog} from '../scripts/i18n-catalog.mjs';

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');

test('account and email languages cover every frontend catalog', () => {
    const catalogs = readdirSync(join(frontendRoot, 'js/i18n'), {withFileTypes: true})
        .filter(entry => entry.isDirectory()).map(entry => entry.name).sort();
    const server = readFileSync(resolve(frontendRoot, '../../../locale/locale.go'), 'utf8');
    const declaration = server.match(/Codes = \[\.\.\.\]string\{([^}]+)}/)[1];
    const codes = [...declaration.matchAll(/"([^"]+)"/g)].map(match => match[1]).sort();
    assert.deepEqual(codes, catalogs);
});

/**
 * Write a minimal locale fragment fixture.
 * @param {string} root - Fixture root.
 * @param {string} locale - Locale directory.
 * @param {string} fragment - Fragment file name.
 * @param {object} values - Translation values.
 * @returns {void}
 */
function writeFragment(root, locale, fragment, values) {
    const directory = join(root, locale);
    mkdirSync(directory, {recursive: true});
    writeFileSync(join(directory, fragment), `export default ${JSON.stringify(values)};\n`, 'utf8');
}

test('parallel i18n scan accepts complete English-key parity', async () => {
    const root = mkdtempSync(join(tmpdir(), 'renop-i18n-valid-'));
    try {
        const catalogRoot = join(root, 'catalog');
        writeFragment(catalogRoot, 'en-US', 'common.js', {alpha: 'Hello {name}', beta: 'Ready'});
        writeFragment(catalogRoot, 'fr-FR', 'common.js', {alpha: 'Bonjour {name}', beta: 'Prêt'});
        const sourceRoot = join(root, 'src');
        mkdirSync(sourceRoot);
        writeFileSync(join(sourceRoot, 'app.js'), "const label = t('alpha');\n", 'utf8');

        const result = await scanI18nCatalog({i18nDir: catalogRoot, sourceRoots: [sourceRoot]});
        assert.equal(result.keyCount, 2);
        assert.equal(result.referenceCount, 1);
        assert.deepEqual(result.referenceFragments, ['common.js']);
        assert.ok(result.durationMs >= 0);
    } finally {
        rmSync(root, {recursive: true, force: true});
    }
});

test('i18n scan reports missing English keys referenced by JavaScript and HTML', async () => {
    const root = mkdtempSync(join(tmpdir(), 'renop-i18n-references-'));
    try {
        const catalogRoot = join(root, 'catalog');
        const sourceRoot = join(root, 'src');
        mkdirSync(sourceRoot, {recursive: true});
        writeFragment(catalogRoot, 'en-US', 'common.js', {alpha: 'Ready'});
        writeFragment(catalogRoot, 'fr-FR', 'common.js', {alpha: 'Prêt'});
        writeFileSync(join(sourceRoot, 'app.js'), "t('missing.javascript');\n", 'utf8');
        writeFileSync(join(sourceRoot, 'index.html'), '<span data-i18n="missing.html"></span>\n', 'utf8');

        await assert.rejects(
            scanI18nCatalog({i18nDir: catalogRoot, sourceRoots: [sourceRoot]}),
            (error) => {
                assert.equal(error.code, 'I18N_VALIDATION_FAILED');
                assert.match(error.message, /missing English key referenced by source: missing\.javascript \(app\.js:1\)/);
                assert.match(error.message, /missing English key referenced by source: missing\.html \(index\.html:1\)/);
                return true;
            }
        );
    } finally {
        rmSync(root, {recursive: true, force: true});
    }
});

test('i18n scan reports all missing keys, fragments, extras, and placeholder drift', async () => {
    const root = mkdtempSync(join(tmpdir(), 'renop-i18n-invalid-'));
    try {
        writeFragment(root, 'en-US', 'browser.js', {'browser.open': 'Open'});
        writeFragment(root, 'en-US', 'common.js', {alpha: 'Hello {name}', beta: 'Ready'});
        writeFragment(root, 'fr-FR', 'common.js', {alpha: 'Bonjour {nom}', gamma: 'Supplément'});

        await assert.rejects(
            scanI18nCatalog({i18nDir: root}),
            (error) => {
                assert.equal(error.code, 'I18N_VALIDATION_FAILED');
                assert.match(error.message, /missing fragments in fr-FR: browser\.js/);
                assert.match(error.message, /missing keys in fr-FR\/common\.js: beta/);
                assert.match(error.message, /extra keys in fr-FR\/common\.js: gamma/);
                assert.match(error.message, /placeholder mismatches in fr-FR\/common\.js: alpha/);
                return true;
            }
        );
    } finally {
        rmSync(root, {recursive: true, force: true});
    }
});

test('generated catalog loads protobuf languages lazily through one shared key index', async (t) => {
    const root = mkdtempSync(join(tmpdir(), 'renop-i18n-lazy-'));
    try {
        writeFragment(root, 'en-US', 'common.js', {alpha: 'Ready'});
        writeFragment(root, 'fr-FR', 'common.js', {alpha: 'Prêt'});
        const catalogFile = join(root, 'catalog.generated.js');
        const assets = await generateI18nCatalog({i18nDir: root, catalogFile});
        const requests = [];
        t.mock.method(globalThis, 'fetch', async path => {
            requests.push(path);
            return new Response(assets.get(path.slice(1)), {headers: {'Content-Type': 'application/octet-stream'}});
        });
        const {createLocaleLoader} = await import('@renop/ui/i18n-catalog');
        const catalog = vm.runInNewContext(readFileSync(catalogFile, 'utf8')
            .replace(/^import .*;$/gm, '').replace(/^export /gm, '') + '\n({availableLocales, loadLocale});', {createLocaleLoader});
        assert.deepEqual(Array.from(catalog.availableLocales), ['en-US', 'fr-FR']);
        assert.equal(requests.length, 0);
        assert.equal((await catalog.loadLocale('en-US')).alpha, 'Ready');
        const first = await catalog.loadLocale('fr-FR');
        const second = await catalog.loadLocale('fr-FR');
        assert.equal(first.alpha, 'Prêt');
        assert.equal(first, second);
        assert.equal(requests.length, 3);
        await assert.rejects(catalog.loadLocale('invalid'), /unsupported locale: invalid/);
    } finally {
        rmSync(root, {recursive: true, force: true});
    }
});

test('locale loader rejects truncated, oversized, and mismatched catalogs and retries failed requests', async t => {
    const {createLocaleLoader} = await import('@renop/ui/i18n-catalog');
    const {renop} = await import('../../../../../packages/renop-ui/js/i18n.pb.js');
    const {KeyIndex, LocaleCatalog} = renop.i18n.v1;
    const index = KeyIndex.encode({revision: 'test', keys: ['alpha', 'beta']}).finish();
    const valid = {locale: 'fr-FR', key_revision: 'test', values: ['Bonjour {name}', 'Prêt']};
    let mode = 'truncated';
    t.mock.method(globalThis, 'fetch', async path => {
        if (path === '/keys') return new Response(index, {headers: {'Content-Type': 'application/octet-stream'}});
        let data = LocaleCatalog.encode(valid).finish();
        if (mode === 'truncated') data = data.slice(0, -1);
        if (mode === 'mismatched') data = LocaleCatalog.encode({...valid, key_revision: 'old'}).finish();
        if (mode === 'missing') data = LocaleCatalog.encode({...valid, values: ['Bonjour']}).finish();
        const headers = {'Content-Type': 'application/octet-stream'};
        if (mode === 'oversized') headers['Content-Length'] = String((2 << 20) + 1);
        return new Response(data, {headers});
    });
    const load = createLocaleLoader({index: '/keys', revision: 'test', locales: {'fr-FR': '/french'}});
    for (mode of ['truncated', 'mismatched', 'missing', 'oversized']) await assert.rejects(load('fr-FR'));
    mode = 'valid';
    assert.deepEqual({...await load('fr-FR')}, {alpha: 'Bonjour {name}', beta: 'Prêt'});
});

test('locale and control API protobuf modules have independent roots', async () => {
    const {renop} = await import('../../../../../packages/renop-ui/js/i18n.pb.js');
    const {UnreadCountResponse} = await import('../js/proto/index.js');
    const {KeyIndex} = renop.i18n.v1;
    const bytes = KeyIndex.encode({revision: 'shared-test', keys: ['alpha']}).finish();
    assert.deepEqual(KeyIndex.decode(bytes).keys, ['alpha']);
    const apiBytes = UnreadCountResponse.encode({unread_count: 7}).finish();
    assert.equal(UnreadCountResponse.decode(apiBytes).unread_count, 7);
});

test('initial translations and language changes expose deterministic loading state', () => {
    const runtime = readFileSync(join(frontendRoot, 'js/i18n.js'), 'utf8');
    const page = renderHTML(join(frontendRoot, 'index.html'));
    const baseStyles = readFileSync(join(frontendRoot, 'css/layout/base.css'), 'utf8');
    const settingsStyles = readFileSync(join(frontendRoot, 'css/manager/settings/loading.css'), 'utf8');

    assert.match(page, /id="language-load-progress"[^>]*role="progressbar"/);
    assert.match(runtime, /setLanguageLoading\(true\)/);
    assert.match(runtime, /finally \{[\s\S]*?setLanguageLoading\(false\)/);
    assert.match(runtime, /document\.documentElement\.dataset\.i18nReady = 'true'/);
    assert.match(runtime, /if \(!document\.body\)[\s\S]*?else \{\s*setupLanguageModal\(\)/);
    assert.match(baseStyles, /html:not\(\[data-i18n-ready="true"\]\) \[data-i18n\][^}]*visibility: hidden/s);
    assert.match(settingsStyles, /\.language-load-progress > span[^}]*animation: languageLoadProgress/s);
    for (const key of ['users.thUser', 'users.thPermissions', 'users.thCreatedAt']) {
        assert.match(page, new RegExp(`data-i18n="${key}"`));
    }
});
