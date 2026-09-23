/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {t} from './i18n.js';
import {fetchProto} from './api.js';
import {el} from '@renop/ui/dom';
import {MavenRepositoriesResponse} from './proto/index.js';
import {createEmptyState, createRoleChip, createRolesGroup} from './components.js';

const basePermissions = ['admin', 'base', 'showing', 'allview', 'canview:*', 'canmoderate:*', 'canupdate:*'];
let editorSequence = 0;

function permissionMeta(permission) {
    const metadata = {
        admin: {title: t('users.roleAdminTitle'), desc: t('users.roleAdminDesc'), tone: 'admin'},
        base: {title: t('users.roleBaseTitle'), desc: t('users.roleBaseDesc'), tone: 'system'},
        showing: {title: t('users.roleShowingTitle'), desc: t('users.roleShowingDesc'), tone: 'system'},
        allview: {title: t('users.roleAllviewTitle'), desc: t('users.roleAllviewDesc'), tone: 'view'},
        'canview:*': {title: t('users.roleCanviewAllTitle'), desc: t('users.roleCanviewAllDesc'), tone: 'view'},
        'canmoderate:*': {
            title: t('users.roleModeratorAllTitle'),
            desc: t('users.roleModeratorAllDesc'),
            tone: 'moderator'
        },
        'canupdate:*': {title: t('users.roleCanupdateAllTitle'), desc: t('users.roleCanupdateAllDesc'), tone: 'update'},
    };
    return metadata[permission] || {title: permission, tone: 'system'};
}

/** Create an independent permission draft that survives delayed repository discovery. */
export function createPermissionEditor(permissions = [], changed = () => {
}) {
    const selected = new Set(permissions);
    const element = el('div', {class: 'permission-editor'});
    const id = ++editorSequence;
    let disposed = false;
    const chip = (value, metadata = permissionMeta(value)) => {
        const control = createRoleChip(value, {...metadata, code: value, checked: selected.has(value)});
        control.setAttribute('for-id', 'permission-' + id + '-' + encodeURIComponent(value));
        control.addEventListener('change', event => {
            if (!event.detail || disposed) return;
            if (event.detail.checked) selected.add(value);
            else selected.delete(value);
            changed([...selected]);
        });
        return control;
    };
    const system = createRolesGroup(t('users.systemRoles'), t('users.systemRolesDesc'));
    const grid = el('div', {class: 'roles-chip-grid'});
    for (const value of new Set([...basePermissions, ...permissions.filter(value => !/^can(view|update|moderate):[^*]/.test(value))])) grid.appendChild(chip(value));
    system.appendChild(grid);
    const repositories = createRolesGroup(t('users.repoAccess'), t('users.repoAccessDesc'));
    const list = el('div', {class: 'roles-repo-list'}, el('div', {role: 'status'}, t('common.loading')));
    repositories.appendChild(list);
    element.append(system, repositories);
    const ready = (async () => {
        let names = new Set();
        for (const value of selected) {
            const match = /^can(view|update|moderate):([^*].*)$/.exec(value);
            if (match) names.add(match[2]);
        }
        let failed = false;
        try {
            const {response, data} = await fetchProto('/api/settings/maven/repositories', MavenRepositoriesResponse);
            if (!response.ok || !data) failed = true;
            else for (const name of Object.keys(data.repositories || {})) names.add(name);
        } catch {
            failed = true;
        }
        if (disposed) return;
        list.replaceChildren();
        if (failed) list.appendChild(createEmptyState({message: t('users.couldNotLoadRepos')}));
        else if (!names.size) list.appendChild(createEmptyState({message: t('users.noReposYet')}));
        for (const name of [...names].sort((a, b) => a.localeCompare(b, undefined, {
            numeric: true,
            sensitivity: 'base'
        }))) {
            const actions = el('div', {class: 'roles-repo-actions'});
            for (const [kind, label, tone] of [['canview', 'users.roleView', 'view'], ['canmoderate', 'users.roleModerate', 'moderator'], ['canupdate', 'users.roleDeploy', 'update']]) {
                actions.appendChild(chip(kind + ':' + name, {title: t(label), tone, compact: true}));
            }
            list.appendChild(el('div', {class: 'roles-repo-row'},
                el('div', {class: 'roles-repo-info'}, el('span', {class: 'roles-repo-name'}, name),
                    el('span', {class: 'roles-repo-hint'}, t('users.perRepoAccess'))), actions));
        }
    })();
    return {
        element, ready, values: () => [...selected], dispose() {
            disposed = true;
        }
    };
}
