/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el} from './dom.js';

/**
 * Build a loading spinner element.
 * Uses the existing `sessions-loading-spinner` class so shared styles apply.
 * @returns {HTMLElement}
 */
export function createLoadingSpinner() {
    return el('div', {class: 'sessions-loading-spinner', 'aria-hidden': 'true'});
}

/**
 * Build a loading state with spinner and label.
 * @param {string} label - Loading message.
 * @param {string} [containerClass] - Optional container class; defaults to generic state wrapper.
 * @returns {HTMLElement}
 */
export function createLoadingState(label, containerClass) {
    return el('div', {class: containerClass || 'loading-state'},
        createLoadingSpinner(),
        el('span', {}, label)
    );
}
