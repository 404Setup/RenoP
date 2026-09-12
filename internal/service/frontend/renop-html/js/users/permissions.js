/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {t} from '../i18n.js';
import {createPermissionEditor} from '../permission-editor.js';

let editor;

/** Return this account editor's draft, including permissions for unavailable repositories. */
export function selectedUserPermissions() {
    return editor?.values() || [];
}

/** Refresh the account editor's selected-permission counter. */
export function updateUserPermissionCount() {
    const count = document.getElementById('user-permissions-count');
    if (!count) return;
    const selected = selectedUserPermissions().length;
    count.textContent = t('users.rolesSelected', {count: selected});
    count.dataset.count = String(selected);
}

/** Mount an independent editor and cancel obsolete asynchronous loads. */
export async function populateUserPermissions(permissions = []) {
    const panel = document.getElementById('user-permissions-grid');
    if (!panel) return;
    cancelUserPermissionLoad();
    editor = createPermissionEditor(permissions, updateUserPermissionCount);
    panel.replaceChildren(editor.element);
    updateUserPermissionCount();
    await editor.ready;
}

/** Stop an obsolete permission editor from applying a delayed repository response. */
export function cancelUserPermissionLoad() {
    editor?.dispose();
    editor = undefined;
}
