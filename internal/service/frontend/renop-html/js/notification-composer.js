/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {byId, el} from '@renop/ui/dom';
import {makeCustomSelect} from '@renop/ui/custom-select';
import {openModalWithAnim} from '@renop/ui/modal';
import {createToggle} from '@renop/ui/toggle';
import {fetchProto, postProto} from './api.js';
import {closeModalWithAnim} from './app-ui.js';
import {cachedIsManager} from './auth.js';
import {showAlert} from './alert.js';
import {t} from './i18n.js';
import {localizedResponseError} from './response-errors.js';
import {createNotificationSessions} from './notification-sessions.js';
import {SendNotificationRequest, SendNotificationResponse, UserSearchResponse} from './proto/index.js';

let initialized = false;
let severitySelect = null;
let broadcastToggle = null;
let sessionTarget = null;
let recipientSuggestions = [];
let activeRecipientSuggestion = -1;
let recipientSuggestionTimer = 0;
let recipientSuggestionVersion = 0;
let sending = false;
let completionTimer = 0;
let open = false;
let recipientInputEl = null;
let recipientSuggestionsEl = null;

/**
 * Resolve the composer recipient input, caching it while it stays connected.
 * @returns {HTMLInputElement|null}
 */
function recipientInput() {
    if (!recipientInputEl?.isConnected) recipientInputEl = byId('message-compose-recipients');
    return recipientInputEl;
}

/**
 * Resolve the recipient suggestion dropdown, caching it while it stays connected.
 * @returns {HTMLElement|null}
 */
function recipientSuggestionsList() {
    if (!recipientSuggestionsEl?.isConnected) recipientSuggestionsEl = byId('message-recipient-suggestions');
    return recipientSuggestionsEl;
}

/**
 * Initialize the independent administrator notification composer once.
 * @returns {void}
 */
export function initNotificationComposer() {
    if (initialized) return;
    initialized = true;
    byId('message-compose-close')?.addEventListener('click', closeNotificationComposer);
    byId('message-compose-backdrop')?.addEventListener('click', closeNotificationComposer);
    byId('message-compose-cancel')?.addEventListener('click', closeNotificationComposer);
    byId('message-compose-form')?.addEventListener('submit', submitNotification);
    recipientInput()?.addEventListener('input', handleRecipientInput);
    recipientInput()?.addEventListener('keydown', handleRecipientKeydown);
    recipientInput()?.addEventListener('blur', handleRecipientBlur);
    recipientSuggestionsList()?.addEventListener('click', handleRecipientSuggestionClick);
    document.addEventListener('click', handleRecipientDocumentClick);
    window.addEventListener('authChanged', updateComposerVisibility);
    window.addEventListener('languageChanged', handleLanguageChanged);
    initializeSeveritySelect();
    initializeBroadcastToggle();
    sessionTarget = createNotificationSessions(byId('message-compose-session-field'),
        byId('message-compose-session'), byId('message-compose-session-status'));
}

/**
 * Open the administrator notification composer.
 * @returns {void}
 */
export function openNotificationComposer() {
    if (!cachedIsManager || sending) return;
    const modal = byId('message-compose-modal');
    if (!modal) return;
    open = true;
    setSubmitState('idle');
    openModalWithAnim(modal);
    window.requestAnimationFrame(() => recipientInput()?.focus());
}

/**
 * Close and reset the composer.
 * @param {Event} [event] - User close event; ignored during submission.
 * @returns {void}
 */
function closeNotificationComposer(event) {
    if (sending && event?.type) return;
    open = false;
    if (completionTimer) {
        window.clearTimeout(completionTimer);
        completionTimer = 0;
    }
    hideRecipientSuggestions();
    const modal = byId('message-compose-modal');
    if (modal) closeModalWithAnim(modal);
    byId('message-compose-form')?.reset();
    severitySelect?.setValue('info');
    if (broadcastToggle) broadcastToggle.checked = false;
    handleBroadcastChange();
    setSubmitState('idle');
}

/**
 * Mount the styled notification severity select.
 * @returns {void}
 */
function initializeSeveritySelect() {
    const host = byId('message-compose-severity');
    if (!host || severitySelect) return;
    severitySelect = makeCustomSelect(severityOptions(), 'info');
    host.replaceChildren(severitySelect);
}

