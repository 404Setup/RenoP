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

/** Create the settings component. */
export function renderSettings() {
    return el("div", {"class": "tab-content", "id": "tab-content-settings", "style": "display: none;"},
        el("div", {"class": "settings-header"},
            el("div", {},
                el("h2", {"data-i18n": "settings.title"}, "Settings"),
                el("p", {"data-i18n": "settings.subtitle"}, "Manage system settings for this instance."),
                el("p", {"class": "settings-propagation", "data-i18n": "settings.propagationNotice"}, "Listener, database, and cache backend changes require a restart.")
            )
        ),
        el("div", {"class": "settings-workspace"},
            el("aside", {"class": "settings-sidebar"},
                el("h3", {"data-i18n": "settings.sections", "id": "settings-nav-title"}, "Settings sections"),
                el("nav", {"aria-labelledby": "settings-nav-title", "id": "settings-nav"})
            ),
            el("div", {"class": "settings-main"},
                el("div", {"id": "settings-page-picker"}),
                el("nav", {"id": "settings-subnav", "data-i18n-aria-label": "settings.sections", "aria-label": "Settings sections"}),
                el("div", {"class": "settings-page-header"},
                    el("div", {"class": "settings-page-heading"},
                        el("h3", {"id": "settings-page-title", "tabindex": "-1"}),
                        el("span", {"aria-live": "polite", "id": "settings-draft-status", "role": "status"})
                    ),
                    el("div", {"class": "settings-actions"},
                        el("button", {"class": "pill-btn pill-btn--soft", "data-i18n-title": "settings.restartBtnTitle", "id": "settings-restart-btn", "title": "Restart the Renop service", "type": "button"},
                            svg("svg", {"aria-hidden": "true", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "16"},
                                svg("polyline", {"points": "23 4 23 10 17 10"}),
                                svg("path", {"d": "M20.49 15a9 9 0 1 1-.38-3.72"})
                            ),
                            el("span", {"data-i18n": "settings.restartBtn"}, "Restart Renop")
                        ),
                        el("button", {"class": "pill-btn pill-btn--soft", "data-i18n": "settings.discard", "disabled": true, "id": "settings-reset-btn", "type": "button"}, "Discard changes "),
                        el("button", {"class": "settings-save-btn", "data-i18n-title": "settings.saveBtnTitle", "disabled": true, "id": "settings-save-btn", "title": "Save changes and reload configuration", "type": "button"},
                            svg("svg", {"aria-hidden": "true", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "16"},
                                svg("path", {"d": "M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"}),
                                svg("polyline", {"points": "17 21 17 13 7 13 7 21"}),
                                svg("polyline", {"points": "7 3 7 8 15 8"})
                            ),
                            el("span", {"data-i18n": "settings.saveBtn"}, "Update and reload")
                        )
                    )
                ),
                el("div", {"aria-labelledby": "settings-page-title", "class": "cfg-page-content", "id": "settings-form-container", "role": "region"}),
                el("nav", {"aria-label": "Settings pages", "class": "renop-pagination", "data-i18n-aria-label": "settings.sections", "id": "settings-pagination"})
            )
        )
    );
}
