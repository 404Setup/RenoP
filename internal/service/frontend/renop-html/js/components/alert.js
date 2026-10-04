/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {Alert, createAlert as createAlertBase} from '@renop/ui/alert';
import {createIcon} from './icon.js';

/**
 * Dismissible alert banner custom element with icon support.
 */
export class RenopAlert extends Alert {}

if (!customElements.get('renop-alert')) {
    customElements.define('renop-alert', RenopAlert);
}

/**
 * Create a dismissible alert element with icon rendering.
 * @param {string} message - Alert message text.
 * @param {string} [type='info'] - Visual tone (e.g. info, success, warning, danger).
 * @returns {HTMLElement}
 */
export function createAlert(message, type = 'info') {
    let iconName = 'info';
    if (type === 'success') iconName = 'success';
    else if (type === 'error' || type === 'danger') iconName = 'alertCircle';
    else if (type === 'warning') iconName = 'warning';

    const icon = createIcon(iconName);
    const closeIcon = createIcon('close');

    return createAlertBase(message, type, {icon, closeIcon});
}