/**
 * Build localized notification severity options.
 * @returns {Array<{value: string, label: string}>} Select options.
 */
function severityOptions() {
    return [
        {value: 'info', label: t('messages.severityInfo')},
        {value: 'success', label: t('messages.severitySuccess')},
        {value: 'warning', label: t('messages.severityWarning')},
        {value: 'error', label: t('messages.severityError')}
    ];
}

/**
 * Mount the shared broadcast switch.
 * @returns {void}
 */
function initializeBroadcastToggle() {
    const host = byId('message-compose-all');
    if (!host || broadcastToggle) return;
    broadcastToggle = createToggle(false, handleBroadcastToggleChange);
    broadcastToggle.classList.add('message-compose-all-control');
    host.replaceChildren(broadcastToggle);
    syncBroadcastToggleLabel();
}

/**
 * Apply a broadcast switch change.
 * @param {boolean} checked - Current switch state.
 * @returns {void}
 */
function handleBroadcastToggleChange(checked) {
    if (broadcastToggle && broadcastToggle.checked !== checked) broadcastToggle.checked = checked;
    handleBroadcastChange();
}

/**
 * Refresh composer controls after a locale change.
 * @returns {void}
 */
function handleLanguageChanged() {
    severitySelect?.setOptions(severityOptions(), 'info');
    sessionTarget?.translate();
    syncBroadcastToggleLabel();
}

/**
 * Synchronize the broadcast switch's accessible label.
 * @returns {void}
 */
function syncBroadcastToggleLabel() {
    broadcastToggle?.querySelector('input[type="checkbox"]')?.setAttribute('aria-label', t('messages.sendAll'));
}

/**
 * Close the composer when manager authority is lost.
 * @returns {void}
 */
function updateComposerVisibility() {
    if (!cachedIsManager && open) closeNotificationComposer();
}

/**
 * Disable explicit recipients while broadcast delivery is selected.
 * @returns {void}
 */
function handleBroadcastChange() {
    const recipients = recipientInput();
    if (!recipients) return;
    recipients.disabled = Boolean(broadcastToggle?.checked);
    if (recipients.disabled) hideRecipientSuggestions();
    updateSessionTarget();
}

/** Synchronize the session selector with the current recipient and broadcast mode. */
function updateSessionTarget() {
    sessionTarget?.update(parseRecipientNames(recipientInput()?.value || ''),
        open && !broadcastToggle?.checked);
}

/**
 * Debounce recipient prefix searches.
 * @returns {void}
 */
function handleRecipientInput() {
    hideRecipientSuggestions();
    updateSessionTarget();
    const input = recipientInput();
    if (!cachedIsManager || !input || input.disabled || !currentRecipientQuery(input)) return;
    recipientSuggestionTimer = window.setTimeout(fetchRecipientSuggestions, 160);
}

/**
 * Resolve the username token at the input caret.
 * @param {HTMLInputElement} input - Recipient input.
 * @returns {string} Lowercase username prefix.
 */
function currentRecipientQuery(input) {
    const caret = Number.isInteger(input.selectionStart) ? input.selectionStart : input.value.length;
    let start = caret;
    while (start > 0 && !isRecipientSeparator(input.value[start - 1])) start--;
    return input.value.slice(start, caret).trim().toLowerCase();
}

/**
 * Identify a recipient separator.
 * @param {string} character - Input character.
 * @returns {boolean} Whether the character separates usernames.
 */
function isRecipientSeparator(character) {
    return character === ',' || /\s/u.test(character);
}

/**
 * Fetch a bounded recipient suggestion page and discard stale responses.
 * @returns {Promise<void>}
 */
async function fetchRecipientSuggestions() {
    recipientSuggestionTimer = 0;
    const input = recipientInput();
    const query = input ? currentRecipientQuery(input) : '';
    if (!cachedIsManager || !input || input.disabled || !query) return;
    const version = recipientSuggestionVersion;
    try {
        const {response, data} = await fetchProto(
            `/api/messages/admin/users?q=${encodeURIComponent(query)}`, UserSearchResponse);
        if (!response.ok) throw await localizedResponseError(response, 'messages.loadFailed');
        if (version !== recipientSuggestionVersion || query !== currentRecipientQuery(input)) return;
        renderRecipientSuggestions(Array.isArray(data?.users) ? data.users : []);
    } catch (error) {
        if (version !== recipientSuggestionVersion) return;
        console.error('Failed to suggest notification recipients', error);
        hideRecipientSuggestions();
    }
}

