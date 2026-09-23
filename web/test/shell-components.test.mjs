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
import {renderHTML} from '../../scripts/render-shell-test.mjs';

test('JS shell components retain unique mount points and native SVG attributes', () => {
    const html = renderHTML(new URL('../index.html', import.meta.url));
    const ids = [...html.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]);
    assert.equal(new Set(ids).size, ids.length);
    for (const id of ['app', 'page-root', 'language-modal', 'footer-copyright']) assert.ok(ids.includes(id), id);
    const source = readFileSync(new URL('../../packages/renop-ui/js/dom.js', import.meta.url), 'utf8')
        .replace(/^import .*;\r?\n/gm, '').replace(/^export /gm, '');
    const context = vm.createContext({
        document: {
            createElementNS(namespace, tag) {
                return {
                    namespace, tag, attrs: {}, children: [],
                    setAttribute(name, value) {
                        this.attrs[name] = value;
                    },
                    append(...children) {
                        this.children.push(...children);
                    }
                };
            },
        }
    });
    vm.runInContext(source, context);
    const child = context.svg('path', {d: 'M0 0L1 1'});
    const icon = context.svg('svg', {viewBox: '0 0 24 24', width: 16}, child);
    assert.equal(icon.namespace, 'http://www.w3.org/2000/svg');
    assert.equal(icon.attrs.viewBox, '0 0 24 24');
    assert.equal(icon.attrs.width, '16');
    assert.equal(icon.children[0], child);
});
