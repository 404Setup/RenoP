/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {apiRequest, fetchProto, getAuthHeaders} from './api.js';
import {el, svg, retriggerClass} from '@renop/ui/dom';
import {formatBytes} from './browser/utils.js';
import {t} from './i18n.js';
import {openAuditLogsDialog} from './audit.js';
import {logout} from './auth.js';
import {showAlert} from './alert.js';
import {CachePurgeResponse, InstanceStatus, StatusSnapshotList, UpdateState} from './proto/index.js';
import {formatTimestamp} from './time.js';
import {refreshMessageUnreadCount} from './messages.js';
import {updaterErrorMessage} from './updater-errors.js';
import {responseErrorMessage} from './response-errors.js';
import {exitProtectedRouteOnDenial} from './protected-route.js';
import {switchTab} from './main.js';

let refreshInterval = null;
let snapshotsInterval = null;

let _uptimeBase = null;
let _uptimeReceivedAt = null;
let _uptimeTicker = null;

/**
 * Starts a 1s interval that advances the dashboard uptime display from the last calibrated base.
 * @returns {void}
 */
function _startUptimeTicker() {
    if (_uptimeTicker) return;
    _uptimeTicker = setInterval(() => {
        const el = document.getElementById('dashboard-uptime');
        if (!el) return;
        if (_uptimeBase === null || _uptimeReceivedAt === null) return;
        const elapsed = (Date.now() - _uptimeReceivedAt) / 1000;
        el.textContent = prettyUptime(_uptimeBase + elapsed);
    }, 1000);
}

/**
 * Stops the local uptime ticker interval if running.
 * @returns {void}
 */
function _stopUptimeTicker() {
    if (_uptimeTicker) {
        clearInterval(_uptimeTicker);
        _uptimeTicker = null;
    }
}

/**
 * Called whenever we receive a fresh uptime value from the API.
 * Calibrates the local base and smoothly continues from there.
 * @param {number} apiSeconds - Uptime in seconds from the instance status API.
 * @returns {void}
 */
function _calibrateUptime(apiSeconds) {
    _uptimeBase = apiSeconds;
    _uptimeReceivedAt = Date.now();
    const el = document.getElementById('dashboard-uptime');
    if (el) el.textContent = prettyUptime(apiSeconds);
}

/**
 * Shows skeleton placeholders on empty dashboard stat elements while data loads.
 * @returns {void}
 */
function renderDashboardSkeleton() {
    ['dashboard-version', 'dashboard-uptime', 'dashboard-memory', 'dashboard-disk', 'dashboard-database', 'dashboard-cache', 'dashboard-threads', 'dashboard-cores', 'dashboard-os-arch', 'dashboard-failures'].forEach(id => {
        const el = document.getElementById(id);
        if (el && (!el.textContent || el.textContent.trim() === '' || el.textContent.trim() === '-')) {
            el.classList.add('skeleton-bone');
        }
    });
}

/**
 * Removes skeleton placeholders and plays a brief enter animation on previously skeleton elements.
 * @returns {void}
 */
function clearDashboardSkeleton() {
    ['dashboard-version', 'dashboard-uptime', 'dashboard-memory', 'dashboard-disk', 'dashboard-database', 'dashboard-cache', 'dashboard-threads', 'dashboard-cores', 'dashboard-os-arch', 'dashboard-failures'].forEach(id => {
        const el = document.getElementById(id);
        if (el) {
            const wasSkeleton = el.classList.contains('skeleton-bone');
            el.classList.remove('skeleton-bone');
            if (wasSkeleton) {
                retriggerClass(el, 'is-content-entering');
            }
        }
    });
}

let longPressTimer = null;
let isLongPressTriggered = false;

/**
 * Binds long-press (1.5s) on the update button to open the offline update modal.
 * Suppresses the subsequent click when a long-press fires. Idempotent per button.
 * @param {HTMLElement|null} updateBtn - The dashboard update button element.
 * @returns {void}
 */
function setupUpdateBtnLongPress(updateBtn) {
    if (!updateBtn || updateBtn.dataset.longPressBound === 'true') return;
    updateBtn.dataset.longPressBound = 'true';

    const startPress = () => {
        if (updateBtn.disabled) return;
        isLongPressTriggered = false;
        if (currentUpdateData && (currentUpdateData.status === 'ready_to_restart' || currentUpdateData.status === 'downloading')) {
            return;
        }
        updateBtn.classList.add('is-pressing');
        longPressTimer = setTimeout(() => {
            isLongPressTriggered = true;
            updateBtn.classList.remove('is-pressing');
            if (window.showOfflineUpdateModal) {
                window.showOfflineUpdateModal();
            }
            setTimeout(() => {
                isLongPressTriggered = false;
            }, 500);
        }, 1500);
    };

    const cancelPress = () => {
        if (longPressTimer) {
            clearTimeout(longPressTimer);
            longPressTimer = null;
        }
        updateBtn.classList.remove('is-pressing');
    };

    updateBtn.addEventListener('mousedown', startPress);
    updateBtn.addEventListener('touchstart', startPress, {passive: true});

    updateBtn.addEventListener('mouseup', cancelPress);
    updateBtn.addEventListener('mouseleave', cancelPress);
    updateBtn.addEventListener('touchend', cancelPress);
    updateBtn.addEventListener('touchcancel', cancelPress);

    updateBtn.addEventListener('click', (e) => {
        if (isLongPressTriggered) {
            e.preventDefault();
            e.stopPropagation();
            e.stopImmediatePropagation();
            isLongPressTriggered = false;
            return false;
        }
    }, true);
}

