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

/** Create the recovery component. */
export function renderRecovery() {
    return el("section", {
            "aria-labelledby": "recovery-title",
            "class": "tab-content",
            "id": "tab-content-recovery",
            "style": "display: none;"
        },
        el("div", {"class": "account-page"},
            el("header", {"class": "account-page-heading"},
                el("renop-icon", {"aria-hidden": "true", "class": "account-page-symbol", "name": "fileKey"}),
                el("h1", {"data-i18n": "login.recoveryTitle", "id": "recovery-title"}, "Recover account"),
                el("p", {"data-i18n": "login.recoverySubtitle"}, "Enter your primary email, four unused recovery codes, and a new password. A previous primary email can restore the account within 14 days of its replacement."),
                el("a", {"class": "account-page-back", "href": "/account/login", "id": "recovery-back-login"},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.signIn"}, "Sign in")
                )
            ),
            el("form", {"class": "account-page-form", "id": "account-recovery-form"},
                el("div", {"class": "account-field"},
                    el("label", {
                        "data-i18n": "login.recoveryIdentifier",
                        "for": "recovery-identifier"
                    }, "Primary email"),
                    el("input", {
                        "autocomplete": "email",
                        "data-i18n-placeholder": "login.recoveryIdentifierPlaceholder",
                        "id": "recovery-identifier",
                        "maxlength": "254",
                        "required": true,
                        "type": "email"
                    })
                ),
                el("fieldset", {"class": "account-recovery-codes"},
                    el("legend", {"data-i18n": "login.recoveryCodesPrompt"}, "Four distinct recovery codes"),
                    el("div", {"class": "account-field"},
                        el("label", {"for": "recovery-code-1"}, "Recovery code 1"),
                        el("input", {
                            "autocapitalize": "characters",
                            "autocomplete": "off",
                            "id": "recovery-code-1",
                            "maxlength": "64",
                            "name": "recovery-code",
                            "required": true,
                            "spellcheck": "false",
                            "type": "text"
                        })
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"for": "recovery-code-2"}, "Recovery code 2"),
                        el("input", {
                            "autocapitalize": "characters",
                            "autocomplete": "off",
                            "id": "recovery-code-2",
                            "maxlength": "64",
                            "name": "recovery-code",
                            "required": true,
                            "spellcheck": "false",
                            "type": "text"
                        })
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"for": "recovery-code-3"}, "Recovery code 3"),
                        el("input", {
                            "autocapitalize": "characters",
                            "autocomplete": "off",
                            "id": "recovery-code-3",
                            "maxlength": "64",
                            "name": "recovery-code",
                            "required": true,
                            "spellcheck": "false",
                            "type": "text"
                        })
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"for": "recovery-code-4"}, "Recovery code 4"),
                        el("input", {
                            "autocapitalize": "characters",
                            "autocomplete": "off",
                            "id": "recovery-code-4",
                            "maxlength": "64",
                            "name": "recovery-code",
                            "required": true,
                            "spellcheck": "false",
                            "type": "text"
                        })
                    )
                ),
                el("div", {"class": "account-field"},
                    el("label", {"data-i18n": "profile.newPasswordLabel", "for": "recovery-password"}, "New password"),
                    el("input", {
                        "autocomplete": "new-password",
                        "id": "recovery-password",
                        "maxlength": "72",
                        "required": true,
                        "type": "password"
                    })
                ),
                el("div", {"class": "account-field"},
                    el("label", {
                        "data-i18n": "login.confirmNewPassword",
                        "for": "recovery-password-confirmation"
                    }, "Confirm new password"),
                    el("input", {
                        "autocomplete": "new-password",
                        "id": "recovery-password-confirmation",
                        "maxlength": "72",
                        "required": true,
                        "type": "password"
                    })
                ),
                el("p", {"class": "account-form-error", "id": "recovery-error", "role": "alert"}),
                el("button", {
                    "class": "account-submit",
                    "data-i18n": "login.resetPassword",
                    "type": "submit"
                }, "Reset password"),
                el("div", {"class": "account-form-links"},
                    el("a", {
                        "class": "account-form-link",
                        "data-i18n": "login.forgotPassword",
                        "data-password-reset-link": true,
                        "hidden": true,
                        "href": "/account/forgot-password"
                    }, "Forgot password?")
                )
            )
        )
    );
}
