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
 * Inline callout banner custom element with icon and message.
 */
export class Callout extends HTMLElement {
    /**
     * @returns {string[]}
     */
    static get observedAttributes() {
        return ['type', 'icon', 'message'];
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
     * Rebuild callout content from attributes.
     * @returns {void}
     */
    render() {
        const type = this.getAttribute('type') || 'info';
        const iconContent = this.getAttribute('icon');
        const message = this.getAttribute('message') || '';

        this.className = `renop-callout renop-callout--${type}`;
        this.innerHTML = '';

        if (iconContent) {
            const iconEl = el('div', {class: 'renop-callout-icon'});
            if (typeof iconContent === 'string' && iconContent.startsWith('<')) {
                iconEl.innerHTML = iconContent;
            } else {
                iconEl.textContent = iconContent;
            }
            this.appendChild(iconEl);
        }

        const textDiv = el('div', {class: 'renop-callout-text'});
        if (message) {
            textDiv.textContent = message;
        }
        this.appendChild(textDiv);
    }
}

if (!customElements.get('renop-callout')) {
    customElements.define('renop-callout', Callout);
}

/**
 * Create a callout element for notices and warnings.
 * @param {string} [type='info'] - Visual tone (e.g. info, danger, warning, neutral).
 * @param {string|Node} [message=''] - Message text or a DOM node to append.
 * @param {string|Node|null} [iconContent=null] - Optional icon HTML string or node.
 * @returns {HTMLElement}
 */
export function createCallout(type = 'info', message = '', iconContent = null) {
    const callout = document.createElement('renop-callout');
    callout.setAttribute('type', type);

    if (iconContent) {
        if (typeof iconContent === 'string') {
            callout.setAttribute('icon', iconContent);
        } else if (iconContent instanceof Node) {
            const iconSlot = el('div', {class: 'renop-callout-icon'});
            iconSlot.appendChild(iconContent);
            callout.appendChild(iconSlot);
        }
    }

    if (typeof message === 'string') {
        callout.setAttribute('message', message);
    } else if (message instanceof Node) {
        const textDiv = el('div', {class: 'renop-callout-text'});
        textDiv.appendChild(message);
        callout.appendChild(textDiv);
    }

    return callout;
}
