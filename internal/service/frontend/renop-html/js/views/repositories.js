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

/** Create the repositories component. */
export function renderRepositories() {
    return el("div", {"class": "tab-content", "id": "tab-content-repositories", "style": "display: none;"},
        el("div", {"class": "settings-header repos-header"},
            el("div", {"class": "repos-header-text"},
                el("h2", {"data-i18n": "repos.title"}, "Repositories"),
                el("p", {"data-i18n": "repos.subtitle"}, "Manage package repositories and their protocol-specific configuration.")
            ),
            el("button", {"class": "settings-save-btn", "data-i18n-title": "repos.addRepoTitle", "id": "btn-add-repository", "title": "Add a package repository", "type": "button"},
                svg("svg", {"aria-hidden": "true", "fill": "none", "height": "16", "stroke": "currentColor", "stroke-linecap": "round", "stroke-linejoin": "round", "stroke-width": "2.5", "viewBox": "0 0 24 24", "width": "16"},
                    svg("line", {"x1": "12", "x2": "12", "y1": "5", "y2": "19"}),
                    svg("line", {"x1": "5", "x2": "19", "y1": "12", "y2": "12"})
                ),
                el("span", {"data-i18n": "repos.addRepoBtn"}, "Add Repository")
            )
        ),
        el("div", {"class": "cfg-page-content", "id": "repositories-container"})
    );
}
