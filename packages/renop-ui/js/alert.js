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
 * Dismissible alert banner custom element.
 */
export class Alert extends HTMLElement {
    /**
     * @returns {string[]}
     */
    static get observedAttributes() {
        return ['type', 'message'];
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
     * @param {string} name - Attribute name.
     * @param {string|null} oldValue - Previous value.
     * @param {string|null} newValue - New value.
     * @returns {void}
     */
    attributeChangedCallback(name, oldValue, newValue) {
        if (oldValue !== newValue) {
            this.render();
        }
    }

    /**
     * Rebuild alert content from attributes.
     * Consumers should provide icon content via slot or custom rendering.
     * @returns {void}
     */
    render() {
        const type = this.getAttribute('type') || 'info';
        const message = this.getAttribute('message') || '';

        const iconSlot = this.querySelector('[slot="icon"]');
        const closeSlot = this.querySelector('[slot="close-icon"]');

        this.className = `alert alert-${type}`;
        this.innerHTML = '';

        const iconWrap = el('div', {class: 'alert-icon'});
        if (iconSlot) {
            while (iconSlot.firstChild) {
                iconWrap.appendChild(iconSlot.firstChild);
            }
        }

        const contentWrap = el('div', {class: 'alert-content'});
        const textSpan = el('span', {class: 'alert-message'}, message);
        contentWrap.appendChild(textSpan);

        const closeBtn = el('button', {
            class: 'alert-close',
            type: 'button',
            ariaLabel: 'Close'
        });
        if (closeSlot && closeSlot.firstChild) {
            while (closeSlot.firstChild) {
                closeBtn.appendChild(closeSlot.firstChild);
            }
        } else {
            closeBtn.textContent = '×';
        }
        closeBtn.onclick = (e) => {
            e.stopPropagation();
            this.dismiss();
        };

        const progressWrap = el('div', {class: 'alert-progress'});
        const progressBar = el('div', {class: 'alert-progress-bar'});
        progressWrap.appendChild(progressBar);

        this.appendChild(iconWrap);
        this.appendChild(contentWrap);
        this.appendChild(closeBtn);
        this.appendChild(progressWrap);
    }

    /**
     * Animate out and remove the alert from the DOM.
     * @returns {void}
     */
    dismiss() {
        if (this.classList.contains('alert-leaving')) return;
        this.classList.add('alert-leaving');
        setTimeout(() => {
            if (this.parentNode) {
                this.parentNode.removeChild(this);
            }
        }, 350);
    }
}

if (!customElements.get('renop-alert')) {
    customElements.define('renop-alert', Alert);
}

/**
 * Create a dismissible alert element.
 * @param {string} message - Alert message text.
 * @param {string} [type='info'] - Visual tone (e.g. info, success, warning, danger).
 * @param {object} [options={}] - Additional options.
 * @param {Node} [options.icon] - Optional icon element to insert.
 * @param {Node} [options.closeIcon] - Optional close icon element.
 * @returns {HTMLElement}
 */
export function createAlert(message, type = 'info', options = {}) {
    const alertEl = document.createElement('renop-alert');
    alertEl.setAttribute('type', type);
    alertEl.setAttribute('message', message);

    if (options.icon) {
        const iconSlot = el('span', {slot: 'icon'});
        iconSlot.appendChild(options.icon);
        alertEl.appendChild(iconSlot);
    }

    if (options.closeIcon) {
        const closeSlot = el('span', {slot: 'close-icon'});
        closeSlot.appendChild(options.closeIcon);
        alertEl.appendChild(closeSlot);
    }

    return alertEl;
}
