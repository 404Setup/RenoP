/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {renop} from './i18n.pb.js';
import {readResponseBytes} from './response-bytes.js';

const {KeyIndex, LocaleCatalog} = renop.i18n.v1;
const maxCatalogBytes = 2 << 20;
const maxKeys = 20000;

/** Create a lazy catalog loader that shares the key index and coalesces concurrent language requests. */
export function createLocaleLoader({index, revision, locales}) {
    let keysPromise;
    const pending = new Map();
    const fetchBytes = async url => {
        const response = await fetch(url, {credentials: 'omit', cache: 'force-cache',
            headers: {Accept: 'application/x-protobuf'}, signal: AbortSignal.timeout(15000)});
        // Keep generic static hosts usable; managed hosts serve .pb with the explicit protobuf media type.
        return readResponseBytes(response, ['application/x-protobuf', 'application/protobuf', 'application/octet-stream'], maxCatalogBytes);
    };
    const loadKeys = () => {
        keysPromise ??= fetchBytes(index).then(bytes => {
            const data = KeyIndex.decode(bytes);
            if (data.revision !== revision || data.keys.length > maxKeys || new Set(data.keys).size !== data.keys.length) {
                throw new Error('Invalid locale key index');
            }
            return data.keys;
        }).catch(error => { keysPromise = undefined; throw error; });
        return keysPromise;
    };
    return function loadLocale(locale) {
        if (!Object.hasOwn(locales, locale)) return Promise.reject(new Error(`unsupported locale: ${locale}`));
        if (!pending.has(locale)) {
            const request = Promise.all([loadKeys(), fetchBytes(locales[locale])]).then(([keys, bytes]) => {
                const data = LocaleCatalog.decode(bytes);
                if (data.locale !== locale || data.key_revision !== revision || data.values.length !== keys.length) {
                    throw new Error('Locale catalog does not match its key index');
                }
                const entries = Object.create(null);
                keys.forEach((key, index) => { entries[key] = data.values[index]; });
                return Object.freeze(entries);
            }).catch(error => { pending.delete(locale); throw error; });
            pending.set(locale, request);
        }
        return pending.get(locale);
    };
}
