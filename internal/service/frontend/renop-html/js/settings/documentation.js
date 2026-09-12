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
import {createIcon} from '../components/icon.js';
import {t} from '../i18n.js';

const guides = Object.freeze({
    oauth: 'security/oauth-login',
    mail: 'configuration/mail',
    mailPermissions: 'configuration/mail-api-permissions',
    captcha: 'security/captcha',
});

/** Link a settings editor to its maintained setup guide without losing the draft. */
export function createSettingsGuide(kind) {
    return el('a', {
        class: 'pill-btn pill-btn--soft pill-btn--sm',
        href: `https://renop.pkg.one/docs/${guides[kind]}`,
        target: '_blank', rel: 'noopener noreferrer',
    }, createIcon('info'), el('span', {'data-i18n': 'settings.setupGuide'}, t('settings.setupGuide')));
}
