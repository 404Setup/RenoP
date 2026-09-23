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

/** Create the message compose modal component. */
export function renderMessageComposeModal() {
    return el("div", {"class": "modal", "id": "message-compose-modal", "style": "display: none;"},
        el("div", {"class": "modal-backdrop", "id": "message-compose-backdrop"}),
        el("div", {"class": "modal-content message-compose-modal-content"},
            el("button", {
                "aria-label": "Close notification composer",
                "class": "close-btn",
                "data-i18n-aria-label": "modal.close",
                "id": "message-compose-close",
                "type": "button"
            }, "× "),
            el("div", {"class": "modal-header message-compose-heading"},
                el("span", {"aria-hidden": "true", "class": "message-compose-heading-icon"},
                    el("renop-icon", {"name": "send"})
                ),
                el("div", {"class": "message-compose-heading-copy"},
                    el("h2", {"class": "modal-title", "data-i18n": "messages.composeTitle"}, "New notification"),
                    el("p", {
                        "class": "modal-subtitle",
                        "data-i18n": "messages.composeHint"
                    }, "Send a plain-text notification to selected users or everyone.")
                )
            ),
            el("form", {"action": "javascript:void(0);", "class": "message-compose-form", "id": "message-compose-form"},
                el("div", {"class": "modal-body message-compose-region", "id": "message-compose-region"},
                    el("div", {"class": "message-compose-audience"},
                        el("div", {"class": "message-compose-field message-compose-recipient-field"},
                            el("label", {
                                "data-i18n": "messages.recipients",
                                "for": "message-compose-recipients"
                            }, "Recipients"),
                            el("span", {"class": "message-recipient-input-wrap"},
                                el("input", {
                                    "aria-autocomplete": "list",
                                    "aria-controls": "message-recipient-suggestions",
                                    "aria-expanded": "false",
                                    "autocomplete": "off",
                                    "data-i18n-placeholder": "messages.recipientsPlaceholder",
                                    "id": "message-compose-recipients",
                                    "maxlength": "4096",
                                    "placeholder": "alice, bob",
                                    "role": "combobox",
                                    "type": "text"
                                }),
                                el("span", {
                                    "class": "custom-select-dropdown message-recipient-suggestions",
                                    "id": "message-recipient-suggestions",
                                    "role": "listbox"
                                })
                            )
                        ),
                        el("div", {"class": "message-compose-broadcast"},
                            el("span", {
                                "data-i18n": "messages.sendAll",
                                "id": "message-compose-all-label"
                            }, "Send to all users"),
                            el("span", {"class": "message-compose-all-switch", "id": "message-compose-all"})
                        ),
                        el("div", {
                                "class": "message-compose-field",
                                "id": "message-compose-session-field",
                                "style": "display: none;",
                                "inert": true
                            },
                            el("span", {
                                "data-i18n": "messages.sessionTarget",
                                "id": "message-compose-session-label"
                            }, "Delivery target"),
                            el("span", {"id": "message-compose-session"}),
                            el("span", {"class": "form-hint", "role": "status", "id": "message-compose-session-status"})
                        )
                    ),
                    el("div", {"class": "message-compose-meta"},
                        el("label", {"class": "message-compose-field"},
                            el("span", {"data-i18n": "messages.subject"}, "Subject"),
                            el("input", {
                                "id": "message-compose-title",
                                "maxlength": "240",
                                "required": true,
                                "type": "text"
                            })
                        ),
                        el("div", {"class": "message-compose-field"},
                            el("span", {"data-i18n": "messages.severity"}, "Severity"),
                            el("span", {"class": "message-compose-severity", "id": "message-compose-severity"})
                        )
                    ),
                    el("label", {"class": "message-compose-field message-compose-body-field"},
                        el("span", {"data-i18n": "messages.body"}, "Message"),
                        el("textarea", {
                            "id": "message-compose-body",
                            "maxlength": "8000",
                            "required": true,
                            "rows": "4"
                        })
                    )
                ),
                el("div", {"class": "modal-footer message-compose-actions"},
                    el("button", {
                        "class": "pill-btn pill-btn--soft",
                        "data-i18n": "common.cancel",
                        "id": "message-compose-cancel",
                        "type": "button"
                    }, "Cancel "),
                    el("button", {
                            "class": "pill-btn pill-btn--primary",
                            "id": "message-compose-submit",
                            "type": "submit"
                        },
                        el("span", {"class": "message-compose-submit-label", "data-i18n": "messages.send"}, "Send")
                    )
                )
            )
        )
    );
}
