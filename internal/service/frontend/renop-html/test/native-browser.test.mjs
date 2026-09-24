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

function fixture(request) {
    class Element {
        constructor(tag, props = {}, children = []) {
            this.tag = tag;
            Object.assign(this, props);
            this.children = children.filter(child => child != null);
            this.classList = {
                add: (...tokens) => {
                    const classes = (this.class || '').split(/\s+/).filter(Boolean);
                    for (const t of tokens) if (!classes.includes(t)) classes.push(t);
                    this.class = classes.join(' ');
                },
                remove: (...tokens) => {
                    const classes = (this.class || '').split(/\s+/).filter(Boolean);
                    this.class = classes.filter(t => !tokens.includes(t)).join(' ');
                }
            };
        }

        get childElementCount() {
            return this.children.length;
        }

        get firstElementChild() {
            return this.children[0];
        }

        get text() {
            return this.children.map(child => child instanceof Element ? child.text : String(child)).join(' ');
        }

        append(...children) {
            this.children.push(...children.filter(child => child != null));
        }

        prepend(...children) {
            this.children.unshift(...children.filter(child => child != null));
        }

        replaceChildren(...children) {
            this.children = children.filter(child => child != null);
        }

        addEventListener() {
        }

        find(predicate) {
            if (predicate(this)) return this;
            for (const child of this.children) {
                const found = child instanceof Element && child.find(predicate);
                if (found) return found;
            }
        }
    }

    const root = new Element('section');
    const mount = new Element('main');
    const listeners = new Map();
    const context = vm.createContext({
        AbortController,
        window: {addEventListener: (name, listener) => listeners.set(name, listener)},
        document: {querySelector: selector => selector === '.file-list-container' ? {parentElement: mount} : null},
        el: (tag, props, ...children) => new Element(tag, props, children),
        createJSONClient: base => (path, options) => request(base + path, options),
        createActionButton: (label, action, props) => new Element('button', {...props, action}, [label]),
        createIcon: () => new Element('svg'), createSkeleton: () => new Element('div'),
        createMetaGrid: items => new Element('div', {}, items.map(item => `${item.label}: ${item.value}`)),
        getRepositoryFormat: format => ({id: format, protocol: format, icon: 'package', labelKey: format}),
        createRepositoryBackButton: ({path, label, navigate, className}) => new Element('button', {
            class: className,
            action: () => navigate?.(path)
        }, [label]),
        ensureRepositoryView: (_current, options) => {
            assert.equal(options.mountResolver(), mount);
            return root;
        },
        hideRepositoryView: target => {
            if (target) {
                target.hidden = true;
                target.replaceChildren();
            }
        },
        setRepositoryViewBusy() {
        },
        replaceRepositoryView: async (target, content) => target.replaceChildren(...(Array.isArray(content) ? content : [content])),
        makeCustomSelect: (_options, value) => new Element('select', {getValue: () => value}),
        RepositoryUserSuggestions: class {
            attach() {
            }

            detach() {
            }
        },
        formatRepositoryTimestamp: String, formatBytes: String,
        decodePathSegment: decodeURIComponent, encodeRelativePath: value => value,
        t: value => value, caughtErrorMessage: (err) => err?.message || 'safe-error',
        apiRequest: async (path, options) => request(path, options),
        showConfirm: async () => true,
        cachedIsLoggedIn: true,
        createUserIdentity: (username) => new Element('span', {class: 'user-identity'}, [username]),
        copyWithFeedback: () => {
        },
        createTicketReportButton: () => new Element('button', {}, ['ticket.report']),
        createDeprecatePackageButton: () => new Element('button', {}, ['package.deprecate']),
        createPackageDeprecationBadge: () => new Element('span', {}, ['package.deprecated']),
        createPackageDeprecationNotice: () => new Element('div', {}, ['package.deprecationNotice']),
        createResourceLockButton: () => new Element('button', {}, ['resourceLock']),
        createResourceLockBadge: () => new Element('span', {}, ['resourceLock']),
        createResourceLockNotices: () => new Element('div', {}, ['resourceLockNotices']),
        openProfileGPGDialog: () => {
        },
        bindAnimatedDetails: () => {
        },
        localStorage: {getItem: () => ''},
    });
    const source = readFileSync(new URL('../js/browser/native.js', import.meta.url), 'utf8')
        .replace(/^import [\s\S]*? from '[^']+';\r?\n/gm, '').replaceAll('export ', '');
    vm.runInContext(source, context);
    return {context, root, listeners};
}

