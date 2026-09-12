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

/** Create the super teams component. */
export function renderSuperTeams() {
    return el("div", {"class": "tab-content", "id": "tab-content-super-teams", "style": "display: none;"},
        el("section", {"aria-labelledby": "super-team-page-title", "class": "super-team-page"},
            el("header", {"class": "super-team-page-header"},
                el("button", {"class": "super-team-back", "id": "super-team-home", "type": "button"},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.backPrevious"}, "Back to previous page")
                ),
                el("div", {},
                    el("span", {"class": "super-team-kicker"}, "RenoP"),
                    el("h2", {"data-i18n": "superTeam.title", "id": "super-team-page-title"}, "Global teams"),
                    el("p", {"data-i18n": "superTeam.subtitle"}, "Manage shared publishing identities and cross-engine namespaces.")
                )
            ),
            el("div", {"class": "super-team-center", "id": "super-team-page-content"})
        )
    );
}
