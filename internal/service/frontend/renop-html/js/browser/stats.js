/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {awaitLayoutSettled, collapseElement, expandElement, morphElementHeight} from '@renop/ui/height-anim';
import {el} from '@renop/ui/dom';
import {formatBytes} from './utils.js';
import {fetchProto} from '../api.js';
import {t} from '../i18n.js';
import {createButton, createMirrorCard} from '../components.js';
import {closeModalWithAnim} from '../app-ui.js';
import {RepoDetailsResponse} from '../proto/index.js';

const REPO_STATS_MARGIN = '1.5rem';
let statsLoadSeq = 0;

/**
 * Create (once) and return the repository mirrors modal element.
 * @returns {HTMLElement}
 */
function ensureMirrorsModal() {
    let modal = document.getElementById('repo-mirrors-modal');
    if (modal) return modal;

    const close = () => closeModalWithAnim(modal);

    const closeButton = el('button', {
        class: 'close-btn',
        type: 'button',
        'aria-label': t('details.closeMirrors'),
        onClick: close
    }, '\u00d7');

    const header = el('div', {class: 'modal-header'},
        el('h3', {id: 'repo-mirrors-modal-title', class: 'modal-title'}, t('details.mirrors')),
        el('p', {id: 'repo-mirrors-modal-subtitle', class: 'mirrors-modal-subtitle'}, t('details.mirrorsSubtitle'))
    );

    const body = el('div', {id: 'repo-mirrors-modal-content', class: 'modal-body mirrors-modal-body'});

    const backdrop = el('div', {class: 'modal-backdrop', onClick: close});
    const content = el('div', {class: 'modal-content mirrors-modal-content'}, closeButton, header, body);

    modal = el('div', {
        id: 'repo-mirrors-modal',
        class: 'modal',
        style: 'display: none',
        role: 'dialog',
        'aria-modal': 'true',
        'aria-labelledby': 'repo-mirrors-modal-title'
    }, backdrop, content);

    document.body.appendChild(modal);
    return modal;
}

/**
 * Populate and show the mirrors modal for the given mirror list.
 * @param {Array<object>} mirrors
 * @returns {void}
 */
function openMirrorsModal(mirrors) {
    const modal = ensureMirrorsModal();
    const body = modal.querySelector('#repo-mirrors-modal-content');
    const subtitle = modal.querySelector('#repo-mirrors-modal-subtitle');
    if (subtitle) {
        subtitle.textContent = t('details.mirrorsConfiguredCount', {count: mirrors.length});
    }
    body.replaceChildren(...mirrors.map(mirror => createMirrorCard(mirror, true)));
    modal.style.display = 'flex';
    if (window.updateModalInertState) window.updateModalInertState();
    modal.querySelector('.close-btn').focus();
}

/**
 * Build a single label/value row for the repo stats card.
 * @param {string} label
 * @param {string} value
 * @param {{nested?: boolean, strong?: boolean}} [options]
 * @returns {HTMLElement}
 */
function buildStatRow(label, value, options = {}) {
    const className = 'repo-stat-row'
        + (options.nested ? ' repo-stat-row--nested' : '')
        + (options.strong ? ' repo-stat-row--strong' : '');

    return el('div', {class: className},
        el('span', {class: 'repo-stat-label'}, label),
        el('span', {class: 'repo-stat-value'}, value)
    );
}

/**
 * Render size/file counts and optional mirrors section into the stats container.
 * @param {HTMLElement} repoStatsContent
 * @param {object} data repo details payload
 * @returns {void}
 */
