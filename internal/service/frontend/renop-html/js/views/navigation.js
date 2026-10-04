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
import {createNavControls} from "@renop/ui";

/** Create the navigation component. @param {object} [config] */
export function renderNavigation(config = {}) {
    return el("div", {"class": "top-nav"},
        el("div", {"class": "top-nav-left"},
            el("h1", {"class": "nav-title"},
                el("a", {"href": "/", "id": "home-link", "title": config.title},
                    el("img", {
                        "alt": "",
                        "aria-hidden": "true",
                        "class": "nav-title-logo",
                        "id": "header-logo",
                        "src": config.organizationLogo
                    }),
                    el("span", {"class": "nav-title-label"}, config.title)
                )
            )
        ),
        el("div", {"class": "top-nav-right"},
            createNavControls({
                groupId: "auth-container",
                dividerId: "nav-divider",
                trailing: [
                el("button", {
                    "class": "nav-sign-in-btn",
                    "data-i18n": "nav.signIn",
                    "id": "login-btn",
                    "type": "button"
                }, "Sign in"),
                el("div", {"class": "nav-user-info", "id": "user-info", "style": "display: none;"},
                    el("div", {"class": "nav-profile-menu-wrap", "id": "profile-menu-wrap"},
                        el("button", {
                                "aria-controls": "profile-menu",
                                "aria-expanded": "false",
                                "aria-haspopup": "menu",
                                "class": "user-text nav-profile-trigger",
                                "data-i18n-title": "profile.openOwnProfile",
                                "id": "profile-trigger",
                                "title": "Open account menu",
                                "type": "button"
                            },
                            el("span", {"aria-hidden": "true", "class": "user-avatar-wrap"},
                                el("span", {"class": "user-avatar-dot", "id": "user-avatar-dot"}),
                                el("span", {
                                    "class": "message-unread-badge profile-message-unread-badge",
                                    "hidden": true,
                                    "id": "profile-message-unread-badge"
                                }, "0")
                            ),
                            el("span", {"class": "user-name", "id": "username-display"}),
                            svg("svg", {
                                    "aria-hidden": "true",
                                    "class": "nav-profile-chevron",
                                    "fill": "none",
                                    "height": "13",
                                    "stroke": "currentColor",
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2.2",
                                    "viewBox": "0 0 24 24",
                                    "width": "13"
                                },
                                svg("polyline", {"points": "6 9 12 15 18 9"})
                            )
                        ),
                        el("div", {"class": "nav-profile-menu", "hidden": true, "id": "profile-menu", "role": "menu"},
                            el("button", {
                                    "class": "nav-profile-menu-item",
                                    "data-profile-tab": "overview",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                                el("span", {"data-i18n": "nav.backHome"}, "Back to home")
                            ),
                            el("div", {"class": "nav-profile-menu-separator", "role": "separator"}),
                            el("button", {
                                    "class": "nav-profile-menu-item",
                                    "data-profile-action": "view",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "user"}),
                                el("span", {"data-i18n": "profile.viewOwnProfile"}, "View profile")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item",
                                    "data-profile-action": "edit",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "edit"}),
                                el("span", {"data-i18n": "profile.editOwnProfile"}, "Edit profile")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item",
                                    "data-account-action": "maven-domains",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "network"}),
                                el("span", {"data-i18n": "maven.domainSettings"}, "Domain settings")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item",
                                    "data-account-action": "super-teams",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "identity"}),
                                el("span", {"data-i18n": "superTeam.nav"}, "Global teams")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item",
                                    "data-account-action": "tickets",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "refresh"}),
                                el("span", {"data-i18n": "review.nav"}, "Reviews")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item message-center-menu-item",
                                    "data-account-action": "messages",
                                    "id": "message-center-btn",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "bell"}),
                                el("span", {"data-i18n": "messages.title"}, "Messages"),
                                el("span", {
                                    "class": "message-unread-badge",
                                    "hidden": true,
                                    "id": "message-unread-badge"
                                }, "0")
                            ),
                            el("div", {
                                "class": "nav-profile-menu-separator manager-only",
                                "data-auth-display": "block",
                                "role": "separator",
                                "style": "display: none;"
                            }),
                            el("button", {
                                    "class": "nav-profile-menu-item manager-only",
                                    "data-account-action": "compose-notification",
                                    "data-auth-display": "flex",
                                    "id": "message-compose-menu-btn",
                                    "role": "menuitem",
                                    "style": "display: none;",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "send"}),
                                el("span", {"data-i18n": "messages.compose"}, "Send notification")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item manager-only",
                                    "data-auth-display": "flex",
                                    "data-profile-tab": "dashboard",
                                    "role": "menuitem",
                                    "style": "display: none;",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "performance"}),
                                el("span", {"data-i18n": "tabs.dashboard"}, "Dashboard")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item manager-only",
                                    "data-auth-display": "flex",
                                    "data-profile-tab": "users",
                                    "role": "menuitem",
                                    "style": "display: none;",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "identity"}),
                                el("span", {"data-i18n": "tabs.users"}, "Users")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item manager-only",
                                    "data-auth-display": "flex",
                                    "data-profile-tab": "repositories",
                                    "role": "menuitem",
                                    "style": "display: none;",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "box"}),
                                el("span", {"data-i18n": "tabs.repositories"}, "Repositories")
                            ),
                            el("button", {
                                    "class": "nav-profile-menu-item manager-only",
                                    "data-auth-display": "flex",
                                    "data-profile-tab": "settings",
                                    "role": "menuitem",
                                    "style": "display: none;",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "settings"}),
                                el("span", {"data-i18n": "tabs.settings"}, "Settings")
                            ),
                            el("div", {"class": "nav-profile-menu-separator", "role": "separator"}),
                            el("button", {
                                    "class": "nav-profile-menu-item is-danger",
                                    "data-account-action": "logout",
                                    "id": "logout-btn",
                                    "role": "menuitem",
                                    "type": "button"
                                },
                                el("renop-icon", {"aria-hidden": "true", "name": "logout"}),
                                el("span", {"data-i18n": "nav.logout"}, "Log out")
                            )
                        )
                    )
                )
                ],
            })
        )
    );
}
