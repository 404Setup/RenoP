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
import {apiRequest, fetchProto} from './api.js';
import {RenopDialog} from './components/dialog.js';
import {FileDetails} from './proto/index.js';
import {t} from './i18n.js';

/** Load bounded suggestions from existing visibility-filtered resource APIs. */
async function loadTargets(kind, signal) {
    if (kind === 'repository') {
        const {response, data} = await fetchProto('/api/repositories/details', FileDetails, {signal});
        if (!response.ok) throw new Error('Target catalog unavailable');
        return (data?.files || []).map(entry => entry.name).slice(0, 256);
    }
    const username = encodeURIComponent(localStorage.getItem('username') || '');
    const formats = kind === 'domain' ? ['maven'] : kind === 'team' ? ['cargo', 'npm', 'docker', 'maven'] : ['cargo', 'npm', 'docker'];
    const lists = await Promise.all(formats.map(async format => {
        const response = await apiRequest(`/api/users/${username}/memberships?format=${format}`, {
            signal,
            cache: 'no-store'
        });
        if (!response.ok) throw new Error('Target catalog unavailable');
        const result = await response.json();
        return (result.memberships || []).slice(0, 256).map(entry => format === 'maven'
            ? (kind === 'team' ? 'domain/' : '') + entry.name
            : (kind === 'team' ? 'package/' : '') + entry.repository + '/' + entry.name);
    }));
    if (kind === 'team') {
        const response = await apiRequest(`/api/users/${username}/super-teams?limit=50&offset=0`, {
            signal,
            cache: 'no-store'
        });
        if (!response.ok) throw new Error('Target catalog unavailable');
        const result = await response.json();
        lists.push((result.teams || []).map(team => 'global/' + team.prefix));
    }
    return [...new Set(lists.flat())].slice(0, 256);
}

/** Edit an isolated target draft; only explicit confirmation changes the permission card. */
export function openAPITokenTargets({label, kind, targets, limit, onConfirm}) {
    if (document.getElementById('profile-api-token-target-dialog')) return;
    const controller = new AbortController();
    const selected = new Set(targets);
    const choices = new Map();
    const all = el('input', {type: 'radio', name: 'api-token-target-mode', checked: targets.length === 0});
    const restricted = el('input', {type: 'radio', name: 'api-token-target-mode', checked: targets.length > 0});
    const cards = el('div', {class: 'profile-api-token-target-cards'});
    const error = el('p', {class: 'password-recovery-error', role: 'alert'});
    const loading = el('p', {class: 'profile-security-hint', role: 'status'}, t('common.loading'));
    const input = el('textarea', {
        class: 'profile-api-token-targets', rows: 2, maxlength: 4096,
        'aria-label': t(`profile.apiTokenTargetPlaceholder.${kind}`),
        placeholder: t(`profile.apiTokenTargetPlaceholder.${kind}`),
        'data-i18n-placeholder': `profile.apiTokenTargetPlaceholder.${kind}`,
    });

    /** Add a selectable exact target, preserving drafts when suggestions arrive. */
    function addChoice(value) {
        if (!value || choices.has(value) || choices.size >= 384) return;
        const checkbox = el('input', {type: 'checkbox', value, checked: selected.has(value)});
        checkbox.addEventListener('change', () => {
            restricted.checked = true;
            if (checkbox.checked) selected.add(value);
            else selected.delete(value);
        });
        choices.set(value, checkbox);
        cards.appendChild(el('label', {class: 'profile-api-token-target-card'}, checkbox, el('code', {}, value)));
    }

    targets.forEach(addChoice);
    const add = el('button', {
        type: 'button', class: 'pill-btn pill-btn--soft', onclick: () => {
            const values = [...new Set(input.value.split(/[\n,]+/u).map(value => value.trim()).filter(Boolean))];
            if (new Set([...selected, ...values]).size > limit || new Set([...choices.keys(), ...values]).size > 384) {
                error.textContent = t('profile.apiTokenTargetLimitReached', {limit});
                return;
            }
            values.forEach(value => {
                selected.add(value);
                addChoice(value);
                choices.get(value).checked = true;
            });
            if (values.length) restricted.checked = true;
            input.value = '';
            error.textContent = '';
        }
    }, t('common.add'));
    const body = el('div', {class: 'profile-api-token-target-dialog'},
        el('label', {class: 'profile-api-token-target-card'}, all,
            el('span', {'data-i18n': 'profile.apiTokenAllTargets'}, t('profile.apiTokenAllTargets'))),
        el('label', {class: 'profile-api-token-target-card'}, restricted,
            el('span', {'data-i18n': 'profile.apiTokenTargetLimit'}, t('profile.apiTokenTargetLimit'))),
        el('p', {
            class: 'profile-security-hint',
            'data-i18n': 'profile.apiTokenTargetsHelp'
        }, t('profile.apiTokenTargetsHelp')),
        loading, cards, input, add, error);
    void RenopDialog.show({
        id: 'profile-api-token-target-dialog', maxWidth: '600px', icon: 'fileKey',
        className: 'profile-api-token-target-modal', glass: false,
        title: label, body,
        footer: [
            {text: t('common.cancel'), className: 'action-btn', onClick: (event, dialog) => dialog.close(false)},
            {
                text: t('confirm.confirmBtn'), className: 'action-btn primary-btn', onClick: (event, dialog) => {
                    if (input.value.trim()) {
                        add.click();
                        if (input.value.trim()) return;
                    }
                    if (restricted.checked && selected.size === 0) {
                        error.textContent = t('profile.apiTokenTargetRequired');
                        return;
                    }
                    if (restricted.checked && selected.size > limit) {
                        error.textContent = t('profile.apiTokenTargetLimitReached', {limit});
                        return;
                    }
                    onConfirm(all.checked ? [] : [...selected]);
                    dialog.close(true);
                }
            },
        ],
        onClose: () => controller.abort(),
    });
    void loadTargets(kind, AbortSignal.any([controller.signal, AbortSignal.timeout(15000)])).then(values => {
        if (controller.signal.aborted) return;
        values.forEach(addChoice);
        loading.remove();
    }).catch(() => {
        if (!controller.signal.aborted) loading.textContent = t('profile.apiTokenTargetsLoadFailed');
    });
}
