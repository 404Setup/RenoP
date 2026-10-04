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
import {createToggle} from '@renop/ui/toggle';
import {apiRequest} from './api.js';
import {showAlert} from './alert.js';
import {createIcon, RenopDialog} from './components.js';
import {t} from './i18n.js';
import {caughtErrorMessage, localizedResponseError} from './response-errors.js';

/**
 * Format megabytes to readable string (e.g. 1.25 GB or 300 MB).
 * @param {number} mb
 * @returns {string}
 */
export function formatMB(mb) {
    if (mb == null || isNaN(mb)) return '0 MB';
    if (mb >= 1024) {
        const gb = mb / 1024;
        return (gb % 1 === 0 ? gb.toFixed(0) : gb.toFixed(2)) + ' GB';
    }
    return (Number(mb) % 1 === 0 ? Number(mb).toFixed(0) : Number(mb).toFixed(2)) + ' MB';
}

/**
 * Format duration seconds to readable string (e.g. 1h, 30m, 1d).
 * @param {number} seconds
 * @returns {string}
 */
export function formatInterval(seconds) {
    if (!seconds || seconds <= 0) return '0s';
    if (seconds % 86400 === 0) return `${seconds / 86400}d`;
    if (seconds % 3600 === 0) return `${seconds / 3600}h`;
    if (seconds % 60 === 0) return `${seconds / 60}m`;
    return `${seconds}s`;
}

/**
 * Render traffic quota usage for a profile or user modal.
 * @param {object|null|undefined} status - Quota status response.
 * @param {{editable?: boolean, onEdit?: function(): void}} [options={}] - Administrator controls.
 * @returns {HTMLElement|null} Quota panel.
 */
export function createTrafficQuotaPanel(status, {editable = false, onEdit = null} = {}) {
    if (!status) return null;
    const headerActions = editable && typeof onEdit === 'function'
        ? el('button', {
            type: 'button', class: 'pill-btn pill-btn--soft pill-btn--sm', onclick: onEdit
        }, createIcon('edit'), el('span', {}, t('common.edit') || 'Edit'))
        : null;

    const badge = status.unlimited
        ? el('span', {class: 'traffic-quota-badge is-unlimited'}, t('trafficQuota.unlimited') || 'Unlimited Traffic')
        : el('span', {class: 'traffic-quota-badge'}, status.subject_type === 'user' ? (t('trafficQuota.userQuota') || 'Account Quota') : (t('trafficQuota.ipQuota') || 'IP Quota'));

    let content;
    if (status.unlimited) {
        content = el('div', {class: 'traffic-quota-unlimited'},
            createIcon('check'),
            el('span', {}, t('trafficQuota.unlimitedHint') || 'No download traffic limits apply to this account.')
        );
    } else {
        const renderMetric = (label, tier) => {
            if (!tier || !tier.enabled) {
                return el('div', {class: 'traffic-quota-metric'},
                    el('span', {class: 'traffic-quota-metric-label'}, label),
                    el('strong', {style: 'color: var(--muted);'}, t('trafficQuota.disabled') || 'Disabled')
                );
            }
            const rem = Number(tier.remaining_mb || 0);
            const lim = Number(tier.limit_mb || 0);
            const ratio = lim > 0 ? Math.max(0, Math.min(1, rem / lim)) : 1;
            const percent = Math.round(ratio * 100);
            return el('div', {class: 'traffic-quota-metric'},
                el('span', {class: 'traffic-quota-metric-label'}, label),
                el('strong', {}, `${formatMB(rem)} / ${formatMB(lim)}`),
                el('span', {class: 'traffic-quota-track', 'aria-hidden': 'true', title: `${percent}% remaining`},
                    el('span', {style: {width: `${percent}%`}})
                )
            );
        };

        const grid = el('div', {class: 'traffic-quota-grid'},
            renderMetric(t('trafficQuota.hourly') || 'Hourly (1h)', status.hourly),
            renderMetric(t('trafficQuota.daily') || 'Daily (24h)', status.daily),
            renderMetric(t('trafficQuota.monthly') || 'Monthly (30d)', status.monthly)
        );

        let recoveryText = t('trafficQuota.recoveryDisabled') || 'Disabled (fixed window reset)';
        if (status.recovery_amount_mb > 0 && status.recovery_interval_s > 0) {
            recoveryText = `+${formatMB(status.recovery_amount_mb)} / ${formatInterval(status.recovery_interval_s)}`;
        }

        const footer = el('div', {class: 'traffic-quota-footer'},
            el('span', {},
                el('strong', {}, `${t('trafficQuota.recovery') || 'Rolling Recovery'}: `),
                recoveryText
            ),
            el('span', {},
                el('strong', {}, `${t('trafficQuota.multiThreadMultiplier') || 'Multi-thread Multiplier'}: `),
                `${status.multi_thread_multiplier || 1.25}x`
            )
        );

        content = [grid, footer];
    }

    return el('section', {class: 'traffic-quota-panel'},
        el('header', {class: 'traffic-quota-header'},
            el('div', {},
                el('h3', {}, createIcon('download'), t('trafficQuota.title') || 'Download Traffic Quota'),
                el('p', {}, t('trafficQuota.subtitle') || 'Download traffic limits, remaining window quota, and rolling recovery.')
            ),
            el('div', {class: 'traffic-quota-header-actions'}, badge, headerActions)
        ),
        content
    );
}

