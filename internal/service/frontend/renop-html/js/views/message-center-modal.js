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

/** Create the message center modal component. */
export function renderMessageCenterModal() {
    return el("div", {"class": "modal", "id": "message-center-modal", "style": "display: none;"},
        el("div", {"class": "modal-backdrop", "id": "message-center-backdrop"}),
        el("div", {"class": "modal-content message-center-modal-content"},
            el("button", {
                "aria-label": "Close message center",
                "class": "close-btn",
                "data-i18n-aria-label": "modal.close",
                "id": "message-center-close",
                "type": "button"
            }, "× "),
            el("div", {"class": "modal-header message-center-header"},
                el("div", {},
                    el("h2", {"class": "modal-title", "data-i18n": "messages.title"}, "Messages"),
                    el("p", {
                        "class": "modal-subtitle",
                        "data-i18n": "messages.subtitle"
                    }, "Notifications and requests for your account.")
                )
            ),
            el("div", {"class": "modal-body message-center-body", "id": "message-center-body"},
                el("div", {"class": "message-center-toolbar"},
                    el("button", {
                        "aria-busy": "false",
                        "class": "pill-btn pill-btn--danger pill-btn--sm message-clear-all",
                        "data-i18n": "messages.clearAll",
                        "id": "message-clear-all",
                        "type": "button"
                    }, "Clear notifications "),
                    el("button", {
                        "class": "pill-btn pill-btn--soft pill-btn--sm",
                        "data-i18n": "messages.markAllRead",
                        "id": "message-mark-all-read",
                        "type": "button"
                    }, "Mark all read ")
                ),
                el("div", {
                    "class": "message-center-state",
                    "data-i18n": "messages.loading",
                    "id": "message-center-loading"
                }, "Loading messages... "),
                el("div", {
                    "class": "message-center-state",
                    "data-i18n": "messages.empty",
                    "hidden": true,
                    "id": "message-center-empty"
                }, "No messages yet. "),
                el("div", {"class": "message-center-list", "id": "message-center-list"}),
                el("button", {
                    "aria-busy": "false",
                    "aria-disabled": "true",
                    "class": "pill-btn pill-btn--soft message-load-more",
                    "data-i18n": "messages.loadMore",
                    "data-next-cursor": "",
                    "disabled": true,
                    "hidden": true,
                    "id": "message-load-more",
                    "type": "button"
                }, "Load more ")
            )
        )
    );
}