/**
 * Shows or hides debug memory dump tools (requires server.debug_mode + restart).
 * @param {boolean} active
 * @returns {void}
 */
function updateDebugMemoryTools(active) {
    const panel = document.getElementById('dashboard-debug-tools');
    if (!panel) return;
    panel.hidden = !active;
    if (!active) return;
    _wireDebugDumpButton('btn-dump-heap', 'heap');
    _wireDebugDumpButton('btn-dump-allocs', 'allocs');
    _wireDebugDumpButton('btn-dump-goroutine', 'goroutine');
}

/**
 * @param {string} buttonId
 * @param {'heap'|'allocs'|'goroutine'} kind
 */
function _wireDebugDumpButton(buttonId, kind) {
    const btn = document.getElementById(buttonId);
    if (!btn || btn.dataset.wired === '1') return;
    btn.dataset.wired = '1';
    btn.addEventListener('click', () => {
        void downloadMemoryProfile(kind, btn);
    });
}

/**
 * Downloads a pprof profile for flame-graph analysis (Speedscope / go tool pprof).
 * @param {'heap'|'allocs'|'goroutine'} kind
 * @param {HTMLButtonElement|null} [btn]
 * @returns {Promise<void>}
 */
async function downloadMemoryProfile(kind, btn = null) {
    const path = kind === 'allocs'
        ? '/api/debug/memory/allocs'
        : kind === 'goroutine'
            ? '/api/debug/memory/goroutine'
            : '/api/debug/memory/heap?gc=1';
    const filename = kind === 'allocs'
        ? 'renop-allocs.pprof'
        : kind === 'goroutine'
            ? 'renop-goroutine.pprof'
            : 'renop-heap.pprof';

    const prevText = btn ? btn.textContent : '';
    if (btn) {
        btn.disabled = true;
        btn.textContent = t('dashboard.dumpWorking');
    }
    try {
        const response = await apiRequest(path, {method: 'GET'});
        if (!response.ok) {
            showAlert(await responseErrorMessage(response, 'dashboard.dumpFailed'), 'error');
            return;
        }
        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        const a = el('a', {
            href: url,
            download: filename
        });
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
        showAlert(t('dashboard.dumpReadyBody', {file: filename}), 'success');
    } catch (e) {
        if (e && e.message === 'Unauthorized') return;
        console.error('memory profile dump failed', e);
        showAlert(t('dashboard.dumpFailed'), 'error');
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = prevText || t(
                kind === 'allocs' ? 'dashboard.dumpAllocs'
                    : kind === 'goroutine' ? 'dashboard.dumpGoroutine'
                        : 'dashboard.dumpHeap',
            );
        }
    }
}

/**
 * Fetches instance status and updates dashboard stats (version, uptime, memory, disk, etc.).
 * Also wires the update button and applies updater UI when an update state is present.
 * @returns {Promise<void>}
 */
