/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {t} from '../i18n.js';
import {getRepositoryFormat} from '../repository-formats.js';
import {decodePathSegment} from './utils.js';
import {copyWithFeedback} from './copy-feedback.js';

let currentSnippets = {};
let snippetUpdateSequence = 0;
let tabsResizeObserver = null;

/**
 * Return the localized tab label for a snippet type.
 * @param {string} type - Catalog snippet tab ID.
 * @returns {string} Localized or product-standard tab label.
 */
function snippetTabLabel(type) {
    switch (type) {
        case 'native-client':
            return t('details.nativeClientTab');
        case 'gradle-kotlin':
            return 'Gradle Kotlin';
        case 'gradle-groovy':
            return 'Gradle Groovy';
        case 'sbt':
            return 'SBT';
        case 'cargo-registry':
            return t('details.cargoRegistryTab');
        case 'cargo-source':
            return t('details.cargoSourceTab');
        case 'cargo-login':
            return t('details.cargoLoginTab');
        case 'cargo-publish':
            return t('details.cargoPublishTab');
        case 'docker-pull':
            return t('details.dockerPullTab');
        case 'docker-tag':
            return t('details.dockerTagTab');
        case 'docker-push':
            return t('details.dockerPushTab');
        case 'docker-login':
            return t('details.dockerLoginTab');
        case 'npm-config':
            return t('details.npmConfigTab');
        case 'npm-install':
            return t('details.npmInstallTab');
        case 'npm-publish':
            return t('details.npmPublishTab');
        default:
            return 'Maven';
    }
}

/**
 * Scroll the activated tab into view and update the sliding indicator.
 * @param {HTMLButtonElement} tab - Activated snippet tab.
 * @returns {void}
 */
function revealSnippetTab(tab) {
    const container = tab.parentElement;
    if (!container) return;
    const left = tab.offsetLeft;
    const right = left + tab.offsetWidth;
    if (left < container.scrollLeft) {
        container.scrollTo({left, behavior: 'smooth'});
    } else if (right > container.scrollLeft + container.clientWidth) {
        container.scrollTo({left: right - container.clientWidth, behavior: 'smooth'});
    }
    syncTabIndicator(container);
}

/**
 * Release an explicit snippet-content height after its transition.
 * @param {HTMLElement} container - Snippet content container.
 * @returns {void}
 */
function releaseSnippetHeight(container) {
    container.style.height = 'auto';
    container.style.transition = '';
    container.__heightTimeout = 0;
}

/**
 * Replace code text and animate the content container to its new height.
 * @param {HTMLElement} container - Snippet content container.
 * @param {HTMLElement} code - Code node.
 * @param {string} snippetType - Selected snippet type.
 * @param {number} oldHeight - Previous content height.
 * @returns {void}
 */
function applySnippetChange(container, code, snippetType, oldHeight) {
    code.textContent = currentSnippets[snippetType] || '';
    container.style.height = 'auto';
    const newHeight = container.getBoundingClientRect().height;
    container.style.height = `${oldHeight}px`;
    void container.offsetHeight;
    code.classList.remove('code-changing');
    container.style.transition = 'height 0.22s cubic-bezier(0.2, 0.8, 0.2, 1)';
    container.style.height = `${newHeight}px`;
    container.__heightTimeout = setTimeout(releaseSnippetHeight.bind(null, container), 220);
    container.__fadeTimeout = 0;
}

/**
 * Activate a snippet tab and animate the code replacement.
 * @param {MouseEvent} event - Snippet tab click.
 * @returns {void}
 */
function handleSnippetTabClick(event) {
    const tab = event.currentTarget;
    if (!(tab instanceof HTMLButtonElement)) return;
    const code = document.getElementById('snippet-code');
    const container = code?.closest('.snippet-content');
    if (!code || !container) return;
    for (const candidate of document.querySelectorAll('.snippet-tab')) {
        candidate.classList.toggle('active', candidate === tab);
    }
    revealSnippetTab(tab);
    if (container.__heightTimeout) clearTimeout(container.__heightTimeout);
    if (container.__fadeTimeout) clearTimeout(container.__fadeTimeout);
    container.style.transition = '';
    const oldHeight = container.getBoundingClientRect().height;
    container.style.height = `${oldHeight}px`;
    code.classList.add('code-changing');
    container.__fadeTimeout = setTimeout(
        applySnippetChange.bind(null, container, code, tab.dataset.snippet || '', oldHeight),
        75
    );
}

/**
 * Keep the active tab indicator aligned after container resizing.
 * @param {ResizeObserverEntry[]} entries - Resize observer entries.
 * @returns {void}
 */
