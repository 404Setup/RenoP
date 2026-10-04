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
 * Empty list / no-data placeholder custom element.
 */
export class EmptyState extends HTMLElement {
    /**
     * @returns {string[]}
     */
    static get observedAttributes() {
        return ['message', 'icon', 'subtext'];
    }

    /**
     * Render when inserted into the DOM.
     * @returns {void}
     */
    connectedCallback() {
        this.render();
    }

    /**
     * Re-render when observed attributes change.
     * @returns {void}
     */
    attributeChangedCallback() {
        this.render();
    }

    /**
     * Rebuild empty-state content from attributes.
     * @returns {void}
     */
    render() {
        const message = this.getAttribute('message') || '';
        const iconContent = this.getAttribute('icon');
        const subtext = this.getAttribute('subtext');

        this.className = 'renop-empty-state';
        this.innerHTML = '';

        if (iconContent) {
            const iconWrap = el('div', {class: 'empty-state-icon'},
                typeof iconContent === 'string' && iconContent.startsWith('<')
                    ? el('span', {innerHTML: iconContent})
                    : iconContent
            );
            this.appendChild(iconWrap);
        }
        if (message) {
            this.appendChild(el('div', {class: 'empty-state-message'}, message));
        }
        if (subtext) {
            this.appendChild(el('div', {class: 'empty-state-subtext'}, subtext));
        }
    }
}

if (!customElements.get('renop-empty-state')) {
    customElements.define('renop-empty-state', EmptyState);
}

/**
 * Create an empty-state placeholder element.
 * @param {string|object} [props={}] - Message string or options object.
 * @param {string} [props.message] - Primary message text.
 * @param {string|Node} [props.icon] - Icon HTML string or node.
 * @param {string} [props.subtext] - Secondary text.
 * @returns {HTMLElement}
 */
export function createEmptyState(props = {}) {
    const emptyState = document.createElement('renop-empty-state');
    if (typeof props === 'string') {
        emptyState.setAttribute('message', props);
    } else {
        if (props.message) emptyState.setAttribute('message', props.message);
        if (props.icon) {
            if (typeof props.icon === 'string') {
                emptyState.setAttribute('icon', props.icon);
            } else {
                emptyState.appendChild(props.icon);
            }
        }
        if (props.subtext) emptyState.setAttribute('subtext', props.subtext);
    }
    return emptyState;
}
