/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el} from '@renop/ui/dom';
import {handleBackClick} from './back-navigation.js';
import {t} from './i18n.js';
import {setSafeMarkdown} from './markdown.js';
import {LEGAL_DOCUMENTS, legalPageFromPath} from './legal-consent.js';
import {readLegalTextResponse} from './legal-response.js';

let request, epoch = 0, showing = false;
const siteTitle = document.title;
const documentCache = new Map();
const documentFetchedAt = new Map();
const inflightFetches = new Map();
let cacheEpoch = 0;

/** Fetch and cache one legal document, coalescing concurrent reads. */
async function fetchLegalDocument(documentName, signal) {
    if (documentCache.has(documentName) && Date.now() - documentFetchedAt.get(documentName) < 60000) {
        return documentCache.get(documentName);
    }
    if (inflightFetches.has(documentName)) {
        return inflightFetches.get(documentName);
    }
    const cacheVersion = cacheEpoch;
    const promise = (async () => {
        try {
            const response = await fetch('/api/legal/' + documentName, {
                credentials: 'omit',
                cache: 'default',
                signal: signal || AbortSignal.timeout(15000),
            });
            const content = await readLegalTextResponse(response);
            if (cacheVersion === cacheEpoch) {
                documentCache.set(documentName, content);
                documentFetchedAt.set(documentName, Date.now());
            }
            return content;
        } finally {
            if (cacheVersion === cacheEpoch) inflightFetches.delete(documentName);
        }
    })();
    inflightFetches.set(documentName, promise);
    return promise;
}

/** Render a public legal document as its own page, cancelling stale navigation. */
export async function updateLegalPage(active) {
    request?.abort();
    request = null;
    const revision = ++epoch;
    const container = document.getElementById('tab-content-legal');
    const documentName = legalPageFromPath();
    if (!active || !container || !documentName) {
        if (showing) document.title = siteTitle;
        showing = false;
        return;
    }
    showing = true;
    const title = t(LEGAL_DOCUMENTS[documentName]);
    document.title = title + ' · ' + siteTitle;
    const cached = documentCache.get(documentName);
    const body = el('article', {class: 'legal-document markdown-body', 'aria-busy': cached ? 'false' : 'true'});
    if (cached) {
        setSafeMarkdown(body, cached);
    } else {
        body.textContent = t('privacy.loading');
    }
    container.replaceChildren(
        el('header', {class: 'legal-page-heading'},
            el('h1', {id: 'legal-page-title', tabindex: '-1'}, title),
            el('a', {href: '/', class: 'account-page-back', onclick: handleBackClick}, t('nav.backPrevious'))),
        body,
    );
    document.getElementById('legal-page-title').focus({preventScroll: true});

    const controller = new AbortController();
    request = controller;
    try {
        const content = await fetchLegalDocument(documentName, AbortSignal.any([controller.signal, AbortSignal.timeout(15000)]));
        if (revision !== epoch || controller.signal.aborted) return;
        setSafeMarkdown(body, content);
    } catch {
        if (revision !== epoch || controller.signal.aborted) return;
        body.replaceChildren(el('p', {role: 'alert'}, t('legal.loadFailed')),
            el('button', {type: 'button', class: 'pill-btn pill-btn--soft', onclick: () => void updateLegalPage(true)}, t('offline.retryBtn')));
    } finally {
        if (revision === epoch) body.setAttribute('aria-busy', 'false');
    }
}

/** Route footer policy links without interfering with modified clicks or new tabs. */
export function initializeLegalPages() {
    window.addEventListener('legalConfigurationChanged', () => {
        cacheEpoch++;
        documentCache.clear();
        documentFetchedAt.clear();
        inflightFetches.clear();
        if (showing) void updateLegalPage(true);
    });

    document.addEventListener('mouseover', event => {
        const link = event.target?.closest?.('a[data-legal-link]');
        if (!link) return;
        const name = legalPageFromPath(link.getAttribute('href') || '');
        if (name) {
            void fetchLegalDocument(name).catch(() => {});
        }
    }, {passive: true});

    document.addEventListener('click', event => {
        const link = event.target.closest?.('a[data-legal-link]');
        if (!link || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || link.target) return;
        event.preventDefault();
        window.history.pushState(null, '', link.getAttribute('href'));
        window.dispatchEvent(new PopStateEvent('popstate'));
    });
}
