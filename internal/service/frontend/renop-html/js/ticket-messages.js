/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {t} from './i18n.js';
import {registerMessageRenderer} from './messages.js';

const resourceTypes = new Set([
    'docker_image', 'npm_package', 'cargo_package', 'maven_artifact', 'maven_domain', 'native_package',
    'docker', 'npm', 'cargo', 'maven', 'maven-domain', 'native', 'conda', 'conda-native', 'apk', 'apt', 'rpm', 'conan',
    'user', 'superteam', 'support', 'git_repo', 'git', 'github', 'gitlab'
]);

const resourceAliases = {
    docker: 'docker_image', npm: 'npm_package', cargo: 'cargo_package',
    maven: 'maven_artifact', 'maven-domain': 'maven_domain',
    git: 'git_repo', github: 'git_repo', gitlab: 'git_repo',
    native: 'native_package', conda: 'native_package', 'conda-native': 'native_package',
    apk: 'native_package', apt: 'native_package', rpm: 'native_package', conan: 'native_package'
};

/**
 * Parse one bounded review notification payload.
 * @param {object} message - Message-center record.
 * @returns {object|null} Validated review payload.
 */
function reviewPayload(message) {
    let payload = message?.payload;
    if (typeof payload === 'string' && payload) {
        try {
            payload = JSON.parse(payload);
        } catch (_) {
            return null;
        }
    }
    if (!payload || typeof payload !== 'object') return null;
    const resourceType = String(payload.resource_type || '');
    const resourceName = String(payload.resource_name || '');
    const status = String(payload.status || '');
    const outcome = String(payload.outcome || '');
    if ((resourceType && !resourceTypes.has(resourceType)) || resourceName.length > 512 ||
        /[\0\r\n]/.test(resourceName) ||
        (status && !['approved', 'rejected', 'cancelled', 'completed', 'closed', 'processed', 'pending', 'unprocessed'].includes(status)) ||
        (outcome && !['upheld', 'dismissed', 'resolved', 'closed'].includes(outcome))) {
        return null;
    }
    return {
        resourceType,
        resourceName,
        status,
        outcome,
        reason: String(payload.decision_reason || '').slice(0, 512)
    };
}

/** @param {string} type - Stored resource type. @returns {string} Localized label. */
function resourceLabel(type) {
    if (!type) return '';
    if (['user', 'superteam', 'support'].includes(type)) {
        const val = t(`ticket.type.${type}`);
        return val && !val.startsWith('ticket.type.') ? val : type;
    }
    const resolved = resourceAliases[type] || type;
    const label = t(`review.type.${resolved}`);
    if (label && !label.startsWith('review.type.')) {
        return label;
    }
    return type;
}

/** @param {string} status - Stable status. @returns {string} Localized status. */
function statusLabel(status) {
    if (!status) return '';
    const ticketKey = `ticket.status.${status}`;
    const tTicket = t(ticketKey);
    if (tTicket && !tTicket.startsWith('ticket.status.')) return tTicket;
    const reviewKey = `review.status.${status}`;
    const tReview = t(reviewKey);
    if (tReview && !tReview.startsWith('review.status.')) return tReview;
    return status;
}

/** @param {string} status - Stable status. @returns {string} Localized title. */
function resultTitle(status) {
    if (status) {
        const reviewTitleKey = `review.message.resultTitle.${status}`;
        const tRev = t(reviewTitleKey);
        if (tRev && !tRev.startsWith('review.message.resultTitle.')) return tRev;
        const stat = statusLabel(status);
        if (stat) {
            return `${t('ticket.message.resultTitle')} · ${stat}`;
        }
    }
    return t('ticket.message.resultTitle');
}

/** @param {string} reason - Stable or custom rejection reason. @returns {string} Localized reason. */
function decisionReason(reason) {
    if (!reason) return '';
    if (reason.startsWith('preset:')) return t(`review.rejectPreset.${reason.slice(7)}`);
    if (reason.startsWith('custom:')) return reason.slice(7);
    return reason;
}

/** @param {object} message - Pending-review message. @returns {{title: string, body: string}} Presentation. */
function renderPendingReview(message) {
    const payload = reviewPayload(message);
    if (message?.kind === 'ticket_pending') {
        const title = t('ticket.message.pendingTitle');
        if (!payload || !payload.resourceName) {
            return {title, body: t('ticket.message.pendingBodyDefault')};
        }
        const type = resourceLabel(payload.resourceType);
        const body = type
            ? t('ticket.message.pendingBody', {type, resource: payload.resourceName})
            : `${payload.resourceName}: ${t('ticket.message.pendingBodyDefault')}`;
        return {title, body};
    }
    if (!payload) return {};
    return {
        title: t('review.message.pendingTitle'),
        body: t('review.message.pendingBody', {
            type: resourceLabel(payload.resourceType), resource: payload.resourceName
        })
    };
}

/** @param {object} message - Review-result message. @returns {{title: string, body: string}} Presentation. */
function renderReviewResult(message) {
    const payload = reviewPayload(message);
    if (message?.kind === 'ticket_report_outcome') {
        const title = payload?.outcome ? t(`ticket.outcome.${payload.outcome}`) : t('ticket.outcome.upheld');
        const body = payload?.resourceName
            ? t('ticket.message.reportOutcomeBodyWithResource', {resource: payload.resourceName})
            : t('ticket.message.reportOutcomeBody');
        return {title, body};
    }

    if (message?.kind === 'ticket_result') {
        if (payload?.outcome) {
            const title = t(`ticket.outcome.${payload.outcome}`);
            const body = payload.resourceName || t('ticket.message.resultBodyDefault');
            return {title, body};
        }
        const title = resultTitle(payload?.status);
        if (!payload || !payload.resourceName) {
            return {title, body: t('ticket.message.resultBodyDefault')};
        }
        const type = resourceLabel(payload.resourceType);
        const stat = statusLabel(payload.status);
        let body = type && stat
            ? t('ticket.message.resultBody', {type, resource: payload.resourceName, status: stat})
            : `${payload.resourceName}: ${stat || t('ticket.message.resultBodyDefault')}`;
        const reason = decisionReason(payload.reason);
        if (reason) body += ` ${t('review.message.resultReason', {reason})}`;
        return {title, body};
    }

    if (!payload) return {};
    if (payload.outcome) return {
        title: t(`ticket.outcome.${payload.outcome}`), body: payload.resourceName
    };
    if (!payload.status) return {};
    let body = t('review.message.resultBody', {
        type: resourceLabel(payload.resourceType), resource: payload.resourceName,
        status: statusLabel(payload.status)
    });
    const reason = decisionReason(payload.reason);
    if (reason) body += ` ${t('review.message.resultReason', {reason})}`;
    return {title: t(`review.message.resultTitle.${payload.status}`), body};
}

registerMessageRenderer('review_pending', renderPendingReview);
registerMessageRenderer('review_result', renderReviewResult);
registerMessageRenderer('ticket_pending', renderPendingReview);
registerMessageRenderer('ticket_result', renderReviewResult);
registerMessageRenderer('ticket_report_outcome', renderReviewResult);
