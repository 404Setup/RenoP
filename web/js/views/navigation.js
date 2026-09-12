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

/** Create the navigation component. */
export function renderNavigation() {
    return el("div", {"class": "top-nav"},
        el("div", {"class": "top-nav-left"},
            el("h1", {"class": "nav-title"},
                el("a", {"class": "nav-brand", "data-link": true, "href": "/"},
                    el("img", {"alt": "", "class": "nav-logo header-logo", "height": "28", "src": "/svg/logo.svg", "width": "28"}),
                    el("span", {}, "RenoP")
                )
            ),
            el("nav", {"aria-label": "Primary", "class": "nav-links"},
                el("a", {"data-i18n": "nav.home", "data-link": true, "href": "/"}, "Home"),
                el("a", {"data-i18n": "nav.docs", "data-link": true, "href": "/docs"}, "Docs"),
                el("a", {"data-i18n": "nav.api", "data-link": true, "href": "/api"}, "API"),
                el("a", {"data-i18n": "nav.download", "data-link": true, "href": "/download"}, "Download"),
                el("a", {"data-i18n": "nav.contributors", "data-link": true, "href": "/contributors"}, "Contributors"),
                el("a", {"data-i18n": "nav.community", "href": "https://discord.gg/ANjjKxpGX9", "rel": "noopener noreferrer", "target": "_blank"}, "Community"),
                el("a", {"data-i18n": "nav.donate", "href": "https://www.patreon.com/tranic", "rel": "noopener noreferrer", "target": "_blank"}, "Donate")
            )
        ),
        el("div", {"class": "top-nav-right"},
            el("div", {"class": "nav-controls-group"},
                el("button", {"aria-label": "Select language", "class": "lang-btn", "data-i18n-aria-label": "language.selectTitle", "data-i18n-title": "language.selectTitle", "id": "lang-btn", "title": "Select Language", "type": "button"},
                    svg("svg", {"aria-hidden": "true", "class": "lang-icon", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2", "viewBox": "0 0 24 24", "width": "16"},
                        svg("circle", {"cx": "12", "cy": "12", "r": "10"}),
                        svg("line", {"x1": "2", "x2": "22", "y1": "12", "y2": "12"}),
                        svg("path", {"d": "M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"})
                    ),
                    el("span", {"class": "current-lang-name", "id": "current-lang-name"}, "English"),
                    svg("svg", {"aria-hidden": "true", "class": "chevron-icon", "fill": "none", "height": "14", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2", "viewBox": "0 0 24 24", "width": "14"},
                        svg("polyline", {"points": "6 9 12 15 18 9"})
                    )
                ),
                el("button", {"aria-label": "Toggle theme", "class": "theme-btn", "data-i18n-aria-label": "nav.toggleTheme", "data-i18n-title": "nav.toggleTheme", "id": "theme-toggle", "title": "Toggle Theme", "type": "button"},
                    svg("svg", {"aria-hidden": "true", "class": "theme-icon sun-icon", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "16"},
                        svg("circle", {"cx": "12", "cy": "12", "r": "5"}),
                        svg("line", {"x1": "12", "x2": "12", "y1": "1", "y2": "3"}),
                        svg("line", {"x1": "12", "x2": "12", "y1": "21", "y2": "23"}),
                        svg("line", {"x1": "4.22", "x2": "5.64", "y1": "4.22", "y2": "5.64"}),
                        svg("line", {"x1": "18.36", "x2": "19.78", "y1": "18.36", "y2": "19.78"}),
                        svg("line", {"x1": "1", "x2": "3", "y1": "12", "y2": "12"}),
                        svg("line", {"x1": "21", "x2": "23", "y1": "12", "y2": "12"}),
                        svg("line", {"x1": "4.22", "x2": "5.64", "y1": "19.78", "y2": "18.36"}),
                        svg("line", {"x1": "18.36", "x2": "19.78", "y1": "5.64", "y2": "4.22"})
                    ),
                    svg("svg", {"aria-hidden": "true", "class": "theme-icon moon-icon", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.2", "viewBox": "0 0 24 24", "width": "16"},
                        svg("path", {"d": "M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"})
                    )
                ),
                el("div", {"aria-hidden": "true", "class": "divider"}),
                el("a", {"class": "nav-github-btn", "href": "https://github.com/404Setup/RenoP", "rel": "noopener noreferrer", "target": "_blank", "title": "GitHub"},
                    svg("svg", {"aria-hidden": "true", "fill": "currentColor", "height": "16", "viewBox": "0 0 24 24", "width": "16"},
                        svg("path", {"d": "M12 .5C5.73.5.5 5.73.5 12c0 5.1 3.29 9.42 7.86 10.95.58.1.79-.25.79-.56 0-.28-.01-1.02-.02-2-3.2.7-3.88-1.54-3.88-1.54-.53-1.34-1.3-1.7-1.3-1.7-1.06-.72.08-.71.08-.71 1.17.08 1.79 1.2 1.79 1.2 1.04 1.78 2.73 1.27 3.4.97.1-.75.41-1.27.74-1.56-2.55-.29-5.23-1.28-5.23-5.69 0-1.26.45-2.29 1.19-3.1-.12-.29-.52-1.46.11-3.05 0 0 .97-.31 3.18 1.18a11.1 11.1 0 0 1 5.8 0c2.2-1.49 3.17-1.18 3.17-1.18.63 1.59.23 2.76.11 3.05.74.81 1.19 1.84 1.19 3.1 0 4.42-2.69 5.39-5.25 5.68.42.36.79 1.08.79 2.18 0 1.57-.01 2.84-.01 3.23 0 .31.21.67.8.56A10.99 10.99 0 0 0 23.5 12C23.5 5.73 18.27.5 12 .5z"})
                    )
                )
            )
        )
    );
}
