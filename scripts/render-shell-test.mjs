/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {readdirSync, readFileSync} from 'node:fs';
import {dirname, join, resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
import vm from 'node:vm';

/** Render component trees without browser globals for existing markup contract tests. @param {string|URL} file */
export function renderHTML(file) {
    const entry = file instanceof URL ? fileURLToPath(file) : file;
    const root = dirname(entry);
    const element = (tag, attrs = {}, ...children) => ({tag, attrs, children: children.flat(Infinity)});
    const context = vm.createContext({el: element, svg: element});
    for (const name of readdirSync(join(root, 'js/views'))) {
        const source = readFileSync(join(root, 'js/views', name), 'utf8')
            .replace(/^import .*;\r?\n/gm, '').replace(/^export /gm, '');
        vm.runInContext(source, context, {filename: name});
    }
    const shell = readFileSync(join(root, 'js/shell.js'), 'utf8').split('\nconst mount =')[0]
        .replace(/^import .*;\r?\n/gm, '').replace(/^export /gm, '');
    vm.runInContext(shell, context);
    const config = {title: '{{RENOP.TITLE}}', organizationLogo: '{{RENOP.ORGANIZATION_LOGO}}',
        organizationWebsite: '{{RENOP.ORGANIZATION_WEBSITE}}', icpLicense: '{{RENOP.ICP_LICENSE}}',
        publicSecurityFiling: '{{RENOP.PUBLIC_SECURITY_FILING}}'};
    const escape = value => String(value).replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    const serialize = node => {
        if (node == null || node === false) return '';
        if (typeof node !== 'object') return escape(node);
        const attrs = Object.entries(node.attrs).filter(([, value]) => value != null && value !== false)
            .map(([name, value]) => ` ${name}="${value === true ? '' : escape(value)}"`).join('');
        const open = `<${node.tag}${attrs}>`;
        if (['br', 'hr', 'img', 'input', 'link', 'meta'].includes(node.tag)) return open;
        return open + node.children.map(serialize).join('') + `</${node.tag}>`;
    };
    return readFileSync(entry, 'utf8').replace(/<div id="app"[^>]*><\/div>/,
        () => context.renderShell(config).map(serialize).join(''));
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
    process.stdout.write(renderHTML(process.argv[2]));
}