/**
 * Render recipient suggestions as accessible dropdown options.
 * @param {Array<string>} users - Suggested usernames.
 * @returns {void}
 */
function renderRecipientSuggestions(users) {
    const dropdown = recipientSuggestionsList();
    const input = recipientInput();
    if (!dropdown || !input) return;
    recipientSuggestions = [];
    dropdown.innerHTML = '';
    for (const rawUser of users) {
        const username = String(rawUser || '');
        if (!username) continue;
        recipientSuggestions.push(username);
        dropdown.appendChild(el('button', {
            type: 'button', class: 'custom-select-dropdown-item message-recipient-suggestion',
            role: 'option', 'aria-selected': 'false', 'data-recipient-suggestion': username
        }, el('span', {class: 'custom-select-item-text'}, username)));
    }
    activeRecipientSuggestion = -1;
    const visible = recipientSuggestions.length > 0;
    dropdown.style.display = visible ? 'block' : 'none';
    input.setAttribute('aria-expanded', String(visible));
}

/**
 * Navigate or accept recipient suggestions from the keyboard.
 * @param {KeyboardEvent} event - Input key event.
 * @returns {void}
 */
function handleRecipientKeydown(event) {
    const dropdown = recipientSuggestionsList();
    if (!dropdown || dropdown.style.display !== 'block' || recipientSuggestions.length === 0) return;
    if (event.key === 'ArrowDown') {
        event.preventDefault();
        setActiveRecipientSuggestion(activeRecipientSuggestion + 1);
    } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        setActiveRecipientSuggestion(activeRecipientSuggestion < 0
            ? recipientSuggestions.length - 1 : activeRecipientSuggestion - 1);
    } else if (event.key === 'Enter') {
        event.preventDefault();
        insertRecipientSuggestion(recipientSuggestions[activeRecipientSuggestion < 0 ? 0 : activeRecipientSuggestion]);
    } else if (event.key === 'Escape') {
        event.preventDefault();
        hideRecipientSuggestions();
    }
}

/**
 * Move the active suggestion with wraparound.
 * @param {number} requestedIndex - Unbounded option index.
 * @returns {void}
 */
function setActiveRecipientSuggestion(requestedIndex) {
    if (recipientSuggestions.length === 0) return;
    activeRecipientSuggestion = (requestedIndex + recipientSuggestions.length) % recipientSuggestions.length;
    let index = 0;
    for (const item of document.querySelectorAll('[data-recipient-suggestion]')) {
        const selected = index === activeRecipientSuggestion;
        item.classList.toggle('is-selected', selected);
        item.setAttribute('aria-selected', String(selected));
        if (selected) item.scrollIntoView({block: 'nearest'});
        index++;
    }
}

/**
 * Accept a clicked username suggestion.
 * @param {MouseEvent} event - Suggestion-list click event.
 * @returns {void}
 */
function handleRecipientSuggestionClick(event) {
    const item = event.target.closest('[data-recipient-suggestion]');
    if (item) insertRecipientSuggestion(item.dataset.recipientSuggestion || '');
}

/**
 * Replace only the username token at the caret.
 * @param {string} username - Selected username.
 * @returns {void}
 */
function insertRecipientSuggestion(username) {
    const input = recipientInput();
    if (!input || !username) return;
    const caret = Number.isInteger(input.selectionStart) ? input.selectionStart : input.value.length;
    let start = caret;
    let end = caret;
    while (start > 0 && !isRecipientSeparator(input.value[start - 1])) start--;
    while (end < input.value.length && !isRecipientSeparator(input.value[end])) end++;
    const prefix = input.value.slice(0, start).replace(/[\s,]+$/u, '');
    const suffix = input.value.slice(end).replace(/^[\s,]+/u, '');
    let value = prefix ? `${prefix}, ${username}` : username;
    const nextCaret = value.length + 2;
    value += suffix ? `, ${suffix}` : ', ';
    input.value = value.slice(0, input.maxLength);
    input.focus();
    input.setSelectionRange(Math.min(nextCaret, input.value.length), Math.min(nextCaret, input.value.length));
    hideRecipientSuggestions();
    updateSessionTarget();
}

