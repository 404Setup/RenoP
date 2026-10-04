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

/** Create the database management view component. */
export function renderDatabase() {
    return el("div", {"class": "tab-content", "id": "tab-content-database", "style": "display: none;"},
        el("div", {"class": "database-hero dashboard-hero"},
            el("div", {"style": "display: flex; align-items: center; gap: 1rem;"},
                el("button", {
                    "class": "pill-btn pill-btn--soft pill-btn--sm",
                    "id": "btn-database-back",
                    "type": "button",
                    "style": "display: flex; align-items: center; gap: 0.35rem;"
                },
                    svg("svg", {
                        "fill": "none",
                        "stroke": "currentColor",
                        "stroke-linecap": "round",
                        "stroke-linejoin": "round",
                        "stroke-width": "2",
                        "viewBox": "0 0 24 24",
                        "width": "16",
                        "height": "16"
                    },
                        svg("line", {"x1": "19", "y1": "12", "x2": "5", "y2": "12"}),
                        svg("polyline", {"points": "12 19 5 12 12 5"})
                    ),
                    el("span", {"data-i18n": "database.backToDashboard"}, "Back to Dashboard")
                ),
                el("div", {"class": "dashboard-hero-text"},
                    el("h2", {"class": "dashboard-title", "data-i18n": "database.title"}, "Database Management"),
                    el("p", {
                        "class": "dashboard-subtitle",
                        "data-i18n": "database.subtitle"
                    }, "Inspect database occupancy and reclaim non-essential data")
                )
            ),
            el("div", {"style": "display: flex; align-items: center; gap: 0.75rem;"},
                el("button", {
                    "class": "pill-btn pill-btn--primary",
                    "id": "btn-clean-database",
                    "type": "button",
                    "style": "display: flex; align-items: center; gap: 0.5rem;"
                },
                    svg("svg", {
                        "fill": "none",
                        "stroke": "currentColor",
                        "stroke-linecap": "round",
                        "stroke-linejoin": "round",
                        "stroke-width": "2",
                        "viewBox": "0 0 24 24",
                        "width": "16",
                        "height": "16"
                    },
                        svg("polyline", {"points": "3 6 5 6 21 6"}),
                        svg("path", {"d": "M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"})
                    ),
                    el("span", {"data-i18n": "database.cleanNow"}, "Clean Non-Essential Data")
                )
            )
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
                        "viewBox": "0 0 24 24"
                    },
                        svg("ellipse", {"cx": "12", "cy": "5", "rx": "9", "ry": "3"}),
                        svg("path", {"d": "M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"}),
                        svg("path", {"d": "M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "database.driver"}, "Database Engine"),
                    el("div", {"class": "dashboard-card-value", "id": "database-driver"}, "...")
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
                        "viewBox": "0 0 24 24"
                    },
                        svg("rect", {"height": "16", "rx": "2", "width": "20", "x": "2", "y": "4"}),
                        svg("line", {"x1": "6", "x2": "10", "y1": "8", "y2": "8"}),
                        svg("line", {"x1": "6", "x2": "14", "y1": "12", "y2": "12"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "database.totalSize"}, "Total Database Size"),
                    el("div", {"class": "dashboard-card-value", "id": "database-total-size"}, "...")
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
                        "viewBox": "0 0 24 24"
                    },
                        svg("line", {"x1": "8", "x2": "21", "y1": "6", "y2": "6"}),
                        svg("line", {"x1": "8", "x2": "21", "y1": "12", "y2": "12"}),
                        svg("line", {"x1": "8", "x2": "21", "y1": "18", "y2": "18"}),
                        svg("line", {"x1": "3", "x2": "3.01", "y1": "6", "y2": "6"}),
                        svg("line", {"x1": "3", "x2": "3.01", "y1": "12", "y2": "12"}),
                        svg("line", {"x1": "3", "x2": "3.01", "y1": "18", "y2": "18"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "database.reclaimable"}, "Reclaimable Rows"),
                    el("div", {"class": "dashboard-card-value", "id": "database-reclaimable-rows"}, "...")
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
                        "viewBox": "0 0 24 24"
                    },
                        svg("path", {"d": "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}),
                        svg("polyline", {"points": "7 10 12 15 17 10"}),
                        svg("line", {"x1": "12", "y1": "15", "x2": "12", "y2": "3"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "database.reclaimable"}, "Reclaimable Size"),
                    el("div", {"class": "dashboard-card-value", "id": "database-reclaimable-bytes"}, "...")
                )
            )
        ),
        el("div", {"class": "dashboard-section"},
            el("div", {"class": "dashboard-chart-card", "style": "padding: 1.25rem; overflow-x: auto;"},
                el("table", {"class": "pure-table pure-table-horizontal database-table", "style": "width: 100%; border-collapse: collapse;"},
                    el("thead", {},
                        el("tr", {},
                            el("th", {"style": "text-align: left; padding: 0.75rem 1rem;", "data-i18n": "database.tableName"}, "Table Name"),
                            el("th", {"style": "text-align: right; padding: 0.75rem 1rem;", "data-i18n": "database.rowCount"}, "Rows"),
                            el("th", {"style": "text-align: right; padding: 0.75rem 1rem;", "data-i18n": "database.tableSize"}, "Size"),
                            el("th", {"style": "text-align: center; padding: 0.75rem 1rem;", "data-i18n": "database.status"}, "Status")
                        )
                    ),
                    el("tbody", {"id": "database-tables-tbody"})
                )
            )
        )
    );
}