export async function fetchInstanceStatus() {
    renderDashboardSkeleton();
    try {
        const {response, data} = await fetchProto('/api/status/instance', InstanceStatus);
        if (response.ok && data) {
            clearDashboardSkeleton();

            let displayVersion = data.version || '';
            if (data.development && /^[0-9a-fA-F]{7,}$/.test(displayVersion)) {
                displayVersion = displayVersion.substring(0, 7);
            } else if (/^[0-9a-fA-F]{40}$/.test(displayVersion)) {
                displayVersion = displayVersion.substring(0, 7);
            }
            document.getElementById('dashboard-version').textContent = displayVersion;
            _calibrateUptime(data.uptime / 1000);
            const memEl = document.getElementById('dashboard-memory');
            const rssMem = data.used_memory !== undefined ? data.used_memory : data.usedMemory;
            const vssMem = data.vss_memory !== undefined ? data.vss_memory : data.vssMemory;
            const totalMem = data.total_memory !== undefined ? data.total_memory : data.totalMemory;
            if (memEl && rssMem !== undefined && totalMem !== undefined && rssMem !== null && totalMem !== null) {
                // API reports memory fields in bytes (not MiB).
                const rssBytes = Number(rssMem || 0);
                const vssBytes = Number(vssMem || 0);
                const totalBytes = Number(totalMem || 0);
                memEl.dataset.rssMemory = String(rssBytes);
                memEl.dataset.vssMemory = String(vssBytes);
                memEl.dataset.totalMemory = String(totalBytes);
                // Backward-compat keys for languageChanged handlers / older markup.
                memEl.dataset.usedMemory = String(rssBytes);
                if (rssBytes === vssBytes || vssBytes === 0) {
                    const procBytes = rssBytes || vssBytes;
                    memEl.textContent = t('dashboard.memoryFormatSimple', {
                        used: formatBytes(procBytes),
                        process: formatBytes(procBytes),
                        total: formatBytes(totalBytes)
                    });
                } else {
                    memEl.textContent = t('dashboard.memoryFormat', {
                        rss: formatBytes(rssBytes),
                        vss: formatBytes(vssBytes),
                        total: formatBytes(totalBytes)
                    });
                }
            }
            const diskEl = document.getElementById('dashboard-disk');
            const renopUsedDisk = data.renop_used_disk !== undefined ? data.renop_used_disk : data.renopUsedDisk;
            const diskUsed = data.disk_used !== undefined ? data.disk_used : data.diskUsed;
            const diskTotal = data.disk_total !== undefined ? data.disk_total : data.diskTotal;
            if (diskEl && renopUsedDisk !== undefined && diskUsed !== undefined && diskTotal !== undefined) {
                diskEl.dataset.renopUsed = String(renopUsedDisk);
                diskEl.dataset.diskUsed = String(diskUsed);
                diskEl.dataset.diskTotal = String(diskTotal);
                diskEl.textContent = t('dashboard.diskFormat', {
                    renopUsed: formatBytes(renopUsedDisk),
                    diskUsed: formatBytes(diskUsed),
                    diskTotal: formatBytes(diskTotal)
                });
            }

            const dbEl = document.getElementById('dashboard-database');
            const dbUsed = data.database_used_bytes !== undefined ? data.database_used_bytes : data.databaseUsedBytes;
            if (dbEl) {
                if (dbUsed !== undefined && dbUsed !== null) {
                    dbEl.dataset.dbUsed = String(dbUsed);
                    dbEl.textContent = formatBytes(Number(dbUsed));
                } else {
                    dbEl.textContent = '-';
                }
            }

            const cacheEl = document.getElementById('dashboard-cache');
            const cacheUsed = data.cache_used_bytes !== undefined ? data.cache_used_bytes : data.cacheUsedBytes;
            if (cacheEl) {
                if (cacheUsed !== undefined && cacheUsed !== null) {
                    cacheEl.dataset.cacheUsed = String(cacheUsed);
                    cacheEl.textContent = formatBytes(Number(cacheUsed));
                } else {
                    cacheEl.textContent = '-';
                }
            }

            const btnStorage = document.getElementById('btn-goto-storage');
            if (btnStorage && !btnStorage.dataset.wired) {
                btnStorage.dataset.wired = 'true';
                btnStorage.onclick = () => {
                    window.history.pushState(null, '', '/storage');
                    void switchTab('storage');
                };
            }

            const btnDb = document.getElementById('btn-goto-database');
            if (btnDb && !btnDb.dataset.wired) {
                btnDb.dataset.wired = 'true';
                btnDb.onclick = () => {
                    window.history.pushState(null, '', '/database');
                    void switchTab('database');
                };
            }

            const btnCache = document.getElementById('btn-purge-cache');
            if (btnCache && !btnCache.dataset.wired) {
                btnCache.dataset.wired = 'true';
                btnCache.onclick = async () => {
                    const confirmed = await (window.showConfirm ? window.showConfirm(t('dashboard.purgeCache')) : Promise.resolve(confirm(t('dashboard.purgeCache'))));
                    if (!confirmed) return;
                    btnCache.disabled = true;
                    try {
                        const {response: res, data} = await fetchProto('/api/status/cache/cleanup', CachePurgeResponse, {method: 'POST'});
                        if (res?.ok) {
                            const reclaimed = Number(data?.reclaimed_bytes || 0);
                            showAlert(t('dashboard.cachePurged', {reclaimed: formatBytes(reclaimed)}), 'success');
                            void fetchInstanceStatus();
                        } else {
                            showAlert(await responseErrorMessage(res, 'dashboard.cachePurgeFailed'), 'error');
                        }
                    } catch (e) {
                        showAlert(t('dashboard.cachePurgeFailed', {error: e.message || e}), 'error');
                    } finally {
                        btnCache.disabled = false;
                    }
                };
            }

            document.getElementById('dashboard-threads').textContent = `${data.used_threads}`;

            document.getElementById('dashboard-cores').textContent = `${data.logical_cores} / ${data.physical_cores}`;
            const archDisplay = data.architecture || '';
            document.getElementById('dashboard-os-arch').textContent = `${data.os} / ${archDisplay}`;
            document.getElementById('dashboard-failures').textContent = data.failures_count !== undefined ? data.failures_count : '0';

            const updateBtn = document.getElementById('btn-dashboard-update');
            if (updateBtn) {
                setupUpdateBtnLongPress(updateBtn);
                if (!updateBtn.onclick) {
                    updateBtn.onclick = () => checkAppUpdate(true);
                }
            }

            if (data.update_state) {
                if (data.update_state.status !== 'idle' || currentUpdateData) {
                    currentUpdateData = data.update_state.status === 'idle' ? null : data.update_state;
                    updateDashboardVersionUI(data.update_state);
                }
            }

            updateDebugMemoryTools(!!data.debug_mode);
        } else if (exitProtectedRouteOnDenial(response)) {
            if (response.status === 401) void logout('kicked');
        }
    } catch (e) {
        console.error('Failed to fetch instance status', e);
    }
}

