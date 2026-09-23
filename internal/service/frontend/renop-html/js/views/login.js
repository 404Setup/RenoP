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

/** Create the login component. */
export function renderLogin() {
    return el("section", {
            "aria-labelledby": "login-title",
            "class": "tab-content",
            "id": "tab-content-login",
            "style": "display: none;"
        },
        el("div", {"class": "account-page"},
            el("header", {"class": "account-page-heading"},
                el("renop-icon", {"aria-hidden": "true", "class": "account-page-symbol", "name": "identity"}),
                el("h1", {"data-i18n": "nav.signIn", "id": "login-title"}, "Sign in"),
                el("p", {"data-i18n": "login.pageDescription"}, "Sign in to manage your account and access your repositories."),
                el("a", {"class": "account-page-back", "href": "/", "data-back": ""},
                    el("renop-icon", {"aria-hidden": "true", "name": "chevronLeft"}),
                    el("span", {"data-i18n": "nav.backPrevious"}, "Back to previous page")
                ),
                el("div", {"class": "account-legal-slot", "id": "login-legal-consent-slot"})
            ),
            el("form", {"class": "account-page-form", "id": "login-form"},
                el("div", {"class": "account-field"},
                    el("label", {"data-i18n": "login.usernameLabel", "for": "username"}, "Username or email"),
                    el("input", {
                        "autocomplete": "username",
                        "data-i18n-placeholder": "login.usernamePlaceholder",
                        "id": "username",
                        "maxlength": "254",
                        "placeholder": "Enter username or private email",
                        "required": true,
                        "type": "text"
                    })
                ),
                el("div", {"class": "account-field"},
                    el("label", {"data-i18n": "login.passwordLabel", "for": "password"}, "Password"),
                    el("input", {
                        "autocomplete": "current-password",
                        "data-i18n-placeholder": "login.passwordPlaceholder",
                        "id": "password",
                        "maxlength": "72",
                        "placeholder": "Enter password",
                        "required": true,
                        "type": "password"
                    })
                ),
                el("div", {
                    "class": "account-form-error",
                    "id": "login-error",
                    "role": "alert",
                    "style": "display: none;"
                }),
                el("div", {"class": "account-form-links"},
                    el("a", {
                        "class": "account-form-link",
                        "data-i18n": "login.forgotPassword",
                        "data-password-reset-link": true,
                        "hidden": true,
                        "href": "/account/forgot-password"
                    }, "Forgot password?"),
                    el("a", {
                        "class": "account-form-link",
                        "data-i18n": "login.recoveryTitle",
                        "href": "/account/recovery",
                        "id": "btn-recover-account"
                    }, "Recover account"),
                    el("a", {
                        "class": "account-form-link",
                        "data-i18n": "registration.title",
                        "data-registration-link": true,
                        "hidden": true,
                        "href": "/account/register"
                    }, "Create account")
                ),
                el("button", {"class": "account-submit", "data-i18n": "login.submitBtn", "type": "submit"}, " Login "),
                el("div", {"class": "account-providers"},
                    el("div", {"class": "account-provider-divider"},
                        el("span", {"data-i18n": "login.or"}, "or")
                    ),
                    el("button", {"class": "account-provider", "id": "btn-fido-login", "type": "button"},
                        svg("svg", {
                                "aria-hidden": "true",
                                "fill": "none",
                                "stroke": "currentColor",
                                "stroke-linecap": "round",
                                "stroke-linejoin": "round",
                                "stroke-width": "2",
                                "viewBox": "0 0 24 24"
                            },
                            svg("circle", {"cx": "8", "cy": "15", "r": "4"}),
                            svg("path", {"d": "m11 12 8-8M16 7l2 2M14 9l2 2"})
                        ),
                        el("span", {"data-i18n": "login.fidoLogin"}, "Continue with Passkey")
                    ),
                    el("div", {"class": "account-providers", "hidden": true, "id": "oauth-login-providers"})
                )
            ),
            el("form", {
                    "aria-labelledby": "mfa-login-title",
                    "class": "account-page-form",
                    "hidden": true,
                    "id": "mfa-login-form"
                },
                el("h2", {"data-i18n": "mfa.title", "id": "mfa-login-title"}, "Two-step verification"),
                el("p", {"data-i18n": "mfa.loginHint"}, "Complete verification to finish signing in."),
                el("div", {"class": "account-field", "id": "mfa-login-totp"},
                    el("label", {"data-i18n": "mfa.code", "for": "mfa-login-code"}, "Six-digit code"),
                    el("input", {
                        "aria-describedby": "mfa-login-error",
                        "autocomplete": "one-time-code",
                        "id": "mfa-login-code",
                        "inputmode": "numeric",
                        "maxlength": "6",
                        "minlength": "6",
                        "pattern": "[0-9]{6}",
                        "required": true,
                        "type": "text"
                    })
                ),
                el("p", {"class": "account-form-error", "hidden": true, "id": "mfa-login-error", "role": "alert"}),
                el("button", {
                    "class": "account-submit",
                    "data-i18n": "mfa.verify",
                    "id": "mfa-login-submit",
                    "type": "submit"
                }, "Verify "),
                el("button", {
                    "class": "account-provider",
                    "data-i18n": "mfa.usePasskey",
                    "id": "mfa-login-passkey",
                    "type": "button"
                }, " Verify with Passkey "),
                el("div", {"class": "account-form-links"},
                    el("button", {
                        "class": "account-form-link",
                        "data-i18n": "mfa.restart",
                        "id": "mfa-login-restart",
                        "type": "button"
                    }, " Start again "),
                    el("a", {
                        "class": "account-form-link",
                        "data-i18n": "login.recoveryTitle",
                        "href": "/account/recovery"
                    }, "Recover account")
                )
            )
        )
    );
}