function handleSnippetTabsResize(entries) {
    const container = entries[0]?.target;
    if (container instanceof HTMLElement) syncTabIndicator(container);
}

/**
 * Render format-specific snippet tabs and select the first one.
 * @param {string[]} types - Catalog tab IDs.
 * @returns {void}
 */
function renderSnippetTabs(types) {
    const container = document.querySelector('.snippet-tabs');
    if (!container) return;
    container.innerHTML = '';
    for (let index = 0; index < types.length; index++) {
        const type = types[index];
        const tab = document.createElement('button');
        tab.type = 'button';
        tab.className = `snippet-tab${index === 0 ? ' active' : ''}`;
        tab.dataset.snippet = type;
        tab.textContent = snippetTabLabel(type);
        tab.addEventListener('click', handleSnippetTabClick);
        container.appendChild(tab);
    }
    syncTabIndicator(container);
    if (tabsResizeObserver) tabsResizeObserver.disconnect();
    tabsResizeObserver = new ResizeObserver(handleSnippetTabsResize);
    tabsResizeObserver.observe(container);
}

/**
 * Copy the currently visible snippet and show bounded success feedback.
 * @param {MouseEvent} event - Copy button click.
 * @returns {Promise<void>}
 */
async function copyCurrentSnippet(event) {
    const button = event.currentTarget;
    const code = document.getElementById('snippet-code');
    if (!(button instanceof HTMLButtonElement) || !code) return;
    try {
        await copyWithFeedback(button, code.textContent || '', {copiedLabel: t('details.copied')});
    } catch (error) {
        console.error('Failed to copy repository snippet', error);
    }
}

/**
 * Bind the stable copy button without accumulating listeners across navigation.
 * @returns {void}
 */
function bindCopyButton() {
    const button = document.getElementById('copy-snippet-btn');
    if (!button) return;
    button.removeEventListener('click', copyCurrentSnippet);
    button.addEventListener('click', copyCurrentSnippet);
}

/**
 * Update dependency or registry snippets for the current repository format.
 * @param {string} path - Browser path.
 * @param {Promise<object|null>} [detailsPromise] - Shared repository details request.
 * @returns {Promise<void>}
 */
export async function updateSnippets(path, detailsPromise) {
    const sequence = ++snippetUpdateSequence;
    const card = document.getElementById('repo-snippets-card');
    const colRight = document.querySelector('.col-right');
    const layoutTwoCol = document.querySelector('.layout-two-col');
    const pathParts = path.split('/').filter(Boolean).map(decodePathSegment);

    if (pathParts.length === 0 || pathParts[0] === 'index.html') {
        if (card) card.style.display = 'none';
        if (colRight) colRight.hidden = true;
        if (layoutTwoCol) layoutTwoCol.classList.add('no-sidebar');
        return;
    }

    if (colRight) colRight.hidden = false;
    if (layoutTwoCol) layoutTwoCol.classList.remove('no-sidebar');
    if (card) card.style.display = 'none';

    const code = document.getElementById('snippet-code');
    if (!code) return;
    const title = document.getElementById('details-card-title');
    const subtitle = document.getElementById('details-card-subtitle');
    let details = null;
    try {
        details = detailsPromise ? await detailsPromise : null;
    } catch (error) {
        console.error('Failed to load repository format for snippets', error);
    }
    if (sequence !== snippetUpdateSequence) return;
    const format = getRepositoryFormat(details?.format);
    if (!format.buildSnippets || format.snippetTabs.length === 0) {
        currentSnippets = {};
        return;
    }
    const snippetState = await format.buildSnippets(path, pathParts);
    if (sequence !== snippetUpdateSequence) return;
    currentSnippets = snippetState.snippets;
    if (title) title.textContent = t(snippetState.titleKey);
    if (subtitle) subtitle.textContent = t(snippetState.subtitleKey);
    if (card) card.style.display = '';
    renderSnippetTabs(format.snippetTabs);
    code.textContent = currentSnippets[format.snippetTabs[0]] || '';
    bindCopyButton();
}

/**
 * Position the sliding active-tab indicator under the current snippet tab.
 * @param {HTMLElement|null} container - Snippet tabs container.
 * @returns {void}
 */
function syncTabIndicator(container) {
    if (!container) return;
    const activeTab = container.querySelector('.snippet-tab.active');
    if (!(activeTab instanceof HTMLElement)) return;
    let indicator = container.querySelector('.snippet-tab-indicator');
    if (!indicator) {
        indicator = document.createElement('div');
        indicator.className = 'snippet-tab-indicator';
        container.appendChild(indicator);
    }
    indicator.style.left = `${activeTab.offsetLeft}px`;
    indicator.style.width = `${activeTab.offsetWidth}px`;
}
