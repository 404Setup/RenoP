/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {EmptyState, createEmptyState as createEmptyStateBase} from '@renop/ui/empty-state';
import {createIcon} from './icon.js';

/**
 * Empty list / no-data placeholder custom element with icon support.
 */
export class RenopEmptyState extends EmptyState {}

if (!customElements.get('renop-empty-state')) {
    customElements.define('renop-empty-state', RenopEmptyState);
}

/**
 * Create an empty-state placeholder element with icon rendering.
 * @param {string|object} [props={}] - Message string or options object.
 * @param {string} [props.message] - Primary message text.
 * @param {string} [props.icon] - Icon name.
 * @param {string} [props.subtext] - Secondary text.
 * @returns {HTMLElement}
 */
export function createEmptyState(props = {}) {
    if (typeof props === 'object' && props.icon && typeof props.icon === 'string') {
        const iconNode = createIcon(props.icon);
        return createEmptyStateBase({...props, icon: iconNode});
    }
    return createEmptyStateBase(props);
}
