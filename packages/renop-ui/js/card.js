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
 * Generic card container with icon and action slot.
 */
export class Card extends HTMLElement {
    /**
     * @returns {string[]}
     */
    static get observedAttributes() {
        return ['icon', 'title', 'description', 'href'];
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
     * Rebuild card content.
     * @returns {void}
     */
    render() {
        const iconContent = this.getAttribute('icon');
        const title = this.getAttribute('title') || '';
        const description = this.getAttribute('description') || '';
        const href = this.getAttribute('href');

        this.className = 'renop-card';
        this.innerHTML = '';

        const content = el('div', {class: 'renop-card-content'});

        if (iconContent) {
            const iconDiv = el('div', {class: 'renop-card-icon'});
            if (typeof iconContent === 'string' && iconContent.startsWith('<')) {
                iconDiv.innerHTML = iconContent;
            } else {
                const iconSlot = this.querySelector('[slot="icon"]');
                if (iconSlot) {
                    iconDiv.appendChild(iconSlot.cloneNode(true));
                }
            }
            content.appendChild(iconDiv);
        }

        const textDiv = el('div', {class: 'renop-card-text'});
        if (title) {
            textDiv.appendChild(el('h3', {class: 'renop-card-title'}, title));
        }
        if (description) {
            textDiv.appendChild(el('p', {class: 'renop-card-description'}, description));
        }
        content.appendChild(textDiv);

        if (href) {
            const link = el('a', {href, class: 'renop-card-link'});
            link.appendChild(content);
            this.appendChild(link);
        } else {
            this.appendChild(content);
        }
    }
}

if (!customElements.get('renop-card')) {
    customElements.define('renop-card', Card);
}

/**
 * Create a card element.
 * @param {object} [props={}] - Card configuration.
 * @param {string|Node} [props.icon] - Icon HTML string or node.
 * @param {string} [props.title] - Card title.
 * @param {string} [props.description] - Card description.
 * @param {string} [props.href] - Optional link URL.
 * @returns {HTMLElement}
 */
export function createCard(props = {}) {
    const card = document.createElement('renop-card');
    if (props.title) card.setAttribute('title', props.title);
    if (props.description) card.setAttribute('description', props.description);
    if (props.href) card.setAttribute('href', props.href);

    if (props.icon) {
        if (typeof props.icon === 'string') {
            card.setAttribute('icon', props.icon);
        } else if (props.icon instanceof Node) {
            const iconSlot = el('span', {slot: 'icon'});
            iconSlot.appendChild(props.icon);
            card.appendChild(iconSlot);
        }
    }

    return card;
}