test('native pages reject stale loads and discard account-specific views', async () => {
    let finish;
    const pending = new Promise(resolve => {
        finish = resolve;
    });
    const {context, root, listeners} = fixture(async url => url.includes('/first/') ? pending : {
        resources: [],
        can_create: false
    });
    const earlier = context.renderNativeRepository('/first/', {format: 'conda'}, () => {
    });
    await context.renderNativeRepository('/second/', {format: 'conda'}, () => {
    });
    finish({resources: [{name: 'private-first'}], can_create: true});
    await earlier;
    assert.match(root.text, /second/);
    assert.doesNotMatch(root.text, /private-first/);
    assert.equal(root.find(node => node.tag === 'button' && node.text === 'native.reserve'), undefined);
    listeners.get('authChanged')();
    assert.equal(root.hidden, true);
    assert.equal(root.children.length, 0);
});

test('native resource controls follow L2 and L3 authority', async () => {
    let level = -1;
    const {context, root} = fixture(async () => ({
        resource: {name: 'example', permission_level: level, published_at: 1},
        artifacts: [{path: 'noarch/example.tar.bz2', version: '1.0', size: 100, published: true}],
        members: [{username: 'owner', level: 4}]
    }));
    const render = () => context.renderNativeRepository('/channel/~/example', {format: 'conda'}, () => {
    });
    await render();
    assert.doesNotMatch(root.text, /native.settings|native.members|native.release|common.delete/);
    level = 2;
    await render();
    assert.match(root.text, /common.delete/);
    assert.doesNotMatch(root.text, /native.settings|native.members|native.release/);
    level = 3;
    await render();
    assert.match(root.text, /native.settings/);
    assert.match(root.text, /native.members/);
    assert.doesNotMatch(root.text, /native.release/);
    const owner = root.find(node => node.class === 'native-member' && node.text.includes('owner'));
    assert.ok(owner);
    assert.equal(owner.find(node => node.tag === 'select'), undefined, 'L3 cannot edit an L4 owner');
});

test('a detached native action cannot write after an account change', async () => {
    let mutations = 0;
    const {context, root, listeners} = fixture(async (_url, options) => {
        if (options) mutations++;
        return {resources: [], can_create: true};
    });
    await context.renderNativeRepository('/channel/', {format: 'conda'}, () => {
    });
    const reserve = root.find(node => node.tag === 'button' && node.text === 'native.reserve');
    assert.ok(reserve);
    listeners.get('authChanged')();
    assert.equal(await reserve.action(), false);
    assert.equal(mutations, 0);
});

