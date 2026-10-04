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
 * Settings section with icon header and a fields container.
 */
export class Section extends HTMLElement {
    /**
     * @returns {string[]}
     */
    static get observedAttributes() {
        return ['icon', 'title', 'subtitle'];
    }

    /**
     * The fields container element (or the host if missing).
     * @returns {Element}
     */
    get fields() {
        return this.querySelector('.renop-section-fields') || this;
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
     * Rebuild section header and empty fields container.
     * @returns {void}
     */
    render() {
        const iconContent = this.getAttribute('icon');
        const title = this.getAttribute('title') || '';
        const subtitle = this.getAttribute('subtitle') || '';

        this.className = 'renop-section';
        this.innerHTML = '';

        const header = el('div', {class: 'renop-section-header'});
        const iconDiv = el('div', {class: 'renop-section-icon'});
        if (iconContent) {
            if (typeof iconContent === 'string' && iconContent.startsWith('<')) {
                iconDiv.innerHTML = iconContent;
            } else {
                const iconSlot = this.querySelector('[slot="icon"]');
                if (iconSlot) {
                    iconDiv.appendChild(iconSlot.cloneNode(true));
                }
            }
        }
        header.appendChild(iconDiv);

        const metaDiv = el('div', {class: 'renop-section-meta'});
        metaDiv.appendChild(el('h3', {class: 'renop-section-title'}, title));
        if (subtitle) {
            metaDiv.appendChild(el('p', {class: 'renop-section-subtitle'}, subtitle));
        }
        header.appendChild(metaDiv);

        const fieldsDiv = el('div', {class: 'renop-section-fields'});
        this.appendChild(header);
        this.appendChild(fieldsDiv);
    }
}

if (!customElements.get('renop-section')) {
    customElements.define('renop-section', Section);
}

/**
 * Create a section element.
 * @param {object} [props={}] - Section configuration.
 * @param {string|Node} [props.icon] - Icon HTML string or node.
 * @param {string} [props.title] - Section title.
 * @param {string} [props.subtitle] - Optional subtitle.
 * @returns {HTMLElement}
 */
export function createSection(props = {}) {
    const section = document.createElement('renop-section');
    if (props.title) section.setAttribute('title', props.title);
    if (props.subtitle) section.setAttribute('subtitle', props.subtitle);

    if (props.icon) {
        if (typeof props.icon === 'string') {
            section.setAttribute('icon', props.icon);
        } else if (props.icon instanceof Node) {
            const iconSlot = el('span', {slot: 'icon'});
            iconSlot.appendChild(props.icon);
            section.appendChild(iconSlot);
        }
    }

    return section;
}
