/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {StatCard, createStatCard as createStatCardBase} from '@renop/ui/stat-card';
import {createIcon} from './icon.js';

/**
 * Dashboard statistic card custom element with icon support.
 */
export class RenopStatCard extends StatCard {}

if (!customElements.get('renop-stat-card')) {
    customElements.define('renop-stat-card', RenopStatCard);
}

/**
 * Create a statistic card element with icon rendering.
 * @param {object} [props={}] - Card configuration.
 * @param {string} [props.icon] - Icon name.
 * @param {string} [props.label] - Stat label.
 * @param {string} [props.value] - Stat value text.
 * @param {string} [props.subtext] - Optional subtext.
 * @param {string} [props.variant] - Visual variant (e.g. primary).
 * @param {string} [props.valueId] - Id applied to the value element.
 * @returns {HTMLElement}
 */
export function createStatCard(props = {}) {
    const iconNode = props.icon ? createIcon(props.icon) : null;
    return createStatCardBase({...props, icon: iconNode});
}
