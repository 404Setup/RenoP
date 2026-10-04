/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {Callout, createCallout as createCalloutBase} from '@renop/ui/callout';
import {createIcon} from './icon.js';

/**
 * Inline callout banner custom element with icon and message.
 */
export class RenopCallout extends Callout {}

if (!customElements.get('renop-callout')) {
    customElements.define('renop-callout', RenopCallout);
}

/**
 * Create a callout element for notices and warnings with icon rendering.
 * @param {string} [type='info'] - Visual tone (e.g. info, danger, warning).
 * @param {string|Node} [message=''] - Message text or a DOM node to append.
 * @param {string|null} [iconName=null] - Optional icon name override.
 * @returns {HTMLElement}
 */
export function createCallout(type = 'info', message = '', iconName = null) {
    const effectiveIconName = iconName || (type === 'danger' || type === 'warning' ? 'warning' : 'info');
    const iconNode = createIcon(effectiveIconName, {class: 'cfg-callout-icon'});
    return createCalloutBase(type, message, iconNode);
}
