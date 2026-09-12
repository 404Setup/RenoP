/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

/** Read a successful response with a strict media type and a bound on actual decoded bytes. */
export async function readResponseBytes(response, contentType, maxBytes) {
    if (!response?.ok || !Number.isSafeInteger(maxBytes) || maxBytes < 0) throw new Error('Invalid binary response');
    const mediaType = String(response.headers.get('Content-Type') || '').split(';', 1)[0].trim().toLowerCase();
    if (!(Array.isArray(contentType) ? contentType.includes(mediaType) : mediaType === contentType)) throw new Error('Unexpected binary response type');
    const declared = Number(response.headers.get('Content-Length'));
    if (Number.isFinite(declared) && declared > maxBytes) throw new Error('Binary response exceeds the size limit');
    const reader = response.body?.getReader();
    if (!reader) throw new Error('Binary response is not streamable');
    const chunks = [];
    let length = 0;
    try {
        while (true) {
            const {done, value} = await reader.read();
            if (done) break;
            length += value.byteLength;
            if (length > maxBytes) throw new Error('Binary response exceeds the size limit');
            chunks.push(value);
        }
    } catch (error) {
        await reader.cancel().catch(() => {
        });
        throw error;
    } finally {
        reader.releaseLock();
    }
    const result = new Uint8Array(length);
    let offset = 0;
    for (const chunk of chunks) {
        result.set(chunk, offset);
        offset += chunk.byteLength;
    }
    return result;
}
