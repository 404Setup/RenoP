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
import {StorageCleanupResponse, StorageStatsResponse} from './proto/index.js';
import {formatBytes} from './browser/utils.js';
import {showAlert} from './alert.js';
import {t} from './i18n.js';
import {switchTab} from './main.js';

let isCleaning = false;

function resolveStorageCategory(category) {
    switch (category) {
        case 'repositories': return t('storage.repositories') || 'Package Repositories';
        case 'database': return t('storage.database') || 'Database';
        case 'updater': return t('storage.updaterResiduals') || 'Updater Residuals';
        case 'temp': return t('storage.tempFiles') || 'Temporary Uploads & Chunks';
        case 'system': return t('storage.systemOther') || 'System & Other';
        default: return category || '-';
    }
}

function resolveStorageItemName(name) {
    switch (name) {
        case 'RenoP Package Repositories': return t('storage.itemPackageRepositories') || t('storage.repositories') || name;
        case 'Database Persistence': return t('storage.itemDatabasePersistence') || t('storage.database') || name;
        case 'Updater Residuals & Binaries': return t('storage.itemUpdaterResiduals') || t('storage.updaterResiduals') || name;
        case 'Temporary Uploads & Chunks': return t('storage.itemTempUploads') || t('storage.tempFiles') || name;
        case 'System & Other Files': return t('storage.itemSystemOther') || t('storage.systemOther') || name;
        default: return name || '-';
    }
}

/** Load and render the storage management page data. */
export async function loadStoragePage() {
    const renopUsedEl = document.getElementById('storage-renop-used');
    const updaterEl = document.getElementById('storage-updater-residuals');
    const tempEl = document.getElementById('storage-temp-files');
    const diskSummaryEl = document.getElementById('storage-disk-summary');
    const tbody = document.getElementById('storage-items-tbody');
    const cleanBtn = document.getElementById('btn-clean-storage');
    const backBtn = document.getElementById('btn-storage-back');

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
            isCleaning = true;
            cleanBtn.disabled = true;
            const originalText = cleanBtn.querySelector('span')?.textContent || '';
            const span = cleanBtn.querySelector('span');
            if (span) span.textContent = t('storage.cleaning');

            try {
                const {response: resp, data: res} = await fetchProto('/api/status/storage/cleanup', StorageCleanupResponse, {
                    method: 'POST'
                });
                if (!resp?.ok) {
                    throw new Error(`HTTP ${resp?.status}`);
                }
                const count = Number(res?.cleaned_files || 0);
                const reclaimed = Number(res?.reclaimed_bytes || 0);
                showAlert(t('storage.cleanSuccess', {
                    count,
                    reclaimed: formatBytes(reclaimed)
                }), 'success');
                await loadStoragePage();
            } catch (err) {
                showAlert(t('storage.cleanFailed', {error: err.message || err}), 'error');
            } finally {
                isCleaning = false;
                cleanBtn.disabled = false;
                if (span) span.textContent = t('storage.cleanNow');
            }
        };
    }

    try {
        const {response: resp, data} = await fetchProto('/api/status/storage', StorageStatsResponse);
        if (!resp?.ok || !data) {
            showAlert(t('storage.cleanFailed', {error: `HTTP ${resp?.status}`}), 'error');
            return;
        }

        if (renopUsedEl) renopUsedEl.textContent = formatBytes(Number(data.renop_used_bytes || 0));
        if (updaterEl) {
            const bytes = formatBytes(Number(data.updater_residuals_bytes || 0));
            updaterEl.textContent = data.updater_residuals_count ? `${bytes} (${data.updater_residuals_count})` : bytes;
        }
        if (tempEl) {
            const bytes = formatBytes(Number(data.temp_files_bytes || 0));
            tempEl.textContent = data.temp_files_count ? `${bytes} (${data.temp_files_count})` : bytes;
        }
        if (diskSummaryEl) {
            diskSummaryEl.textContent = t('storage.diskSummary', {
                free: formatBytes(Number(data.disk_free_bytes || 0)),
                total: formatBytes(Number(data.disk_total_bytes || 0))
            });
        }

        if (tbody) {
            tbody.innerHTML = '';
            const items = data.items || [];
            if (items.length === 0) {
                const tr = el('tr', {},
                    el('td', {colspan: '4', style: 'text-align: center; padding: 1.5rem; opacity: 0.6;'}, t('common.none'))
                );
                tbody.appendChild(tr);
            } else {
                items.forEach(item => {
                    const categoryLabel = resolveStorageCategory(item.category);
                    const nameLabel = resolveStorageItemName(item.name);
                    const tr = el('tr', {style: 'border-bottom: 1px solid var(--border-color);'},
                        el('td', {style: 'padding: 0.75rem 1rem; font-weight: 500;'}, categoryLabel),
                        el('td', {style: 'padding: 0.75rem 1rem;'}, nameLabel),
                        el('td', {style: 'padding: 0.75rem 1rem; text-align: right; opacity: 0.7;'}, item.count !== undefined ? String(item.count) : '-'),
                        el('td', {style: 'padding: 0.75rem 1rem; text-align: right; font-weight: 600;'}, formatBytes(Number(item.size_bytes || 0)))
                    );
                    tbody.appendChild(tr);
                });
            }
        }
    } catch (e) {
        console.error('Failed to load storage page data:', e);
    }
}