/**
 * Formats a duration in seconds as a compact human-readable uptime string (e.g. "1d 2h 3min 4s").
 * @param {number} seconds - Total uptime in seconds.
 * @returns {string} Formatted uptime string.
 */
function prettyUptime(seconds) {
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor(((seconds % 86400) % 3600) / 60);
    const s = Math.floor(((seconds % 86400) % 3600) % 60);

    const dDisplay = d > 0 ? d + "d " : "";
    const hDisplay = h > 0 ? h + "h " : "";
    const mDisplay = m > 0 ? m + "min " : "";
    const sDisplay = s >= 0 ? s + "s" : "";

    return dDisplay + hDisplay + mDisplay + sDisplay;
}

/**
 * Fetches memory status snapshots and redraws the snapshots chart.
 * @returns {Promise<void>}
 */
async function fetchSnapshots() {
    try {
        const {response, data} = await fetchProto('/api/status/snapshots', StatusSnapshotList);
        if (response.ok && data) {
            drawSnapshotsChart(data.snapshots || []);
        }
    } catch (e) {
        console.error('Failed to fetch status snapshots', e);
    }
}

/**
 * Reads RSS (used_memory) and VSS from a snapshot point, in bytes.
 * @param {object} point - Status snapshot.
 * @returns {{rss: number, vss: number}}
 */
function _snapshotMemoryBytes(point) {
    const rss = Number(point.used_memory !== undefined ? point.used_memory : (point.usedMemory || 0)) || 0;
    let vss = Number(point.vss_memory !== undefined ? point.vss_memory : (point.vssMemory || 0)) || 0;
    if (vss < rss) vss = rss;
    return {rss, vss};
}

let _lastSnapshotsData = null;
let _snapshotsResizeObserver = null;

/**
 * Renders or updates the memory usage line chart from status snapshot points.
 * When RSS equals VSS across the dataset, only total memory is displayed (single line & legend).
 * Otherwise, both RSS and VSS lines are rendered.
 * @param {Array<object>} data - Snapshot points with used_memory/vss_memory and timestamp.
 * @returns {void}
 */
