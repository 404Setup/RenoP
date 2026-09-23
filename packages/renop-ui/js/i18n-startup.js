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

/** Load startup dictionaries with an English fallback and an accessible retry when no fallback can be fetched. */
export async function loadInitialLocales(load, preferred, fallback = 'en-US') {
    while (true) {
        const [english, selected] = await Promise.allSettled([load(fallback), load(preferred)]);
        if (english.status === 'fulfilled') {
            return {
                locale: selected.status === 'fulfilled' ? preferred : fallback,
                dictionaries: {[fallback]: english.value, ...(selected.status === 'fulfilled' ? {[preferred]: selected.value} : {})},
            };
        }
        // These bootstrap strings must remain available when every translation request fails.
        await new Promise(resolve => {
            const retry = el('button', {type: 'button', class: 'pill-btn pill-btn--primary'}, 'Retry');
            const notice = el('div', {class: 'locale-load-error', role: 'alert'},
                el('p', {}, 'Language data could not be loaded. Please retry.'), retry);
            retry.addEventListener('click', () => {
                notice.remove();
                resolve();
            }, {once: true});
            document.body.appendChild(notice);
            retry.focus();
        });
    }
}

