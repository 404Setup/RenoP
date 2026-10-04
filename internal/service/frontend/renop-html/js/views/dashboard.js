/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

import {el, svg} from '@renop/ui/dom';

/** Create the dashboard component with denser aggregated cards. */
export function renderDashboard() {
    return el("div", {"class": "tab-content", "id": "tab-content-dashboard", "style": "display: none;"},
        el("div", {"class": "dashboard-hero"},
            el("div", {"class": "dashboard-hero-text"},
                el("h2", {"class": "dashboard-title", "data-i18n": "dashboard.title"}, "Instance Status"),
                el("p", {
                    "class": "dashboard-subtitle",
                    "data-i18n": "dashboard.subtitle"
                }, "Real-time metrics for your Renop instance")
            ),
            el("div", {"class": "dashboard-hero-actions"},
                el("button", {
                    "class": "pill-btn pill-btn--soft pill-btn--sm manager-only",
                    "id": "btn-dashboard-logs",
                    "type": "button"
                },
                    svg("svg", {
                        "fill": "none",
                        "stroke": "currentColor",
                        "stroke-linecap": "round",
                        "stroke-linejoin": "round",
                        "stroke-width": "2",
                        "viewBox": "0 0 24 24",
                        "style": "width: 13px; height: 13px; margin-right: 0.35rem; vertical-align: -1px;"
                    },
                        svg("path", {"d": "M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"}),
                        svg("polyline", {"points": "14 2 14 8 20 8"}),
                        svg("line", {"x1": "16", "x2": "8", "y1": "13", "y2": "13"}),
                        svg("line", {"x1": "16", "x2": "8", "y1": "17", "y2": "17"}),
                        svg("polyline", {"points": "10 9 9 9 8 9"})
                    ),
                    el("span", {"data-i18n": "audit.globalTitle"}, "Global logs")
                ),
                el("div", {
                    "class": "dashboard-status-dot",
                    "data-i18n-title": "dashboard.healthy",
                    "id": "dashboard-status-indicator",
                    "title": "Instance healthy"
                })
            )
        ),
        el("div", {"class": "dashboard-large-cards"},
            // Card 1: 运行状态与系统概览
            el("div", {"class": "dashboard-card dashboard-card--large dashboard-card--overview"},
                el("div", {"class": "dashboard-card-header"},
                    el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                        svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24"
                        },
                            svg("rect", {"height": "16", "rx": "2", "width": "16", "x": "4", "y": "4"}),
                            svg("rect", {"height": "6", "width": "6", "x": "9", "y": "9"}),
                            svg("line", {"x1": "9", "x2": "9", "y1": "1", "y2": "4"}),
                            svg("line", {"x1": "15", "x2": "15", "y1": "1", "y2": "4"}),
                            svg("line", {"x1": "9", "x2": "9", "y1": "20", "y2": "23"}),
                            svg("line", {"x1": "15", "x2": "15", "y1": "20", "y2": "23"}),
                            svg("line", {"x1": "20", "x2": "23", "y1": "9", "y2": "9"}),
                            svg("line", {"x1": "20", "x2": "23", "y1": "14", "y2": "14"}),
                            svg("line", {"x1": "1", "x2": "4", "y1": "9", "y2": "9"}),
                            svg("line", {"x1": "1", "x2": "4", "y1": "14", "y2": "14"})
                        )
                    ),
                    el("div", {"class": "dashboard-card-heading"},
                        el("h3", {"data-i18n": "dashboard.systemOverview"}, "System Overview")
                    )
                ),
                el("div", {"class": "dashboard-panel-grid dashboard-panel-grid--overview"},
                    el("div", {"class": "dashboard-metric-cell"},
                        el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.version"}, "Version"),
                        el("div", {"class": "dashboard-card-value dashboard-card-value--version"},
                            el("a", {
                                "href": "https://github.com/404Setup/RenoP",
                                "id": "dashboard-version",
                                "target": "_blank"
                            }, "..."),
                            el("button", {
                                "class": "pill-btn pill-btn--soft pill-btn--sm manager-only",
                                "data-i18n": "dashboard.checkUpdate",
                                "id": "btn-dashboard-update",
                                "style": "padding: 0.15rem 0.5rem; font-size: 0.72rem;",
                                "type": "button"
                            }, "Check for Updates ")
                        )
                    ),
                    el("div", {"class": "dashboard-metric-cell"},
                        el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.uptime"}, "Uptime"),
                        el("div", {"class": "dashboard-card-value", "id": "dashboard-uptime"}, "...")
                    ),
                    el("div", {"class": "dashboard-metric-cell"},
                        el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.os"}, "OS / Arch"),
                        el("div", {"class": "dashboard-card-value", "id": "dashboard-os-arch"}, "...")
                    ),
                    el("div", {"class": "dashboard-metric-cell"},
                        el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.cores"}, "Cores (Log / Phy)"),
                        el("div", {"class": "dashboard-card-value", "id": "dashboard-cores"}, "...")
                    ),
                    el("div", {"class": "dashboard-metric-cell"},
                        el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.threads"}, "Threads"),
                        el("div", {"class": "dashboard-card-value", "id": "dashboard-threads"}, "...")
                    ),
                    el("div", {"class": "dashboard-metric-cell"},
                        el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.failures"}, "Failures"),
                        el("div", {"class": "dashboard-card-value", "id": "dashboard-failures"}, "...")
                    )
                )
            ),
            // Card 2: 存储与资源概况
            el("div", {"class": "dashboard-card dashboard-card--large dashboard-card--resources"},
                el("div", {"class": "dashboard-card-header"},
                    el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                        svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24"
                        },
                            svg("ellipse", {"cx": "12", "cy": "5", "rx": "9", "ry": "3"}),
                            svg("path", {"d": "M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"}),
                            svg("path", {"d": "M3 12c0 1.66 4.03 3 9 3s9-1.34 9-3"})
                        )
                    ),
                    el("div", {"class": "dashboard-card-heading"},
                        el("h3", {"data-i18n": "dashboard.resources"}, "Storage & Resources")
                    )
                ),
                el("div", {"class": "dashboard-panel-grid dashboard-panel-grid--resources"},
                    el("div", {"class": "dashboard-metric-cell dashboard-metric-cell--actionable"},
                        el("div", {"class": "dashboard-cell-main"},
                            el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.memory"}, "Memory Usage"),
                            el("div", {"class": "dashboard-card-value", "id": "dashboard-memory"}, "...")
                        ),
                        el("span", {"class": "dashboard-badge-ram"}, "RAM")
                    ),
                    el("div", {"class": "dashboard-metric-cell dashboard-metric-cell--actionable"},
                        el("div", {"class": "dashboard-cell-main"},
                            el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.disk"}, "Disk Usage"),
                            el("div", {"class": "dashboard-card-value", "id": "dashboard-disk"}, "...")
                        ),
                        el("button", {
                            "class": "pill-btn pill-btn--soft pill-btn--sm",
                            "id": "btn-goto-storage",
                            "type": "button",
                            "data-i18n": "dashboard.manageStorage"
                        }, "Manage Storage")
                    ),
                    el("div", {"class": "dashboard-metric-cell dashboard-metric-cell--actionable"},
                        el("div", {"class": "dashboard-cell-main"},
                            el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.database"}, "Database Usage"),
                            el("div", {"class": "dashboard-card-value", "id": "dashboard-database"}, "...")
                        ),
                        el("button", {
                            "class": "pill-btn pill-btn--soft pill-btn--sm",
                            "id": "btn-goto-database",
                            "type": "button",
                            "data-i18n": "dashboard.manageDatabase"
                        }, "Manage Database")
                    ),
                    el("div", {"class": "dashboard-metric-cell dashboard-metric-cell--actionable"},
                        el("div", {"class": "dashboard-cell-main"},
                            el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.cache"}, "Cache Usage"),
                            el("div", {"class": "dashboard-card-value", "id": "dashboard-cache"}, "...")
                        ),
                        el("button", {
                            "class": "pill-btn pill-btn--soft pill-btn--sm",
                            "id": "btn-purge-cache",
                            "type": "button",
                            "data-i18n": "dashboard.purgeCache"
                        }, "Purge Cache")
                    )
                )
            )
        ),
        // Card 3: 内存使用趋势 (折线图)
        el("div", {"class": "dashboard-section", "id": "snapshots-chart-container", "style": "display: none;"},
            el("div", {"class": "dashboard-section-header"},
                el("h3", {"class": "dashboard-section-title", "data-i18n": "dashboard.chartTitle"}, "Memory Over Time"),
                el("span", {"class": "dashboard-section-badge", "data-i18n": "dashboard.liveBadge"}, "Live")
            ),
            el("div", {"class": "dashboard-chart-card"},
                el("div", {"class": "snapshots-chart", "id": "snapshots-chart"}),
                el("div", {"class": "dashboard-chart-legend", "id": "dashboard-chart-legend"})
            )
        ),
        // Debug section
        el("div", {"class": "dashboard-section dashboard-debug-section", "hidden": true, "id": "dashboard-debug-tools"},
            el("div", {"class": "dashboard-section-header"},
                el("h3", {"class": "dashboard-section-title", "data-i18n": "dashboard.debugTitle"}, "Debug memory"),
                el("span", {
                    "class": "dashboard-section-badge dashboard-section-badge--debug",
                    "data-i18n": "dashboard.debugBadge"
                }, "Debug")
            ),
            el("div", {"class": "dashboard-debug-card"},
                el("p", {
                    "class": "dashboard-debug-hint",
                    "data-i18n": "dashboard.debugMemoryHint"
                }, " Download a full Go heap profile (pprof) for flame-graph analysis. Open with Speedscope or go tool pprof. "),
                el("div", {"class": "dashboard-debug-actions"},
                    el("button", {
                        "class": "pill-btn pill-btn--soft",
                        "data-i18n": "dashboard.dumpHeap",
                        "id": "btn-dump-heap",
                        "type": "button"
                    }, " Dump heap flamegraph "),
                    el("button", {
                        "class": "pill-btn pill-btn--soft",
                        "data-i18n": "dashboard.dumpAllocs",
                        "id": "btn-dump-allocs",
                        "type": "button"
                    }, " Dump allocs profile "),
                    el("button", {
                        "class": "pill-btn pill-btn--soft",
                        "data-i18n": "dashboard.dumpGoroutine",
                        "id": "btn-dump-goroutine",
                        "type": "button"
                    }, " Dump goroutines ")
                )
            )
        )
    );
}
