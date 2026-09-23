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

/** Create the footer component. */
export function renderFooter() {
    return el("footer", {},
        el("div", {"id": "footer-links"},
            el("a", {
                "href": "https://github.com/404Setup/RenoP",
                "rel": "noopener noreferrer",
                "target": "_blank"
            }, "GitHub"),
            el("span", {"class": "separator"}, "·"),
            el("a", {"data-i18n": "nav.docs", "data-link": true, "href": "/docs"}, "Docs"),
            el("span", {"class": "separator"}, "·"),
            el("a", {"data-i18n": "nav.api", "data-link": true, "href": "/api"}, "API"),
            el("span", {"class": "separator"}, "·"),
            el("a", {"data-i18n": "nav.download", "data-link": true, "href": "/download"}, "Download"),
            el("span", {"class": "separator"}, "·"),
            el("a", {"data-i18n": "nav.contributors", "data-link": true, "href": "/contributors"}, "Contributors"),
            el("span", {"class": "separator"}, "·"),
            el("a", {
                "data-i18n": "nav.community",
                "href": "https://discord.gg/ANjjKxpGX9",
                "rel": "noopener noreferrer",
                "target": "_blank"
            }, "Community"),
            el("span", {"class": "separator"}, "·"),
            el("a", {
                "data-i18n": "nav.donate",
                "href": "https://www.patreon.com/tranic",
                "rel": "noopener noreferrer",
                "target": "_blank"
            }, "Donate")
        ),
        el("div", {"id": "footer-copyright"})
    );
}
