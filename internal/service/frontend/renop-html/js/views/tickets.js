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

/** Create the tickets component. */
export function renderTickets() {
    return el("div", {"class": "tab-content", "id": "tab-content-tickets", "style": "display: none;"},
        el("section", {"aria-labelledby": "review-page-title", "class": "review-page"},
            el("header", {"class": "review-page-header"},
                el("button", {"class": "super-team-back", "id": "review-home", "type": "button"},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.backPrevious"}, "Back to previous page")
                ),
                el("div", {},
                    el("h2", {"data-i18n": "review.title", "id": "review-page-title"}, "Reviews"),
                    el("p", {"data-i18n": "review.subtitle"}, "Review ownership transfers and moderated publications independently from notifications.")
                )
            ),
            el("div", {"class": "review-center", "id": "review-page-content"})
        )
    );
}