/**
 * Hide autocomplete after a click outside the recipient field.
 * @param {MouseEvent} event - Document click event.
 * @returns {void}
 */
function handleRecipientDocumentClick(event) {
    const wrapper = document.querySelector('.message-recipient-input-wrap');
    if (wrapper && !wrapper.contains(event.target)) hideRecipientSuggestions();
}

/** Close recipient suggestions when focus moves to the session selector or another field. */
function handleRecipientBlur(event) {
    if (!document.querySelector('.message-recipient-input-wrap')?.contains(event.relatedTarget)) hideRecipientSuggestions();
}

/**
 * Cancel pending suggestion work and reset accessibility state.
 * @returns {void}
 */
function hideRecipientSuggestions() {
    if (recipientSuggestionTimer) window.clearTimeout(recipientSuggestionTimer);
    recipientSuggestionTimer = 0;
    recipientSuggestionVersion++;
    recipientSuggestions = [];
    activeRecipientSuggestion = -1;
    const dropdown = recipientSuggestionsList();
    const input = recipientInput();
    if (dropdown) {
        dropdown.style.display = 'none';
        dropdown.innerHTML = '';
    }
    if (input) input.setAttribute('aria-expanded', 'false');
}

/**
 * Parse comma-or-whitespace-delimited recipient names.
 * @param {string} value - Raw recipient field.
 * @returns {Array<string>} Non-empty usernames.
 */
function parseRecipientNames(value) {
    return [...new Set(value.toLowerCase().split(/[\s,]+/u).filter(Boolean))];
}

/**
 * Apply busy and completion feedback to composer actions.
 * @param {'idle'|'sending'|'success'} state - Submission state.
 * @returns {void}
 */
function setSubmitState(state) {
    const submit = byId('message-compose-submit');
    const cancel = byId('message-compose-cancel');
    const close = byId('message-compose-close');
    const form = byId('message-compose-form');
    const isSending = state === 'sending';
    byId('message-compose-region')?.toggleAttribute('inert', isSending);
    if (submit) {
        submit.disabled = isSending;
        submit.classList.toggle('is-sending', isSending);
        submit.classList.toggle('is-success', state === 'success');
        submit.setAttribute('aria-busy', String(isSending));
    }
    if (cancel) cancel.disabled = isSending;
    if (close) close.disabled = isSending;
    broadcastToggle?.toggleAttribute('disabled', isSending);
    form?.classList.toggle('is-sent', state === 'success');
}

/**
 * Finish completion feedback before resetting the composer.
 * @returns {void}
 */
function completeSuccessfulNotification() {
    completionTimer = 0;
    closeNotificationComposer();
}

/**
 * Submit one plain-text administrator notification.
 * @param {SubmitEvent} event - Composer form submission.
 * @returns {Promise<void>}
 */
async function submitNotification(event) {
    event.preventDefault();
    if (!cachedIsManager || sending) return;
    const all = Boolean(broadcastToggle?.checked);
    const recipients = parseRecipientNames(recipientInput()?.value || '');
    const severity = severitySelect?.getValue() || 'info';
    const title = byId('message-compose-title')?.value.trim() || '';
    const body = byId('message-compose-body')?.value.trim() || '';
    const session_id = all ? '' : sessionTarget?.value(recipients) || '';
    if ((!all && recipients.length === 0) || !title || !body) {
        showAlert(t('messages.invalidNotification'), 'error');
        return;
    }
    sending = true;
    setSubmitState('sending');
    try {
        const {response, data} = await postProto('/api/messages/admin', SendNotificationRequest,
            {all, recipients, severity, title, body, session_id}, SendNotificationResponse);
        if (!response.ok) throw await localizedResponseError(response, 'messages.sendFailed');
        sending = false;
        setSubmitState('success');
        showAlert(t('messages.sent', {count: Number(data?.sent) || 0}), 'success');
        completionTimer = window.setTimeout(completeSuccessfulNotification, 650);
    } catch (error) {
        sending = false;
        setSubmitState('idle');
        console.error('Failed to send notification', error);
        showAlert(t('messages.sendFailed'), 'error');
    }
}