test('native resource parses versions and renders report, lock, deprecate, and archive actions', async () => {
    let deletedPackage = false;
    const {context, root} = fixture(async (url, options) => {
        if (options?.method === 'DELETE' && options?.json?.name === 'example') {
            deletedPackage = true;
            return {ok: true};
        }
        return {
            resource: {
                name: 'example',
                permission_level: 4,
                published_at: 1,
                locked: true,
                lock_reason: 'security check',
                deprecated: true,
                deprecation_reason: 'use example-v2',
                archived: true,
            },
            versions: [
                {version: '2.0.0', published: true, files: ['linux-64/example-2.0.0-py39_0.tar.bz2']},
                {version: '1.0.0', published: true, files: ['linux-64/example-1.0.0-py38_0.tar.bz2']},
            ],
            artifacts: [
                {
                    path: 'linux-64/example-2.0.0-py39_0.tar.bz2',
                    version: '2.0.0#py39_0#linux-64',
                    size: 2048,
                    published: true
                },
                {
                    path: 'linux-64/example-1.0.0-py38_0.tar.bz2',
                    version: '1.0.0#py38_0#linux-64',
                    size: 1024,
                    published: true
                },
            ],
            members: [{username: 'owner', level: 4}]
        };
    });

    await context.renderNativeRepository('/channel/~/example', {format: 'conda'}, () => {
    });

    // Badges & notices for locked, deprecated, archived
    assert.match(root.text, /resourceLock/);
    assert.match(root.text, /package\.deprecated/);
    assert.match(root.text, /npm\.archived/);

    // Header actions: report, lock, deprecate/unarchive, delete
    assert.match(root.text, /ticket\.report/);
    assert.match(root.text, /npm\.restorePackage/);
    assert.match(root.text, /native\.release/);

    // Version cards rendered
    assert.match(root.text, /2\.0\.0/);
    assert.match(root.text, /1\.0\.0/);
    assert.match(root.text, /linux-64/);

    // Delete package action works even when artifacts exist
    const deleteBtn = root.find(node => node.tag === 'button' && node.text === 'native.release');
    assert.ok(deleteBtn);
    await deleteBtn.action();
    assert.equal(deletedPackage, true);
});

test('rpm and conan formats provide GPG profile key dialog', async () => {
    let openedGPG = false;
    const {context, root} = fixture(async () => ({
        resource: {name: 'pkg', permission_level: 4, published_at: 1},
        artifacts: [],
        members: [{username: 'owner', level: 4}]
    }));
    context.openProfileGPGDialog = () => {
        openedGPG = true;
    };

    await context.renderNativeRepository('/repo/~/pkg', {format: 'rpm'}, () => {
    });
    const gpgBtn = root.find(node => node.tag === 'button' && node.text === 'profile.gpgBtn');
    assert.ok(gpgBtn, 'RPM format should have GPG manage button');
    gpgBtn.action();
    assert.equal(openedGPG, true);
});

test('native detail renders Docker-style topNav with kicker, version body wrapper, and invite controls', async () => {
    let navigatedPath = '';
    const {context, root} = fixture(async () => ({
        resource: {name: 'demo-pkg', permission_level: 4, published_at: 1},
        artifacts: [{path: 'linux-64/demo.conda', version: '1.0.0#build_0#linux-64', size: 512, published: true}],
        members: [{username: 'owner', level: 4}]
    }));

    await context.renderNativeRepository('/conda-repo/~/demo-pkg', {format: 'conda'}, path => {
        navigatedPath = path;
    });

    // Docker-style topNav with back button & kicker
    const topNav = root.find(node => node.class === 'native-hero-nav');
    assert.ok(topNav, 'Hero nav header should be present');
    const backBtn = topNav.find(node => node.class?.includes('native-page-back'));
    assert.ok(backBtn, 'Docker-style back button should be present');
    backBtn.action();
    assert.equal(navigatedPath, '/conda-repo/');
    const kicker = topNav.find(node => node.class === 'native-page-kicker');
    assert.ok(kicker, 'Kicker should be present');
    assert.equal(kicker.text, 'conda');

    // Version body wrapper for smooth animation without stutter
    const versionBody = root.find(node => node.class === 'native-version-body');
    assert.ok(versionBody, 'native-version-body wrapper should be present');
    const versionArtifacts = versionBody.find(node => node.class === 'native-version-artifacts');
    assert.ok(versionArtifacts, 'native-version-artifacts should be inside native-version-body');

    // Install command
    const installCmd = root.find(node => node.class === 'native-install-cmd');
    assert.ok(installCmd, 'native-install-cmd should be present');
    assert.ok(installCmd.find(node => node.tag === 'code'));

    // Invite controls
    const inviteControls = root.find(node => node.class === 'native-invite-controls');
    assert.ok(inviteControls, 'native-invite-controls should be present');
    assert.ok(inviteControls.find(node => node.class?.includes('native-invite-input')));
    assert.ok(inviteControls.find(node => node.class?.includes('native-invite-permission-select')));
});


