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

/** Create the password recovery component. */
export function renderPasswordRecovery() {
    return el("section", {
            "aria-labelledby": "password-reset-title",
            "class": "tab-content",
            "id": "tab-content-password-recovery",
            "style": "display: none;"
        },
        el("div", {"class": "account-page"},
            el("header", {"class": "account-page-heading"},
                el("renop-icon", {"aria-hidden": "true", "class": "account-page-symbol", "name": "fileKey"}),
                el("h1", {"data-i18n": "login.forgotPassword", "id": "password-reset-title"}, "Forgot password?"),
                el("p", {"data-i18n": "login.emailResetDescription"}, "Verify your primary email with an eight-digit code, then set a new password. Secondary emails cannot recover an account. The code expires in 10 minutes."),
                el("a", {"class": "account-page-back", "href": "/account/login", "id": "password-reset-back-login"},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.signIn"}, "Sign in")
                )
            ),
            el("form", {"class": "account-page-form", "id": "password-reset-form"},
                el("p", {"class": "account-form-error", "id": "password-reset-availability", "role": "status"}),
                el("fieldset", {"class": "account-reset-fields", "disabled": true, "id": "password-reset-fields"},
                    el("div", {"class": "account-field"},
                        el("label", {
                            "data-i18n": "profile.privateEmailLabel",
                            "for": "password-reset-email"
                        }, "Private email"),
                        el("input", {
                            "autocomplete": "email",
                            "id": "password-reset-email",
                            "maxlength": "254",
                            "required": true,
                            "type": "email"
                        })
                    ),
                    el("button", {
                        "class": "account-provider",
                        "data-i18n": "login.sendEmailCode",
                        "id": "password-reset-send",
                        "type": "button"
                    }, "Send verification code "),
                    el("div", {
                        "aria-live": "polite",
                        "class": "account-mail-status",
                        "id": "password-reset-delivery",
                        "role": "status"
                    }),
                    el("button", {
                        "class": "account-provider",
                        "data-i18n": "mail.refreshStatus",
                        "hidden": true,
                        "id": "password-reset-refresh",
                        "type": "button"
                    }, "Refresh "),
                    el("div", {"class": "account-field"},
                        el("label", {
                            "data-i18n": "login.emailCode",
                            "for": "password-reset-code"
                        }, "Verification code"),
                        el("input", {
                            "autocomplete": "one-time-code",
                            "id": "password-reset-code",
                            "inputmode": "numeric",
                            "maxlength": "8",
                            "pattern": "[0-9]{8}",
                            "required": true,
                            "type": "text"
                        })
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {
                            "data-i18n": "profile.newPasswordLabel",
                            "for": "password-reset-password"
                        }, "New password"),
                        el("input", {
                            "autocomplete": "new-password",
                            "id": "password-reset-password",
                            "maxlength": "72",
                            "required": true,
                            "type": "password"
                        })
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {
                            "data-i18n": "login.confirmNewPassword",
                            "for": "password-reset-confirmation"
                        }, "Confirm new password"),
                        el("input", {
                            "autocomplete": "new-password",
                            "id": "password-reset-confirmation",
                            "maxlength": "72",
                            "required": true,
                            "type": "password"
                        })
                    ),
                    el("p", {"class": "account-form-error", "id": "password-reset-error", "role": "alert"}),
                    el("button", {
                        "class": "account-submit",
                        "data-i18n": "login.resetPassword",
                        "type": "submit"
                    }, "Reset password ")
                ),
                el("div", {"class": "account-form-links"},
                    el("a", {
                        "class": "account-form-link",
                        "data-i18n": "login.recoveryTitle",
                        "href": "/account/recovery",
                        "id": "password-reset-recover-account"
                    }, "Recover account")
                )
            )
        )
    );
}
