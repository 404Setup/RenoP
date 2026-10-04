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
 * Dashboard statistic card custom element.
 */
export class StatCard extends HTMLElement {
    /**
     * @returns {string[]}
     */
    static get observedAttributes() {
        return ['icon', 'label', 'value', 'subtext', 'variant', 'value-id'];
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
     * Rebuild stat card content from attributes.
     * @returns {void}
     */
    render() {
        const iconContent = this.getAttribute('icon');
        const label = this.getAttribute('label') || '';
        const value = this.getAttribute('value') || '-';
        const subtext = this.getAttribute('subtext') || '';
        const variant = this.getAttribute('variant') || 'primary';
        const valueId = this.getAttribute('value-id');

        this.className = `renop-stat-card renop-stat-card--${variant}`;
        this.innerHTML = '';

        if (iconContent) {
            const iconDiv = el('div', {class: 'renop-stat-icon'});
            if (typeof iconContent === 'string' && iconContent.startsWith('<')) {
                iconDiv.innerHTML = iconContent;
            } else {
                const iconSlot = this.querySelector('[slot="icon"]');
                if (iconSlot) {
                    iconDiv.appendChild(iconSlot.cloneNode(true));
                }
            }
            this.appendChild(iconDiv);
        }

        const content = el('div', {class: 'renop-stat-content'});
        const labelEl = el('div', {class: 'renop-stat-label'}, label);
        const valueEl = el('div', {class: 'renop-stat-value'}, value);
        if (valueId) {
            valueEl.id = valueId;
        }
        content.appendChild(labelEl);
        content.appendChild(valueEl);

        if (subtext) {
            const subEl = el('div', {class: 'renop-stat-subtext'}, subtext);
            content.appendChild(subEl);
        }

        this.appendChild(content);
    }
}

if (!customElements.get('renop-stat-card')) {
    customElements.define('renop-stat-card', StatCard);
}

/**
 * Create a statistic card element.
 * @param {object} [props={}] - Card configuration.
 * @param {string|Node} [props.icon] - Icon HTML string or node.
 * @param {string} [props.label] - Stat label.
 * @param {string} [props.value] - Stat value text.
 * @param {string} [props.subtext] - Optional subtext.
 * @param {string} [props.variant] - Visual variant (e.g. primary, success).
 * @param {string} [props.valueId] - Id applied to the value element.
 * @returns {HTMLElement}
 */
export function createStatCard(props = {}) {
    const card = document.createElement('renop-stat-card');
    if (props.label) card.setAttribute('label', props.label);
    if (props.value) card.setAttribute('value', props.value);
    if (props.subtext) card.setAttribute('subtext', props.subtext);
    if (props.variant) card.setAttribute('variant', props.variant);
    if (props.valueId) card.setAttribute('value-id', props.valueId);

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
