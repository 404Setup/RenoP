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

/** Create the dashboard component. */
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
            el("button", {
                "class": "pill-btn manager-only",
                "data-i18n": "audit.globalTitle",
                "id": "btn-dashboard-logs",
                "type": "button"
            }, "Global logs "),
            el("div", {
                "class": "dashboard-status-dot",
                "data-i18n-title": "dashboard.healthy",
                "id": "dashboard-status-indicator",
                "title": "Instance healthy"
            })
        ),
        el("div", {"class": "dashboard-cards"},
            el("div", {"class": "dashboard-card dashboard-card--version"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("path", {"d": "M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"}),
                        svg("line", {"x1": "7", "x2": "7.01", "y1": "7", "y2": "7"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.version"}, "Version"),
                    el("div", {
                            "class": "dashboard-card-value",
                            "style": "display:flex; flex-direction:column; align-items:flex-start; gap:0.25rem;"
                        },
                        el("a", {
                            "href": "https://github.com/404Setup/RenoP",
                            "id": "dashboard-version",
                            "target": "_blank"
                        }, "..."),
                        el("button", {
                            "class": "pill-btn pill-btn--soft pill-btn--sm manager-only",
                            "data-i18n": "dashboard.checkUpdate",
                            "id": "btn-dashboard-update",
                            "style": "padding: 0.2rem 0.6rem; font-size: 0.75rem;",
                            "type": "button"
                        }, "Check for Updates ")
                    )
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--uptime"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("circle", {"cx": "12", "cy": "12", "r": "10"}),
                        svg("polyline", {"points": "12 6 12 12 16 14"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.uptime"}, "Uptime"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-uptime"}, "...")
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--memory"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("rect", {"height": "12", "rx": "2", "width": "20", "x": "2", "y": "6"}),
                        svg("line", {"x1": "6", "x2": "6", "y1": "10", "y2": "14"}),
                        svg("line", {"x1": "10", "x2": "10", "y1": "10", "y2": "14"}),
                        svg("line", {"x1": "14", "x2": "14", "y1": "10", "y2": "14"}),
                        svg("line", {"x1": "18", "x2": "18", "y1": "10", "y2": "14"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.memory"}, "Memory Usage"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-memory"}, "...")
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--disk"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("ellipse", {"cx": "12", "cy": "5", "rx": "9", "ry": "3"}),
                        svg("path", {"d": "M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"}),
                        svg("path", {"d": "M3 12c0 1.66 4.03 3 9 3s9-1.34 9-3"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.disk"}, "Disk Usage"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-disk"}, "...")
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--threads"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("path", {"d": "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"}),
                        svg("circle", {"cx": "9", "cy": "7", "r": "4"}),
                        svg("path", {"d": "M23 21v-2a4 4 0 0 0-3-3.87"}),
                        svg("path", {"d": "M16 3.13a4 4 0 0 1 0 7.75"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.threads"}, "Threads"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-threads"}, "...")
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--cores"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
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
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.cores"}, "Cores (Log / Phy)"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-cores"}, "...")
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--os"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("rect", {"height": "14", "rx": "2", "width": "20", "x": "2", "y": "3"}),
                        svg("line", {"x1": "8", "x2": "16", "y1": "21", "y2": "21"}),
                        svg("line", {"x1": "12", "x2": "12", "y1": "17", "y2": "21"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.os"}, "OS / Arch"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-os-arch"}, "...")
                )
            ),
            el("div", {"class": "dashboard-card dashboard-card--failures"},
                el("div", {"aria-hidden": "true", "class": "dashboard-card-icon"},
                    svg("svg", {
                            "fill": "none",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "xmlns": "http://www.w3.org/2000/svg"
                        },
                        svg("path", {"d": "M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"}),
                        svg("line", {"x1": "12", "x2": "12", "y1": "9", "y2": "13"}),
                        svg("line", {"x1": "12", "x2": "12.01", "y1": "17", "y2": "17"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.failures"}, "Failures"),
                    el("div", {"class": "dashboard-card-value", "id": "dashboard-failures"}, "...")
                )
            )
        ),
        el("div", {"class": "dashboard-section", "id": "snapshots-chart-container", "style": "display: none;"},
            el("div", {"class": "dashboard-section-header"},
                el("h3", {"class": "dashboard-section-title", "data-i18n": "dashboard.chartTitle"}, "Memory Over Time"),
                el("span", {"class": "dashboard-section-badge", "data-i18n": "dashboard.liveBadge"}, "Live")
            ),
            el("div", {"class": "dashboard-chart-card"},
                el("div", {"class": "snapshots-chart", "id": "snapshots-chart"}),
                el("div", {"class": "dashboard-chart-legend"},
                    el("span", {"class": "dashboard-chart-legend-item"},
                        el("span", {
                            "aria-hidden": "true",
                            "class": "dashboard-chart-legend-swatch dashboard-chart-legend-swatch--rss"
                        }),
                        el("span", {"data-i18n": "dashboard.chartLegendRss"}, "RSS (resident)")
                    ),
                    el("span", {"class": "dashboard-chart-legend-item"},
                        el("span", {
                            "aria-hidden": "true",
                            "class": "dashboard-chart-legend-swatch dashboard-chart-legend-swatch--vss"
                        }),
                        el("span", {"data-i18n": "dashboard.chartLegendVss"}, "VSS (virtual)")
                    )
                )
            )
        ),
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
