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

/** Create the storage management view component. */
export function renderStorage() {
    return el("div", {"class": "tab-content", "id": "tab-content-storage", "style": "display: none;"},
        el("div", {"class": "storage-hero dashboard-hero"},
            el("div", {"style": "display: flex; align-items: center; gap: 1rem;"},
                el("button", {
                    "class": "pill-btn pill-btn--soft pill-btn--sm",
                    "id": "btn-storage-back",
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
                    el("span", {"data-i18n": "storage.backToDashboard"}, "Back to Dashboard")
                ),
                el("div", {"class": "dashboard-hero-text"},
                    el("h2", {"class": "dashboard-title", "data-i18n": "storage.title"}, "Storage Management"),
                    el("p", {
                        "class": "dashboard-subtitle",
                        "data-i18n": "storage.subtitle"
                    }, "Inspect RenoP storage breakdown and reclaim temporary or residual space")
                )
            ),
            el("div", {"style": "display: flex; align-items: center; gap: 0.75rem;"},
                el("button", {
                    "class": "pill-btn pill-btn--primary",
                    "id": "btn-clean-storage",
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
                    el("span", {"data-i18n": "storage.cleanNow"}, "Clean Storage Space")
                )
            )
        ),
        el("div", {"class": "dashboard-cards"},
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
                        svg("path", {"d": "M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "storage.repositories"}, "Package Repositories"),
                    el("div", {"class": "dashboard-card-value", "id": "storage-renop-used"}, "...")
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
                    el("div", {"class": "dashboard-card-title", "data-i18n": "storage.updaterResiduals"}, "Updater Residuals"),
                    el("div", {"class": "dashboard-card-value", "id": "storage-updater-residuals"}, "...")
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
                        svg("rect", {"height": "14", "rx": "2", "width": "20", "x": "2", "y": "5"}),
                        svg("line", {"x1": "2", "x2": "22", "y1": "10", "y2": "10"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "storage.tempFiles"}, "Temporary Uploads & Chunks"),
                    el("div", {"class": "dashboard-card-value", "id": "storage-temp-files"}, "...")
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
                        "viewBox": "0 0 24 24"
                    },
                        svg("ellipse", {"cx": "12", "cy": "5", "rx": "9", "ry": "3"}),
                        svg("path", {"d": "M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"}),
                        svg("path", {"d": "M3 12c0 1.66 4.03 3 9 3s9-1.34 9-3"})
                    )
                ),
                el("div", {"class": "dashboard-card-body"},
                    el("div", {"class": "dashboard-card-title", "data-i18n": "dashboard.disk"}, "Disk Usage"),
                    el("div", {"class": "dashboard-card-value", "id": "storage-disk-summary"}, "...")
                )
            )
        ),
        el("div", {"class": "dashboard-section"},
            el("div", {"class": "dashboard-chart-card", "style": "padding: 1.25rem; overflow-x: auto;"},
                el("table", {"class": "pure-table pure-table-horizontal storage-table", "style": "width: 100%; border-collapse: collapse;"},
                    el("thead", {},
                        el("tr", {},
                            el("th", {"data-i18n": "storage.category", "style": "text-align: left; padding: 0.75rem 1rem;"}, "Category"),
                            el("th", {"data-i18n": "storage.name", "style": "text-align: left; padding: 0.75rem 1rem;"}, "Name"),
                            el("th", {"data-i18n": "storage.files", "style": "text-align: right; padding: 0.75rem 1rem;"}, "Files"),
                            el("th", {"data-i18n": "storage.size", "style": "text-align: right; padding: 0.75rem 1rem;"}, "Size")
                        )
                    ),
                    el("tbody", {"id": "storage-items-tbody"})
                )
            )
        )
    );
}
