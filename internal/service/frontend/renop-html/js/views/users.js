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

/** Create the users component. */
export function renderUsers() {
    return el("div", {"class": "tab-content", "id": "tab-content-users", "style": "display: none;"},
        el("div", {"class": "settings-header"},
            el("div", {},
                el("h2", {"data-i18n": "users.title"}, "User Management"),
                el("p", {"data-i18n": "users.subtitle"}, "Create and manage user accounts and access permissions.")
            )
        ),
        el("div", {"class": "users-stats-cards"},
            el("div", {"class": "stat-card"},
                el("div", {"class": "stat-icon stat-icon--total"},
                    svg("svg", {
                            "fill": "none",
                            "height": "20",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "width": "20"
                        },
                        svg("path", {"d": "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"}),
                        svg("circle", {"cx": "9", "cy": "7", "r": "4"}),
                        svg("path", {"d": "M23 21v-2a4 4 0 0 0-3-3.87"}),
                        svg("path", {"d": "M16 3.13a4 4 0 0 1 0 7.75"})
                    )
                ),
                el("div", {"class": "stat-content"},
                    el("div", {"class": "stat-label", "data-i18n": "users.statTotal"}, "Total Users"),
                    el("div", {"class": "stat-value", "id": "stat-total-users"}, "0")
                )
            ),
            el("div", {"class": "stat-card"},
                el("div", {"class": "stat-icon stat-icon--admin"},
                    svg("svg", {
                            "fill": "none",
                            "height": "20",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "width": "20"
                        },
                        svg("path", {"d": "M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"})
                    )
                ),
                el("div", {"class": "stat-content"},
                    el("div", {"class": "stat-label", "data-i18n": "users.statAdmins"}, "Administrators"),
                    el("div", {"class": "stat-value", "id": "stat-admin-users"}, "0")
                )
            ),
            el("div", {"class": "stat-card"},
                el("div", {"class": "stat-icon stat-icon--active"},
                    svg("svg", {
                            "fill": "none",
                            "height": "20",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2",
                            "viewBox": "0 0 24 24",
                            "width": "20"
                        },
                        svg("rect", {"height": "11", "rx": "2", "width": "18", "x": "3", "y": "11"}),
                        svg("path", {"d": "M7 11V7a5 5 0 0 1 10 0v4"})
                    )
                ),
                el("div", {"class": "stat-content"},
                    el("div", {"class": "stat-label", "data-i18n": "users.statTokens"}, "API Tokens"),
                    el("div", {"class": "stat-value", "id": "stat-key-users"}, "0")
                )
            )
        ),
        el("div", {"class": "users-toolbar"},
            el("div", {"class": "search-input-wrapper"},
                svg("svg", {
                        "class": "search-icon",
                        "fill": "none",
                        "height": "16",
                        "stroke": "currentColor",
                        "stroke-linecap": "round",
                        "stroke-linejoin": "round",
                        "stroke-width": "2.5",
                        "viewBox": "0 0 24 24",
                        "width": "16"
                    },
                    svg("circle", {"cx": "11", "cy": "11", "r": "8"}),
                    svg("line", {"x1": "21", "x2": "16.65", "y1": "21", "y2": "16.65"})
                ),
                el("input", {
                    "autocomplete": "off",
                    "data-i18n-placeholder": "users.searchPlaceholder",
                    "id": "users-search-input",
                    "placeholder": "Search users by name or permission...",
                    "type": "text"
                })
            ),
            el("div", {
                    "class": "users-toolbar-actions",
                    "style": "display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap;"
                },
                el("button", {
                        "class": "settings-save-btn",
                        "data-i18n-title": "users.createUserBtnTitle",
                        "id": "btn-create-user",
                        "title": "Create a new user",
                        "type": "button"
                    },
                    svg("svg", {
                            "aria-hidden": "true",
                            "fill": "none",
                            "height": "16",
                            "stroke": "currentColor",
                            "stroke-linecap": "round",
                            "stroke-linejoin": "round",
                            "stroke-width": "2.5",
                            "viewBox": "0 0 24 24",
                            "width": "16"
                        },
                        svg("line", {"x1": "12", "x2": "12", "y1": "5", "y2": "19"}),
                        svg("line", {"x1": "5", "x2": "19", "y1": "12", "y2": "12"})
                    ),
                    el("span", {"data-i18n": "users.createUserBtn"}, "Create User")
                )
            )
        ),
        el("div", {"class": "border-container users-table-container", "style": "padding: 0;"},
            el("table", {"id": "tokens-table"},
                el("thead", {},
                    el("tr", {},
                        el("th", {"data-i18n": "users.thUser"}, "User"),
                        el("th", {"data-i18n": "users.thPermissions"}, "Permissions"),
                        el("th", {"data-i18n": "users.thCreatedAt"}, "Created At"),
                        el("th", {"data-i18n": "users.thActions", "style": "text-align: right;"}, "Actions")
                    )
                ),
                el("tbody", {"id": "tokens-table-body"})
            ),
            el("div", {"class": "table-pagination", "id": "users-pagination"})
        )
    );
}
