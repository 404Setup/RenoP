/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {cachedIsManager} from '../auth.js';
import {apiRequest} from '../api.js';
import {createIcon, runButtonAction} from '../components.js';
import {openUserBanDialog} from './ban.js';
import {t} from '../i18n.js';
import {el} from '@renop/ui/dom';

/** Add the existing suspension editor directly to another user's public profile for administrators. */
export function createProfileBanAction(profile) {
    if (!cachedIsManager || profile.own_profile || Number(profile.deleted_at) > 0) return null;
    const button = el('button', {type: 'button', class: 'pill-btn pill-btn--danger pill-btn--sm manager-only'},
        createIcon('warning'), el('span', {}, t('users.manageBan')));
    const refresh = async () => {
        try {
            const response = await apiRequest(`/api/tokens/${encodeURIComponent(profile.username)}/ban`);
            if (!response.ok) return;
            const status = await response.json();
            if (!button.isConnected || !cachedIsManager) return;
            button.disabled = !status.ban && status.protected_role;
            button.title = button.disabled ? t('users.banProtected') : '';
            button.replaceChildren(createIcon('warning'), el('span', {}, t(status.ban ? 'users.unban' : 'users.ban')));
        } catch {
            // Opening the editor retries a failed status load through the normal error UI.
        }
    };
    button.addEventListener('click', () => {
        if (cachedIsManager) void runButtonAction(button, () => openUserBanDialog({name: profile.username}, refresh));
    });
    void refresh();
    return button;
}
