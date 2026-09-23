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

/** Create the profile fido modal component. */
export function renderProfileFidoModal() {
    return el("div", {"class": "modal", "id": "profile-fido-modal", "style": "display: none;"},
        el("div", {"class": "modal-backdrop", "id": "profile-fido-backdrop"}),
        el("div", {"class": "modal-content", "style": "max-width: 520px;"},
            el("button", {
                "aria-label": "Close modal",
                "class": "close-btn",
                "data-i18n-aria-label": "modal.close",
                "id": "close-profile-fido-modal"
            }, "× "),
            el("div", {"class": "modal-header"},
                el("h2", {
                    "class": "modal-title",
                    "data-i18n": "profile.fidoTitle",
                    "id": "profile-fido-modal-title"
                }, "Passkeys")
            ),
            el("div", {"class": "modal-body"},
                el("div", {"class": "fido-device-list", "id": "profile-fido-list", "style": "margin-bottom: 1rem;"}),
                el("button", {
                    "class": "pill-btn pill-btn--primary profile-action-btn",
                    "data-i18n": "profile.addFidoBtn",
                    "id": "btn-add-fido-device",
                    "type": "button"
                }, "Add Passkey ")
            )
        )
    );
}
