/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {fetchProto} from './api.js';
import {DemoInfo} from './proto/index.js';
import {t} from './i18n.js';

export const demoMode = {enabled: false, temporary: false};
let credentials, loadSequence = 0;

/** Prefill only empty fields, preserving typed or browser-filled credentials. */
export function fillDemoCredentials() {
    if (!demoMode.enabled || !credentials) return;
    const username = document.getElementById('username'), password = document.getElementById('password');
    if (!username || !password || username.value || password.value) return;
    username.value = credentials.username;
    password.value = credentials.password;
}

function renderDemoMode() {
    const banner = document.getElementById('demo-banner');
    if (!banner) return;
    banner.hidden = !demoMode.enabled;
    banner.textContent = demoMode.enabled
        ? `${t(demoMode.temporary ? 'demo.temporary' : 'demo.readOnly')} ${t('demo.credentials')}` : '';
}

/** Load process mode before account-language synchronization and the first login screen. */
export async function initializeDemoMode() {
    const sequence = ++loadSequence;
    try {
        const {response, data} = await fetchProto('/api/demo', DemoInfo);
        if (sequence !== loadSequence || !response.ok || !data) return;
        demoMode.enabled = data.enabled === true;
        demoMode.temporary = demoMode.enabled && data.temporary === true;
        credentials = demoMode.enabled ? {username: data.username, password: data.password} : null;
        renderDemoMode();
        fillDemoCredentials();
    } catch {
        // The existing offline screen owns startup connection failures.
    }
}

document.getElementById('login-form')?.addEventListener('reset', () => queueMicrotask(fillDemoCredentials));
window.addEventListener('languageChanged', renderDemoMode);
window.addEventListener('pageshow', () => void initializeDemoMode());
