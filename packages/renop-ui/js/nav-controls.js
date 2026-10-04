/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el, svg} from './dom.js';

/**
 * Language selector button with its globe and chevron icons.
 * Renders as `#lang-btn` with `#current-lang-name` so the language bootstrap finds it.
 * @returns {HTMLElement}
 */
export function createLangButton() {
    return el('button', {
            'aria-label': 'Select language',
            class: 'lang-btn',
            'data-i18n-aria-label': 'language.selectTitle',
            'data-i18n-title': 'language.selectTitle',
            id: 'lang-btn',
            title: 'Select Language',
            type: 'button',
        },
        svg('svg', {
                'aria-hidden': 'true',
                class: 'lang-icon',
                fill: 'none',
                height: '16',
                stroke: 'currentColor',
                'stroke-linecap': 'round',
                'stroke-linejoin': 'round',
                'stroke-width': '2',
                viewBox: '0 0 24 24',
                width: '16',
            },
            svg('circle', {cx: '12', cy: '12', r: '10'}),
            svg('line', {x1: '2', x2: '22', y1: '12', y2: '12'}),
            svg('path', {d: 'M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z'}),
        ),
        el('span', {class: 'current-lang-name', id: 'current-lang-name'}, 'English'),
        svg('svg', {
                'aria-hidden': 'true',
                class: 'chevron-icon',
                fill: 'none',
                height: '14',
                stroke: 'currentColor',
                'stroke-linecap': 'round',
                'stroke-linejoin': 'round',
                'stroke-width': '2',
                viewBox: '0 0 24 24',
                width: '14',
            },
            svg('polyline', {points: '6 9 12 15 18 9'}),
        ),
    );
}

/**
 * Theme toggle button with its sun and moon icons.
 * Renders as `#theme-toggle` so the shared theme bootstrap binds it.
 * @returns {HTMLElement}
 */
export function createThemeToggle() {
    return el('button', {
            'aria-label': 'Toggle theme',
            class: 'theme-btn',
            'data-i18n-aria-label': 'nav.toggleTheme',
            'data-i18n-title': 'nav.toggleTheme',
            id: 'theme-toggle',
            title: 'Toggle Theme',
            type: 'button',
        },
        svg('svg', {
                'aria-hidden': 'true',
                class: 'theme-icon sun-icon',
                fill: 'none',
                height: '16',
                stroke: 'currentColor',
                'stroke-linecap': 'round',
                'stroke-linejoin': 'round',
                'stroke-width': '2.2',
                viewBox: '0 0 24 24',
                width: '16',
            },
            svg('circle', {cx: '12', cy: '12', r: '5'}),
            svg('line', {x1: '12', x2: '12', y1: '1', y2: '3'}),
            svg('line', {x1: '12', x2: '12', y1: '21', y2: '23'}),
            svg('line', {x1: '4.22', x2: '5.64', y1: '4.22', y2: '5.64'}),
            svg('line', {x1: '18.36', x2: '19.78', y1: '18.36', y2: '19.78'}),
            svg('line', {x1: '1', x2: '3', y1: '12', y2: '12'}),
            svg('line', {x1: '21', x2: '23', y1: '12', y2: '12'}),
            svg('line', {x1: '4.22', x2: '5.64', y1: '19.78', y2: '18.36'}),
            svg('line', {x1: '18.36', x2: '19.78', y1: '5.64', y2: '4.22'}),
        ),
        svg('svg', {
                'aria-hidden': 'true',
                class: 'theme-icon moon-icon',
                fill: 'none',
                height: '16',
                stroke: 'currentColor',
                'stroke-linecap': 'round',
                'stroke-linejoin': 'round',
                'stroke-width': '2.2',
                viewBox: '0 0 24 24',
                width: '16',
            },
            svg('path', {d: 'M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z'}),
        ),
    );
}

/**
 * Vertical divider between navigation controls.
 * @param {string} [id] - Optional id, used by consumers that toggle the divider.
 * @returns {HTMLElement}
 */
export function createNavDivider(id) {
    return el('div', {id, 'aria-hidden': 'true', class: 'divider'});
}

/**
 * Compose the shared language + theme controls group.
 * Consumers append their own trailing controls (sign-in, profile, links).
 * @param {{dividerId?: string, groupId?: string, trailing?: Node[]}} [options={}]
 * @returns {HTMLElement}
 */
export function createNavControls({dividerId, groupId, trailing = []} = {}) {
    return el('div', {class: 'nav-controls-group', id: groupId},
        createLangButton(),
        createThemeToggle(),
        createNavDivider(dividerId),
        ...trailing,
    );
}
