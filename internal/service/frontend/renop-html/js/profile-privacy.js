/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el} from '@renop/ui/dom';
import {createToggle, createFieldRow, createActionButton, createIcon} from './components.js';
import {createJSONClient} from './api.js';
import {syncUserProfile} from './user-profiles.js';
import {t} from './i18n.js';

/** Render an owner-controlled privacy draft independently from profile links and identity. */
export function createProfilePrivacyEditor(profile) {
    let privateProfile = profile.private === true;
    const toggle = createToggle(privateProfile, value => { privateProfile = value; });
    const save = createJSONClient('/api/auth/profile/privacy', 'profile.privacySaveFailed');
    const card = el('details', {class: 'profile-settings-section profile-privacy-card profile-collapsible-card'});
    const button = createActionButton(t('common.save'), async () => {
        const updated = await save('', {method: 'PUT', json: {user_id: profile.user_id, private: privateProfile}});
        if (!card.isConnected || localStorage.getItem('username') !== profile.username) return false;
        Object.assign(profile, updated);
        syncUserProfile(updated);
    }, {errorKey: 'profile.privacySaveFailed', successKey: 'profile.privacySaved'});
    card.append(
        el('summary', {class: 'profile-section-card-header profile-collapsible-summary'},
            el('div', {class: 'profile-section-icon'}, createIcon('user')),
            el('div', {class: 'profile-section-meta'},
                el('h3', {class: 'profile-section-title'}, t('profile.privacyTitle')),
                el('p', {class: 'profile-section-desc'}, t('profile.privacyHint'))),
            createIcon('chevronDown', {class: 'profile-collapse-chevron'})),
        el('div', {class: 'profile-collapsible-content', hidden: true},
            el('div', {class: 'profile-section-body'},
                createFieldRow(t('profile.privateProfile'), '', toggle, 'cfg-field-row--toggle'),
                el('div', {class: 'profile-identity-actions'}, button))));
    return card;
}