function drawSnapshotsChart(data) {
    const container = document.getElementById('snapshots-chart-container');
    const chart = document.getElementById('snapshots-chart');
    if (!container || !chart) return;

    if (!Array.isArray(data) || data.length === 0) {
        container.style.display = 'none';
        chart.innerHTML = '';
        _lastSnapshotsData = null;
        return;
    }

    _lastSnapshotsData = data;
    container.style.display = '';

    const card = chart.parentElement || chart;
    const legend = document.getElementById('dashboard-chart-legend');

    const points = data.map((d) => {
        const {rss, vss} = _snapshotMemoryBytes(d);
        return {
            rss,
            vss,
            total: Math.max(rss, vss),
            timestamp: d.timestamp
        };
    });

    const isSingleLine = points.every(p => p.rss === p.vss || p.vss === 0);

    const peakMemory = Math.max(
        ...points.map(p => Math.max(p.rss, p.vss)),
        10 * 1024 * 1024
    );
    const maxMemory = Math.ceil(peakMemory * 1.1);

    _appendYLabels(card, maxMemory);

    if (legend) {
        legend.innerHTML = '';
        if (isSingleLine) {
            legend.appendChild(el('div', {class: 'dashboard-chart-legend-item'},
                el('span', {class: 'dashboard-chart-legend-swatch dashboard-chart-legend-swatch--total'}),
                el('span', {}, t('dashboard.chartLegendTotal'))
            ));
        } else {
            legend.appendChild(el('div', {class: 'dashboard-chart-legend-item'},
                el('span', {class: 'dashboard-chart-legend-swatch dashboard-chart-legend-swatch--rss'}),
                el('span', {}, t('dashboard.chartLegendRss'))
            ));
            legend.appendChild(el('div', {class: 'dashboard-chart-legend-item'},
                el('span', {class: 'dashboard-chart-legend-swatch dashboard-chart-legend-swatch--vss'}),
                el('span', {}, t('dashboard.chartLegendVss'))
            ));
        }
    }

    const width = Math.max(chart.clientWidth || 800, 320);
    const height = 220;
    const padTop = 16;
    const padBottom = 20;
    const padLeft = 8;
    const padRight = 8;
    const plotWidth = width - padLeft - padRight;
    const plotHeight = height - padTop - padBottom;
    const n = points.length;

    const getX = (idx) => n > 1 ? padLeft + (idx / (n - 1)) * plotWidth : padLeft + plotWidth / 2;
    const getY = (val) => padTop + plotHeight - (Math.min(val, maxMemory) / maxMemory) * plotHeight;

    function buildLineD(valGetter) {
        return points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${getX(i).toFixed(1)} ${getY(valGetter(p)).toFixed(1)}`).join(' ');
    }

    function buildAreaD(valGetter) {
        const lineD = buildLineD(valGetter);
        const firstX = getX(0).toFixed(1);
        const lastX = getX(n - 1).toFixed(1);
        const baseY = (padTop + plotHeight).toFixed(1);
        return `${lineD} L ${lastX} ${baseY} L ${firstX} ${baseY} Z`;
    }

    const svgEl = svg('svg', {
        class: 'snapshots-line-chart',
        viewBox: `0 0 ${width} ${height}`,
        preserveAspectRatio: 'none',
        style: 'width: 100%; height: 100%; display: block;'
    });

    const defs = svg('defs', {},
        svg('linearGradient', {id: 'area-grad-total', x1: '0', y1: '0', x2: '0', y2: '1'},
            svg('stop', {offset: '0%', 'stop-color': '#3b82f6', 'stop-opacity': '0.35'}),
            svg('stop', {offset: '100%', 'stop-color': '#3b82f6', 'stop-opacity': '0.02'})
        ),
        svg('linearGradient', {id: 'area-grad-rss', x1: '0', y1: '0', x2: '0', y2: '1'},
            svg('stop', {offset: '0%', 'stop-color': '#3b82f6', 'stop-opacity': '0.35'}),
            svg('stop', {offset: '100%', 'stop-color': '#3b82f6', 'stop-opacity': '0.02'})
        ),
        svg('linearGradient', {id: 'area-grad-vss', x1: '0', y1: '0', x2: '0', y2: '1'},
            svg('stop', {offset: '0%', 'stop-color': '#8b5cf6', 'stop-opacity': '0.22'}),
            svg('stop', {offset: '100%', 'stop-color': '#8b5cf6', 'stop-opacity': '0.01'})
        )
    );
    svgEl.appendChild(defs);

    [0.25, 0.5, 0.75, 1.0].forEach(ratio => {
        const gridY = padTop + plotHeight - ratio * plotHeight;
        svgEl.appendChild(svg('line', {
            x1: String(padLeft),
            y1: gridY.toFixed(1),
            x2: String(padLeft + plotWidth),
            y2: gridY.toFixed(1),
            class: 'chart-grid-line'
        }));
    });

    if (isSingleLine) {
        svgEl.appendChild(svg('path', {
            d: buildAreaD(p => p.total),
            fill: 'url(#area-grad-total)',
            class: 'chart-area chart-area--total'
        }));
        svgEl.appendChild(svg('path', {
            d: buildLineD(p => p.total),
            class: 'chart-line chart-line--total'
        }));
    } else {
        svgEl.appendChild(svg('path', {
            d: buildAreaD(p => p.vss),
            fill: 'url(#area-grad-vss)',
            class: 'chart-area chart-area--vss'
        }));
        svgEl.appendChild(svg('path', {
            d: buildLineD(p => p.vss),
            class: 'chart-line chart-line--vss'
        }));
        svgEl.appendChild(svg('path', {
            d: buildAreaD(p => p.rss),
            fill: 'url(#area-grad-rss)',
            class: 'chart-area chart-area--rss'
        }));
        svgEl.appendChild(svg('path', {
            d: buildLineD(p => p.rss),
            class: 'chart-line chart-line--rss'
        }));
    }

    points.forEach((p, idx) => {
        const cx = getX(idx).toFixed(1);
        if (isSingleLine) {
            svgEl.appendChild(svg('circle', {
                cx,
                cy: getY(p.total).toFixed(1),
                r: '3.5',
                class: 'chart-point chart-point--total'
            }));
        } else {
            svgEl.appendChild(svg('circle', {
                cx,
                cy: getY(p.vss).toFixed(1),
                r: '3',
                class: 'chart-point chart-point--vss'
            }));
            svgEl.appendChild(svg('circle', {
                cx,
                cy: getY(p.rss).toFixed(1),
                r: '3.5',
                class: 'chart-point chart-point--rss'
            }));
        }
    });

    const cursorLine = svg('line', {
        class: 'chart-cursor-line',
        x1: '0',
        y1: String(padTop),
        x2: '0',
        y2: (padTop + plotHeight).toFixed(1),
        style: 'display: none;'
    });
    svgEl.appendChild(cursorLine);

    let activeDotTotal = null;
    let activeDotRss = null;
    let activeDotVss = null;
    if (isSingleLine) {
        activeDotTotal = svg('circle', {class: 'chart-point-active chart-point-active--total', r: '5', style: 'display: none;'});
        svgEl.appendChild(activeDotTotal);
    } else {
        activeDotVss = svg('circle', {class: 'chart-point-active chart-point-active--vss', r: '5', style: 'display: none;'});
        activeDotRss = svg('circle', {class: 'chart-point-active chart-point-active--rss', r: '5', style: 'display: none;'});
        svgEl.appendChild(activeDotVss);
        svgEl.appendChild(activeDotRss);
    }

    chart.innerHTML = '';
    chart.appendChild(svgEl);

    let tooltip = card.querySelector('.chart-tooltip');
    if (!tooltip) {
        tooltip = el('div', {class: 'chart-tooltip', style: 'display: none;'});
        card.appendChild(tooltip);
    }

    const handlePointer = (e) => {
        const rect = svgEl.getBoundingClientRect();
        if (rect.width <= 0) return;
        const mouseX = e.clientX - rect.left;
        const scaleX = width / rect.width;
        const svgMouseX = mouseX * scaleX;
        const step = n > 1 ? plotWidth / (n - 1) : plotWidth;
        const idx = Math.max(0, Math.min(n - 1, Math.round((svgMouseX - padLeft) / step)));
        const pt = points[idx];
        const ptSvgX = getX(idx);

        cursorLine.setAttribute('x1', ptSvgX.toFixed(1));
        cursorLine.setAttribute('x2', ptSvgX.toFixed(1));
        cursorLine.style.display = '';

        if (isSingleLine && activeDotTotal) {
            activeDotTotal.setAttribute('cx', ptSvgX.toFixed(1));
            activeDotTotal.setAttribute('cy', getY(pt.total).toFixed(1));
            activeDotTotal.style.display = '';
        } else if (!isSingleLine && activeDotRss && activeDotVss) {
            activeDotRss.setAttribute('cx', ptSvgX.toFixed(1));
            activeDotRss.setAttribute('cy', getY(pt.rss).toFixed(1));
            activeDotRss.style.display = '';
            activeDotVss.setAttribute('cx', ptSvgX.toFixed(1));
            activeDotVss.setAttribute('cy', getY(pt.vss).toFixed(1));
            activeDotVss.style.display = '';
        }

        const timeStr = formatTimestamp(pt.timestamp, {timeOnly: true, fallback: t('common.unknown')});
        if (isSingleLine) {
            tooltip.innerHTML = `
                <div class="chart-tooltip-time">${timeStr}</div>
                <div class="chart-tooltip-row">
                    <span class="chart-tooltip-label" style="color: #3b82f6;">${t('dashboard.chartLegendTotal')}</span>
                    <span class="chart-tooltip-val">${formatBytes(pt.total)}</span>
                </div>
            `;
        } else {
            tooltip.innerHTML = `
                <div class="chart-tooltip-time">${timeStr}</div>
                <div class="chart-tooltip-row">
                    <span class="chart-tooltip-label" style="color: #3b82f6;">RSS</span>
                    <span class="chart-tooltip-val">${formatBytes(pt.rss)}</span>
                </div>
                <div class="chart-tooltip-row">
                    <span class="chart-tooltip-label" style="color: #8b5cf6;">VSS</span>
                    <span class="chart-tooltip-val">${formatBytes(pt.vss)}</span>
                </div>
                <div class="chart-tooltip-row">
                    <span class="chart-tooltip-label" style="opacity: 0.7;">Total</span>
                    <span class="chart-tooltip-val">${formatBytes(pt.total)}</span>
                </div>
            `;
        }
        tooltip.style.display = 'block';

        const pixelX = chart.offsetLeft + (ptSvgX / width) * rect.width;
        const highestVal = isSingleLine ? pt.total : Math.max(pt.rss, pt.vss);
        const pixelY = chart.offsetTop + (getY(highestVal) / height) * rect.height;
        tooltip.style.left = `${pixelX}px`;
        tooltip.style.top = `${pixelY}px`;
    };

    const handlePointerLeave = () => {
        cursorLine.style.display = 'none';
        if (activeDotTotal) activeDotTotal.style.display = 'none';
        if (activeDotRss) activeDotRss.style.display = 'none';
        if (activeDotVss) activeDotVss.style.display = 'none';
        tooltip.style.display = 'none';
    };

    svgEl.addEventListener('pointermove', handlePointer);
    svgEl.addEventListener('pointerleave', handlePointerLeave);

    if (!_snapshotsResizeObserver && window.ResizeObserver) {
        _snapshotsResizeObserver = new ResizeObserver(() => {
            if (_lastSnapshotsData) {
                drawSnapshotsChart(_lastSnapshotsData);
            }
        });
        _snapshotsResizeObserver.observe(chart);
    }
}

/**
 * Appends top/mid/bottom Y-axis labels for the snapshots chart based on max memory (bytes).
 * @param {HTMLElement} card - Chart card element that hosts the Y labels.
 * @param {number} maxMemoryBytes - Maximum used memory in bytes across snapshots.
 * @returns {void}
 */
function _appendYLabels(card, maxMemoryBytes) {
    card.querySelectorAll('.chart-y-label').forEach(el => el.remove());

    const maxLabel = el('div', {
        class: 'chart-y-label chart-y-label--top'
    }, formatBytes(maxMemoryBytes, 0));

    const midLabel = el('div', {
        class: 'chart-y-label chart-y-label--mid'
    }, formatBytes(Math.round(maxMemoryBytes / 2), 0));

    const minLabel = el('div', {
        class: 'chart-y-label chart-y-label--bottom'
    }, '0 B');

    card.appendChild(maxLabel);
    card.appendChild(midLabel);
    card.appendChild(minLabel);
}

let currentUpdateData = null;

/**
 * Updates the version badge and update button according to the current updater state.
 * @param {object} data - Updater state (has_update, status, latest_version, progress, etc.).
 * @returns {void}
 */
export function updateDashboardVersionUI(data) {
    const versionEl = document.getElementById('dashboard-version');
    const updateBtn = document.getElementById('btn-dashboard-update');
    if (!versionEl || !updateBtn) return;

    if (data.status === 'disabled') {
        if (longPressTimer) clearTimeout(longPressTimer);
        longPressTimer = null;
        updateBtn.classList.remove('is-pressing');
        document.getElementById('dashboard-version-badge')?.remove();
        updateBtn.disabled = true;
        updateBtn.onclick = null;
        updateBtn.textContent = t('updater.containerDisabled');
        updateBtn.title = t('updater.containerManaged');
        return;
    }

    setupUpdateBtnLongPress(updateBtn);

    let badge = document.getElementById('dashboard-version-badge');
    if (data.has_update || data.status === 'available') {
        if (!badge) {
            badge = el('span', {
                id: 'dashboard-version-badge',
                class: 'badge badge-warning'
            });
            versionEl.parentNode.insertBefore(badge, versionEl.nextSibling);
        } else {
            badge.className = 'badge badge-warning';
        }
        badge.textContent = t('dashboard.hasNewVersion', {version: data.latest_version});

        updateBtn.textContent = t('dashboard.downloadUpdate');
        updateBtn.className = 'pill-btn pill-btn--primary pill-btn--sm manager-only';
        updateBtn.disabled = false;
        updateBtn.onclick = async () => {
            const updateInfo = currentUpdateData || data;
            if (window.showUpdateModal) {
                const confirmed = await window.showUpdateModal(updateInfo);
                if (confirmed) {
                    await installAppUpdate();
                }
            } else if (await window.showConfirm(t('dashboard.confirmDownload', {version: updateInfo.latest_version}))) {
                await installAppUpdate();
            }
        };
    } else if (data.status === 'ready_to_restart') {
        isLongPressTriggered = false;
        if (!badge) {
            badge = el('span', {
                id: 'dashboard-version-badge',
                class: 'badge badge-success'
            });
            versionEl.parentNode.insertBefore(badge, versionEl.nextSibling);
        } else {
            badge.className = 'badge badge-success';
        }
        badge.textContent = t('dashboard.versionReady');

        updateBtn.textContent = t('dashboard.restartToUpdate');
        updateBtn.className = 'pill-btn pill-btn--success pill-btn--sm manager-only';
        updateBtn.disabled = false;
        updateBtn.onclick = async () => {
            if (await window.showConfirm(t('dashboard.confirmRestart'))) {
                await restartApp();
            }
        };
    } else if (data.status === 'downloading') {
        updateBtn.textContent = t('dashboard.downloadingProgress', {progress: data.progress || 0});
        updateBtn.className = 'pill-btn pill-btn--soft pill-btn--sm manager-only';
        updateBtn.disabled = true;
    } else {
        if (badge) badge.remove();
        updateBtn.textContent = t('dashboard.checkUpdate');
        updateBtn.className = 'pill-btn pill-btn--soft pill-btn--sm manager-only';
        updateBtn.disabled = false;
        updateBtn.onclick = () => checkAppUpdate(true);
    }
}

/**
 * Quietly polls updater status and refreshes the dashboard version UI when not idle.
 * @returns {Promise<void>}
 */
export async function fetchUpdaterStatusQuietly() {
    try {
        const {response, data: status} = await fetchProto('/api/updater/status', UpdateState);
        if (response.ok && status) {
            if (status.status !== 'idle') {
                currentUpdateData = status;
                updateDashboardVersionUI(status);
            }
        }
    } catch {
    }
}

window.fetchUpdaterStatusQuietly = fetchUpdaterStatusQuietly;

/**
 * Checks for application updates via the updater API.
 * Manual checks show button progress while the backend records the result in the message center.
 * @param {boolean} [manual=false] - Whether the check was user-initiated.
 * @returns {Promise<void>}
 */
export async function checkAppUpdate(manual = false) {
    const updateBtn = document.getElementById('btn-dashboard-update');
    const origText = updateBtn ? updateBtn.textContent : '';
    let renderedResult = false;

    if (manual && updateBtn) {
        updateBtn.disabled = true;
        updateBtn.textContent = t('dashboard.checkingUpdate');
    }

    try {
        const headers = getAuthHeaders();
        const resp = await fetch('/api/updater/check', {method: 'POST', headers});
        if (!resp.ok) {
            if (manual) showAlert(t('updaterNotice.checkFailedTitle'), 'error');
            return;
        }

        const data = await resp.json();
        currentUpdateData = data;

        updateDashboardVersionUI(data);
        renderedResult = true;
        if (manual) {
            if (data.has_update) {
                showAlert(t('updaterNotice.availableTitle', {version: data.latest_version || ''}), 'info');
            } else {
                showAlert(t('updaterNotice.currentTitle'), 'success');
            }
        }
    } catch (e) {
        console.error('Update check error', e);
        if (manual) showAlert(t('updaterNotice.checkFailedTitle'), 'error');
    } finally {
        if (manual && updateBtn && !renderedResult) {
            if (currentUpdateData) updateDashboardVersionUI(currentUpdateData);
            else {
                updateBtn.disabled = false;
                updateBtn.textContent = origText || t('dashboard.checkUpdate');
            }
        }
    }
}

/**
 * Starts downloading/installing the available app update and begins status polling.
 * @returns {Promise<void>}
 */
export async function installAppUpdate() {
    try {
        const headers = getAuthHeaders();
        const resp = await fetch('/api/updater/install', {method: 'POST', headers});
        if (resp.ok) {
            showAlert(t('dashboard.downloadingBg'), 'info');
            pollUpdaterStatus();
        } else {
            showAlert(updaterErrorMessage(resp, 'updaterNotice.installFailedTitle'), 'error');
        }
    } catch (e) {
        console.error('Install update error', e);
        showAlert(t('updaterNotice.installFailedTitle'), 'error');
    } finally {
        await refreshMessageUnreadCount();
    }
}

let updaterPollTimer = null;

/**
 * Polls updater status every 3s until ready_to_restart or error; prompts restart when ready.
 * No-ops if a poll interval is already running.
 * @returns {void}
 */
export function pollUpdaterStatus() {
    if (updaterPollTimer) return;
    updaterPollTimer = setInterval(async () => {
        try {
            const {response, data: status} = await fetchProto('/api/updater/status', UpdateState);
            if (!response.ok || !status) return;
            currentUpdateData = status;
            updateDashboardVersionUI(status);

            if (status.status === 'ready_to_restart') {
                clearInterval(updaterPollTimer);
                updaterPollTimer = null;
                await refreshMessageUnreadCount();
            } else if (status.status === 'error') {
                clearInterval(updaterPollTimer);
                updaterPollTimer = null;
                await refreshMessageUnreadCount();
            }
        } catch {
        }
    }, 3000);
}

/**
 * Requests an application restart to apply a downloaded update, then reloads the page.
 * @returns {Promise<void>}
 */
export async function restartApp() {
    showAlert(t('dashboard.restarting'), 'info');
    try {
        const headers = getAuthHeaders();
        const resp = await fetch('/api/updater/restart', {method: 'POST', headers});
        if (!resp.ok) {
            showAlert(updaterErrorMessage(resp, 'updaterNotice.restartFailedTitle'), 'error');
            await refreshMessageUnreadCount();
            return;
        }
        setTimeout(() => window.location.reload(), 4000);
    } catch (e) {
        console.error('Restart error', e);
    }
}

/**
 * Starts uptime ticker, initial status/snapshot fetches, and periodic refresh intervals.
 * @returns {void}
 */
export function startDashboardRefresh() {
    const logsButton = document.getElementById('btn-dashboard-logs');
    if (logsButton) logsButton.onclick = () => openAuditLogsDialog({mode: 'global'});
    _startUptimeTicker();
    fetchInstanceStatus();
    fetchSnapshots();
    if (!refreshInterval) {
        refreshInterval = setInterval(fetchInstanceStatus, 10000);
    }
    if (!snapshotsInterval) {
        snapshotsInterval = setInterval(fetchSnapshots, 60000);
    }
}

/**
 * Stops the uptime ticker and all dashboard refresh intervals.
 * @returns {void}
 */
export function stopDashboardRefresh() {
    _stopUptimeTicker();
    if (refreshInterval) {
        clearInterval(refreshInterval);
        refreshInterval = null;
    }
    if (snapshotsInterval) {
        clearInterval(snapshotsInterval);
        snapshotsInterval = null;
    }
}

window.addEventListener('languageChanged', () => {
    const memoryEl = document.getElementById('dashboard-memory');
    if (memoryEl && memoryEl.dataset.totalMemory !== undefined) {
        const rssBytes = Number(memoryEl.dataset.rssMemory ?? memoryEl.dataset.usedMemory ?? 0);
        const vssBytes = Number(memoryEl.dataset.vssMemory ?? 0);
        const totalBytes = Number(memoryEl.dataset.totalMemory);
        if (rssBytes === vssBytes || vssBytes === 0) {
            const procBytes = rssBytes || vssBytes;
            memoryEl.textContent = t('dashboard.memoryFormatSimple', {
                used: formatBytes(procBytes),
                process: formatBytes(procBytes),
                total: formatBytes(totalBytes)
            });
        } else {
            memoryEl.textContent = t('dashboard.memoryFormat', {
                rss: formatBytes(rssBytes),
                vss: formatBytes(vssBytes),
                total: formatBytes(totalBytes)
            });
        }
    }
    if (_lastSnapshotsData) {
        drawSnapshotsChart(_lastSnapshotsData);
    }
    const diskEl = document.getElementById('dashboard-disk');
    if (diskEl && diskEl.dataset.renopUsed !== undefined && diskEl.dataset.diskUsed !== undefined && diskEl.dataset.diskTotal !== undefined) {
        diskEl.textContent = t('dashboard.diskFormat', {
            renopUsed: formatBytes(Number(diskEl.dataset.renopUsed)),
            diskUsed: formatBytes(Number(diskEl.dataset.diskUsed)),
            diskTotal: formatBytes(Number(diskEl.dataset.diskTotal))
        });
    }
    if (currentUpdateData) {
        updateDashboardVersionUI(currentUpdateData);
    } else {
        const updateBtn = document.getElementById('btn-dashboard-update');
        if (updateBtn && !updateBtn.disabled && (!currentUpdateData || !currentUpdateData.has_update)) {
            updateBtn.textContent = t('dashboard.checkUpdate');
        }
    }
});
