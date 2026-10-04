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
import {fetchProto} from './api.js';
import {DatabaseCleanupResponse, DatabaseStatsResponse} from './proto/index.js';
import {formatBytes} from './browser/utils.js';
import {showAlert, showConfirm} from './alert.js';
import {t} from './i18n.js';
import {switchTab} from './main.js';

let isCleaning = false;

/** Load and render the database management page data. */
export async function loadDatabasePage() {
    const driverEl = document.getElementById('database-driver');
    const totalSizeEl = document.getElementById('database-total-size');
    const reclaimableRowsEl = document.getElementById('database-reclaimable-rows');
    const reclaimableBytesEl = document.getElementById('database-reclaimable-bytes');
    const tbody = document.getElementById('database-tables-tbody');
    const cleanBtn = document.getElementById('btn-clean-database');
    const backBtn = document.getElementById('btn-database-back');

    if (backBtn && !backBtn.dataset.wired) {
        backBtn.dataset.wired = 'true';
        backBtn.onclick = (e) => {
            e.preventDefault();
            window.history.pushState(null, '', '/dashboard');
            void switchTab('dashboard');
        };
    }

    if (cleanBtn && !cleanBtn.dataset.wired) {
        cleanBtn.dataset.wired = 'true';
        cleanBtn.onclick = async () => {
            if (isCleaning) return;
            const confirmed = await showConfirm(t('database.cleanConfirm'), {
                title: t('database.cleanNow'),
                confirmText: t('database.cleanNow'),
                danger: false
            });
            if (!confirmed) return;

            isCleaning = true;
            cleanBtn.disabled = true;
            const span = cleanBtn.querySelector('span');
            if (span) span.textContent = t('database.cleaning');

            try {
                const {response: resp, data: res} = await fetchProto('/api/status/database/cleanup', DatabaseCleanupResponse, {
                    method: 'POST'
                });
                if (!resp?.ok) {
                    throw new Error(`HTTP ${resp?.status}`);
                }
                const rows = Number(res?.cleaned_rows || 0);
                const reclaimed = Number(res?.reclaimed_bytes || 0);
                showAlert(t('database.cleanSuccess', {
                    rows,
                    reclaimed: formatBytes(reclaimed)
                }), 'success');
                await loadDatabasePage();
            } catch (err) {
                showAlert(t('database.cleanFailed', {error: err.message || err}), 'error');
            } finally {
                isCleaning = false;
                cleanBtn.disabled = false;
                if (span) span.textContent = t('database.cleanNow');
            }
        };
    }

    try {
        const {response: resp, data} = await fetchProto('/api/status/database', DatabaseStatsResponse);
        if (!resp?.ok || !data) {
            showAlert(t('database.cleanFailed', {error: `HTTP ${resp?.status}`}), 'error');
            return;
        }

        if (driverEl) driverEl.textContent = (data.driver || 'SQLite').toUpperCase();
        if (totalSizeEl) totalSizeEl.textContent = formatBytes(Number(data.total_size_bytes || 0));
        if (reclaimableRowsEl) reclaimableRowsEl.textContent = String(data.reclaimable_rows || 0);
        if (reclaimableBytesEl) reclaimableBytesEl.textContent = formatBytes(Number(data.reclaimable_bytes || 0));

        if (tbody) {
            tbody.innerHTML = '';
            const tables = data.tables || [];
            if (tables.length === 0) {
                const tr = el('tr', {},
                    el('td', {colspan: '4', style: 'text-align: center; padding: 1.5rem; opacity: 0.6;'}, t('common.none'))
                );
                tbody.appendChild(tr);
            } else {
                tables.forEach(table => {
                    const statusBadge = table.reclaimable
                        ? el('span', {class: 'badge badge-warning', style: 'font-size: 0.75rem; padding: 0.15rem 0.5rem;'}, t('database.reclaimable'))
                        : el('span', {style: 'opacity: 0.5; font-size: 0.8rem;'}, '-');

                    const tr = el('tr', {style: 'border-bottom: 1px solid var(--border-color);'},
                        el('td', {style: 'padding: 0.75rem 1rem; font-family: monospace; font-size: 0.9rem;'}, table.name || '-'),
                        el('td', {style: 'padding: 0.75rem 1rem; text-align: right;'}, String(table.row_count || 0)),
                        el('td', {style: 'padding: 0.75rem 1rem; text-align: right; font-weight: 600;'}, formatBytes(Number(table.size_bytes || 0))),
                        el('td', {style: 'padding: 0.75rem 1rem; text-align: center;'}, statusBadge)
                    );
                    tbody.appendChild(tr);
                });
            }
        }
    } catch (e) {
        console.error('Failed to load database page data:', e);
    }
}
