/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

const depthKey = 'renopHistoryDepth';
const installed = new WeakSet();

/** Track application history across SPA navigation and reloads, preserving route state. */
export function installBackNavigation(target = window) {
    if (installed.has(target)) return;
    installed.add(target);
    const history = target.history;
    const depth = () => Number.isSafeInteger(history.state?.[depthKey]) ? history.state[depthKey] : 0;
    const push = history.pushState.bind(history);
    const replace = history.replaceState.bind(history);
    replace({...history.state, [depthKey]: depth()}, '');
    history.pushState = (state, title, url) => push({...state, [depthKey]: depth() + 1}, title, url);
    history.replaceState = (state, title, url) => replace({...state, [depthKey]: depth()}, title, url);
}

/** Return to a known previous local page, otherwise replace this page with the home page. */
export function navigateBack(target = window) {
    let localReferrer = false;
    try {
        const referrer = new URL(target.document.referrer);
        localReferrer = referrer.origin === target.location.origin && referrer.href !== target.location.href;
    } catch {
        // A direct entry or a referrer policy may provide no previous-document URL.
    }
    if (target.history.state?.[depthKey] > 0 || (localReferrer && target.history.length > 1)) {
        target.history.back();
        return;
    }
    target.history.replaceState(null, '', '/');
    target.dispatchEvent(new target.PopStateEvent('popstate'));
}

/** Handle page back links while retaining native modified-click behavior. */
export function handleBackClick(event) {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    navigateBack();
}