function renderRepoStatsContent(repoStatsContent, data) {
    const mirrors = Array.isArray(data.mirrors) ? data.mirrors : [];
    repoStatsContent.replaceChildren();

    const statsBlock = el('div', {
        class: 'repo-stats-block' + (mirrors.length > 0 ? ' repo-stats-block--with-mirrors' : '')
    },
        buildStatRow(t('stats.repoSize'), formatBytes(data.total_size), {strong: true}),
        buildStatRow(t('stats.artifactsSize'), formatBytes(data.artifact_size), {nested: true}),
        buildStatRow(t('stats.metadataSize'), formatBytes(data.metadata_size), {nested: true}),
        buildStatRow(
            t('stats.totalFiles'),
            t('stats.totalFilesFormat', {
                total: data.total_files,
                artifacts: data.artifact_count,
                metadata: data.metadata_count
            }),
            {strong: true}
        )
    );
    repoStatsContent.appendChild(statsBlock);

    if (mirrors.length > 0) {
        const heading = el('div', {class: 'repo-mirrors-heading'},
            el('h4', {class: 'repo-mirrors-title'}, t('details.mirrors')),
            el('span', {class: 'repo-mirrors-count'}, String(mirrors.length))
        );

        const mirrorList = el('div', {class: 'repo-mirrors-list'}, createMirrorCard(mirrors[0]));

        if (mirrors.length > 1) {
            const expandButton = createButton(t('details.viewAllMirrors', {count: mirrors.length}), {
                class: 'pill-btn pill-btn--soft pill-btn--sm mirrors-expand-btn',
                icon: 'network',
                iconProps: {width: '14', height: '14'},
                onClick: () => openMirrorsModal(mirrors)
            });
            mirrorList.appendChild(expandButton);
        }

        const section = el('div', {class: 'repo-mirrors-section'}, heading, mirrorList);
        repoStatsContent.appendChild(section);
    }
}

/**
 * Whether the repo stats card is currently shown.
 * @param {HTMLElement} card
 * @returns {boolean}
 */
function isRepoStatsVisible(card) {
    return card.classList.contains('is-visible')
        && card.style.display !== 'none'
        && getComputedStyle(card).display !== 'none';
}

/**
 * Collapse and hide the repository stats card.
 * @returns {Promise<void>}
 */
export function hideRepoStats() {
    const repoStatsCard = document.getElementById('repo-stats-card');
    if (!repoStatsCard) return Promise.resolve();
    statsLoadSeq += 1;
    return collapseElement(repoStatsCard, {
        duration: 300,
        marginTop: true,
    });
}

/**
 * Fetch repository details once so multiple consumers can share the result.
 * @param {string} repoName
 * @returns {Promise<object|null>}
 */
export async function fetchRepoDetails(repoName) {
    const {
        response,
        data
    } = await fetchProto(`/api/repositories/repo-details/${encodeURIComponent(repoName)}`, RepoDetailsResponse);
    return response.ok && data ? data : null;
}

/**
 * Fetch and display repository size/mirror stats for `repoName`.
 * @param {string} repoName
 * @param {Promise<object|null>} [detailsPromise]
 * @returns {Promise<void>}
 */
export async function updateRepoStats(repoName, detailsPromise) {
    const repoStatsCard = document.getElementById('repo-stats-card');
    const repoStatsContent = document.getElementById('repo-stats-content');
    if (!repoStatsCard || !repoStatsContent) return;

    const seq = ++statsLoadSeq;

    try {
        const data = await (detailsPromise || fetchRepoDetails(repoName));
        if (seq !== statsLoadSeq) return;

        if (!data) {
            await hideRepoStats();
            return;
        }

        if (seq !== statsLoadSeq) return;

        const wasVisible = isRepoStatsVisible(repoStatsCard);

        if (wasVisible) {
            repoStatsCard.classList.add('is-updating');
            await morphElementHeight(repoStatsCard, () => {
                if (seq !== statsLoadSeq) return;
                renderRepoStatsContent(repoStatsContent, data);
            }, {duration: 340});
            if (seq === statsLoadSeq) {
                repoStatsCard.classList.remove('is-updating');
            }
        } else {
            renderRepoStatsContent(repoStatsContent, data);
            // The sidebar column may still be widening; measuring now would overshoot the height.
            await awaitLayoutSettled(repoStatsCard);
            if (seq !== statsLoadSeq) return;
            await expandElement(repoStatsCard, {
                duration: 360,
                marginTop: REPO_STATS_MARGIN,
            });
        }
    } catch (e) {
        if (seq !== statsLoadSeq) return;
        console.error('Failed to load repo stats', e);
        await hideRepoStats();
    }
}
