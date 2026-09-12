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

/** Create the maven domains component. */
export function renderMavenDomains() {
    return el("div", {"class": "tab-content", "id": "tab-content-maven-domains", "style": "display: none;"},
        el("section", {"aria-labelledby": "maven-domain-page-title", "class": "maven-domain-page"},
            el("header", {"class": "maven-domain-page-header"},
                el("button", {"class": "maven-back-btn", "id": "maven-domain-home", "type": "button"},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.backPrevious"}, "Back to previous page")
                ),
                el("div", {},
                    el("span", {"class": "maven-kicker"}, "Maven"),
                    el("h2", {"data-i18n": "maven.domainCenterTitle", "id": "maven-domain-page-title"}, "Global Maven domains"),
                    el("p", {"data-i18n": "maven.domainCenterSubtitle"}, "Configure a publishing domain once and use its team across every Maven repository.")
                )
            ),
            el("div", {"class": "maven-domain-center", "id": "maven-domain-page-content"})
        )
    );
}
