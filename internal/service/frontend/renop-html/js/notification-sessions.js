/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {makeCustomSelect} from '@renop/ui/custom-select';
import {collapseElement, expandElement} from '@renop/ui/height-anim';
import {fetchProto} from './api.js';
import {SessionList} from './proto/index.js';
import {t} from './i18n.js';
import {formatTimestamp} from './time.js';

/** Mount the composer's session target control with cancellable, paginated session discovery. */
export function createNotificationSessions(field, host, status) {
    const select = makeCustomSelect([{value: '', label: t('messages.allSessions')}], '');
    select.querySelector('button')?.setAttribute('aria-labelledby', 'message-compose-session-label');
    host.replaceChildren(select);
    let username = '';
    let generation = 0;
    let timer = 0;
    let sessions = [];
    let state = '';
    let visible = false;
    let controller = null;

    /** Render only text labels; session public IDs never carry authentication credentials. */
    function render() {
        select.setOptions([{value: '', label: t('messages.allSessions')}, ...sessions.map(session => ({
            value: session.public_id,
            label: `${String(session.user_agent || t('sessions.unknownDevice')).slice(0, 72)} · ${formatTimestamp(session.last_active)} · ${session.public_id.slice(0, 8)}`,
        }))], '');
        status.textContent = state ? t(state) : '';
    }

    /** Fetch every active session through bounded pages and discard obsolete recipient results. */
    async function load(request, recipient) {
        const signal = controller.signal;
        state = 'messages.sessionsLoading';
        render();
        try {
            let cursor = '';
            const found = [];
            do {
                const {
                    response,
                    data
                } = await fetchProto(`/api/messages/admin/sessions?username=${encodeURIComponent(recipient)}&cursor=${encodeURIComponent(cursor)}`, SessionList, {signal});
                if (request !== generation) return;
                if (!response.ok) throw new Error('Session discovery failed');
                found.push(...(data?.sessions || []));
                cursor = data?.next_cursor || '';
            } while (cursor);
            sessions = found;
            state = sessions.length ? '' : 'messages.sessionsEmpty';
        } catch {
            if (request !== generation) return;
            state = 'messages.sessionsFailed';
        }
        render();
    }

    return {
        /** Reset the target immediately when the recipient or delivery mode changes. */
        update(recipients, enabled) {
            const next = enabled && recipients.length === 1 ? recipients[0].toLowerCase() : '';
            if (next === username) return;
            username = next;
            const request = ++generation;
            clearTimeout(timer);
            controller?.abort();
            controller = next ? new AbortController() : null;
            sessions = [];
            state = '';
            select.setValue('');
            render();
            const show = Boolean(next);
            field.inert = !show;
            if (show !== visible) {
                visible = show;
                void (show ? expandElement(field, {duration: 240}) : collapseElement(field, {duration: 210}));
            }
            if (next) timer = setTimeout(() => void load(request, next), 200);
        },
        /** Return a target only while it belongs to the current single recipient. */
        value(recipients) {
            return visible && recipients.length === 1 && recipients[0].toLowerCase() === username ? select.getValue() : '';
        },
        /** Refresh labels without discarding a confirmed target. */
        translate: render,
    };
}