/**
 * Open the administrator traffic quota editor for one user.
 * @param {{username: string, onSaved?: function(object): void}} options
 * @returns {Promise<void>}
 */
export async function openTrafficQuotaDialog({username, onSaved = null}) {
    let status;
    try {
        const response = await apiRequest(`/api/traffic-quota/users/${encodeURIComponent(username)}`);
        if (!response.ok) throw await localizedResponseError(response, 'trafficQuota.loadFailed');
        status = await response.json();
    } catch (error) {
        showAlert(caughtErrorMessage(error, 'trafficQuota.loadFailed'), 'error');
        return;
    }

    let unlimited = Boolean(status.unlimited);
    let hourlyEnabled = Boolean(status.hourly?.enabled);
    let dailyEnabled = Boolean(status.daily?.enabled);
    let monthlyEnabled = Boolean(status.monthly?.enabled);

    const hourlyInput = el('input', {
        class: 'profile-input', type: 'number', min: '0', step: '0.001',
        value: String(status.hourly?.limit_mb ?? 5120)
    });
    const dailyInput = el('input', {
        class: 'profile-input', type: 'number', min: '0', step: '0.001',
        value: String(status.daily?.limit_mb ?? 35840)
    });
    const monthlyInput = el('input', {
        class: 'profile-input', type: 'number', min: '0', step: '0.001',
        value: String(status.monthly?.limit_mb ?? 614400)
    });
    const recoveryAmountInput = el('input', {
        class: 'profile-input', type: 'number', min: '0', step: '0.001',
        value: String(status.recovery_amount_mb ?? 2048)
    });
    const recoveryIntervalInput = el('input', {
        class: 'profile-input', type: 'number', min: '1', step: '1',
        value: String(status.recovery_interval_s ?? 3600)
    });

    const hourlyToggle = createToggle(hourlyEnabled, (val) => {
        hourlyEnabled = val;
        hourlyInput.disabled = !val || unlimited;
    });
    const dailyToggle = createToggle(dailyEnabled, (val) => {
        dailyEnabled = val;
        dailyInput.disabled = !val || unlimited;
    });
    const monthlyToggle = createToggle(monthlyEnabled, (val) => {
        monthlyEnabled = val;
        monthlyInput.disabled = !val || unlimited;
    });

    const unlimitedToggle = createToggle(unlimited, (val) => {
        unlimited = val;
        hourlyInput.disabled = val || !hourlyEnabled;
        dailyInput.disabled = val || !dailyEnabled;
        monthlyInput.disabled = val || !monthlyEnabled;
        recoveryAmountInput.disabled = val;
        recoveryIntervalInput.disabled = val;
        hourlyToggle.disabled = val;
        dailyToggle.disabled = val;
        monthlyToggle.disabled = val;
    });

    hourlyInput.disabled = unlimited || !hourlyEnabled;
    dailyInput.disabled = unlimited || !dailyEnabled;
    monthlyInput.disabled = unlimited || !monthlyEnabled;
    recoveryAmountInput.disabled = unlimited;
    recoveryIntervalInput.disabled = unlimited;
    hourlyToggle.disabled = unlimited;
    dailyToggle.disabled = unlimited;
    monthlyToggle.disabled = unlimited;

    const fields = el('div', {class: 'traffic-quota-form-fields'},
        el('label', {}, el('span', {}, t('trafficQuota.hourlyLimitMB') || 'Hourly Limit (MB)'), hourlyInput),
        el('label', {}, el('span', {}, t('trafficQuota.dailyLimitMB') || 'Daily Limit (MB)'), dailyInput),
        el('label', {}, el('span', {}, t('trafficQuota.monthlyLimitMB') || 'Monthly Limit (MB)'), monthlyInput),
        el('label', {}, el('span', {}, t('trafficQuota.recoveryAmountMB') || 'Recovery Amount (MB)'), recoveryAmountInput),
        el('label', {}, el('span', {}, t('trafficQuota.recoveryIntervalS') || 'Recovery Interval (s)'), recoveryIntervalInput)
    );

    const previewContainer = el('div', {}, createTrafficQuotaPanel(status));

    const body = el('div', {class: 'traffic-quota-form'},
        previewContainer,
        el('div', {class: 'traffic-quota-toggle-row'},
            el('span', {},
                el('strong', {}, t('trafficQuota.unlimited') || 'Unlimited Traffic'),
                el('small', {}, t('trafficQuota.unlimitedAdminHint') || 'Disable all download limits for this user.')
            ),
            unlimitedToggle
        ),
        el('div', {class: 'traffic-quota-toggle-row'},
            el('span', {}, el('strong', {}, t('trafficQuota.hourly') || 'Hourly Limit')),
            hourlyToggle
        ),
        el('div', {class: 'traffic-quota-toggle-row'},
            el('span', {}, el('strong', {}, t('trafficQuota.daily') || 'Daily Limit')),
            dailyToggle
        ),
        el('div', {class: 'traffic-quota-toggle-row'},
            el('span', {}, el('strong', {}, t('trafficQuota.monthly') || 'Monthly Limit')),
            monthlyToggle
        ),
        fields
    );

    void RenopDialog.show({
        id: 'traffic-quota-dialog',
        maxWidth: '640px',
        icon: 'download',
        title: t('trafficQuota.editTitle', {owner: username}) || `Traffic Quota — ${username}`,
        subtitle: t('trafficQuota.adminUserHint') || 'Configure download rate limiting for this user.',
        body,
        footer: [
            {id: 'traffic-quota-cancel', text: t('common.cancel') || 'Cancel', close: true},
            {
                id: 'traffic-quota-save',
                text: t('common.save') || 'Save',
                primary: true,
                handler: async () => {
                    const hVal = Number(hourlyInput.value);
                    const dVal = Number(dailyInput.value);
                    const mVal = Number(monthlyInput.value);
                    const recVal = Number(recoveryAmountInput.value);
                    const intVal = Number(recoveryIntervalInput.value);

                    if (!unlimited && (!Number.isFinite(hVal) || !Number.isFinite(dVal) || !Number.isFinite(mVal) ||
                        !Number.isFinite(recVal) || !Number.isFinite(intVal) || intVal <= 0)) {
                        showAlert(t('trafficQuota.invalid') || 'Please enter valid values.', 'error');
                        return false;
                    }

                    const payload = {
                        unlimited,
                        hourly_limit_mb: hVal,
                        daily_limit_mb: dVal,
                        monthly_limit_mb: mVal,
                        hourly_enabled: hourlyEnabled,
                        daily_enabled: dailyEnabled,
                        monthly_enabled: monthlyEnabled,
                        recovery_amount_mb: recVal,
                        recovery_interval_s: intVal
                    };

                    try {
                        const response = await apiRequest(`/api/traffic-quota/users/${encodeURIComponent(username)}`, {
                            method: 'PUT',
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify(payload)
                        });
                        if (!response.ok) throw await localizedResponseError(response, 'trafficQuota.saveFailed');
                        const updated = await response.json();
                        showAlert(t('trafficQuota.saved') || 'Traffic quota updated.', 'success');
                        if (typeof onSaved === 'function') onSaved(updated);
                        return true;
                    } catch (err) {
                        showAlert(caughtErrorMessage(err, 'trafficQuota.saveFailed'), 'error');
                        return false;
                    }
                }
            }
        ]
    });
}

/**
 * Refresh current user's traffic quota status in the profile view.
 * @returns {Promise<void>}
 */
export async function refreshProfileTrafficQuota() {
    const container = document.getElementById('profile-traffic-quota-body');
    if (!container) return;
    try {
        const response = await apiRequest('/api/traffic-quota');
        if (!response.ok) {
            container.replaceChildren(el('p', {class: 'profile-security-hint'}, t('profile.trafficQuotaLoadFailed') || 'Failed to load traffic quota.'));
            return;
        }
        const status = await response.json();
        const panel = createTrafficQuotaPanel(status);
        if (panel) {
            container.replaceChildren(panel);
        }
    } catch (e) {
        container.replaceChildren(el('p', {class: 'profile-security-hint'}, t('profile.trafficQuotaLoadFailed') || 'Failed to load traffic quota.'));
    }
}
