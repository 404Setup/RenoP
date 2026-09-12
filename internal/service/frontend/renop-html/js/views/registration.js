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

/** Create the registration component. */
export function renderRegistration() {
    return el("section", {"aria-labelledby": "registration-title", "class": "tab-content", "id": "tab-content-registration", "style": "display: none;"},
        el("div", {"class": "account-page"},
            el("header", {"class": "account-page-heading"},
                el("renop-icon", {"aria-hidden": "true", "class": "account-page-symbol", "name": "user"}),
                el("h1", {"data-i18n": "registration.title", "id": "registration-title"}, "Create account"),
                el("p", {"data-i18n": "registration.description"}, "Choose your account details and password. Verify your email when required."),
                el("a", {"class": "account-page-back", "href": "/account/login", "id": "registration-back-login"},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.signIn"}, "Sign in")
                ),
                el("div", {"class": "account-legal-slot", "id": "registration-legal-consent-slot"})
            ),
            el("form", {"class": "account-page-form", "id": "registration-form"},
                el("p", {"class": "account-form-error", "id": "registration-availability", "role": "status"}),
                el("fieldset", {"class": "account-reset-fields", "disabled": true, "id": "registration-fields"},
                    el("div", {"class": "account-providers", "hidden": true, "id": "oauth-register-providers"}),
                    el("div", {"hidden": true, "id": "registration-provider-info"},
                        el("p", {"id": "registration-provider-hint", "role": "status"}),
                        el("label", {"class": "account-check"},
                            el("input", {"id": "registration-import", "type": "checkbox"}),
                            el("span", {"data-i18n": "oauth.importProfile"}, "Use the provider username, nickname, and avatar")
                        )
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"data-i18n": "profile.usernameLabel", "for": "registration-username"}, "Username"),
                        el("input", {"aria-describedby": "registration-username-hint", "autocomplete": "username", "id": "registration-username", "maxlength": "18", "minlength": "4", "pattern": "[A-Za-z0-9_]{4,18}", "required": true, "type": "text"}),
                        el("p", {"data-i18n": "registration.usernameHint", "id": "registration-username-hint"}, "4–18 letters, numbers, or underscores.")
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"data-i18n": "registration.nickname", "for": "registration-nickname"}, "Nickname (optional)"),
                        el("input", {"autocomplete": "nickname", "id": "registration-nickname", "maxlength": "72", "type": "text"})
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"data-i18n": "profile.privateEmailLabel", "for": "registration-email"}, "Private email"),
                        el("input", {"autocomplete": "email", "id": "registration-email", "maxlength": "254", "type": "email"})
                    ),
                    el("button", {"class": "account-provider", "data-i18n": "login.sendEmailCode", "hidden": true, "id": "registration-send", "type": "button"}, "Send verification code "),
                    el("div", {"aria-live": "polite", "class": "account-mail-status", "id": "registration-delivery", "role": "status"}),
                    el("div", {"class": "account-field", "hidden": true},
                        el("label", {"data-i18n": "login.emailCode", "for": "registration-code"}, "Verification code"),
                        el("input", {"autocomplete": "one-time-code", "id": "registration-code", "inputmode": "numeric", "maxlength": "8", "pattern": "[0-9]{8}", "type": "text"})
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"data-i18n": "profile.newPasswordLabel", "for": "registration-password"}, "New password"),
                        el("input", {"autocomplete": "new-password", "id": "registration-password", "maxlength": "72", "required": true, "type": "password"})
                    ),
                    el("div", {"class": "account-field"},
                        el("label", {"data-i18n": "login.confirmNewPassword", "for": "registration-confirmation"}, "Confirm new password"),
                        el("input", {"autocomplete": "new-password", "id": "registration-confirmation", "maxlength": "72", "required": true, "type": "password"})
                    ),
                    el("p", {"class": "account-form-error", "id": "registration-error", "role": "alert"}),
                    el("button", {"class": "account-submit", "data-i18n": "registration.title", "type": "submit"}, "Create account ")
                )
            )
        )
    );
}
