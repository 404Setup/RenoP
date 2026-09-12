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

/** Create the language component. */
export function renderLanguage() {
    return el("div", {"class": "modal", "id": "language-modal", "style": "display: none;"},
        el("div", {"class": "modal-backdrop", "id": "language-backdrop"}),
        el("div", {"class": "modal-content language-modal-content", "style": "max-width: 580px;"},
            el("button", {"aria-label": "Close modal", "class": "close-btn", "data-i18n-aria-label": "modal.close", "id": "btn-close-language-modal"}, "× "),
            el("div", {"class": "modal-header"},
                el("h3", {"class": "modal-title", "data-i18n": "language.modalTitle"}, "Select Language"),
                el("p", {"class": "modal-subtitle", "data-i18n": "language.modalSubtitle", "style": "margin-top: 4px; font-size: 0.875rem; opacity: 0.75;"}, "Choose your preferred interface language")
            ),
            el("div", {"aria-label": "Loading", "class": "language-load-progress", "data-i18n-aria-label": "common.loading", "hidden": true, "id": "language-load-progress", "role": "progressbar"},
                el("span", {})
            ),
            el("div", {"class": "modal-body"},
                el("div", {"class": "language-grid", "id": "language-grid"})
            )
        )
    );
}
