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

/** Create the profile component. */
export function renderProfile() {
    return el("div", {"class": "tab-content", "id": "tab-content-profile", "style": "display: none;"},
        el("div", {"class": "profile-route-view", "id": "profile-public-view"}),
        el("div", {"class": "profile-route-view", "hidden": true, "id": "profile-edit-view"},
            el("div", {"class": "profile-hero"},
                el("div", {"class": "profile-avatar-ring"},
                    el("div", {"class": "profile-avatar-inner", "id": "profile-avatar-initials"}, "?")
                ),
                el("div", {"class": "profile-hero-text"},
                    el("h2", {
                        "class": "profile-hero-name",
                        "data-i18n": "profile.title",
                        "id": "profile-display-name"
                    }, "My Profile"),
                    el("p", {
                        "class": "profile-hero-sub",
                        "data-i18n": "profile.subtitle"
                    }, "Manage your account settings and access credentials")
                )
            ),
            el("div", {"class": "profile-settings-card"},
                el("details", {
                        "class": "profile-settings-section profile-account-security-section profile-collapsible-card",
                        "id": "profile-account-security-section"
                    },
                    el("summary", {"class": "profile-section-card-header profile-collapsible-summary"},
                        el("div", {
                                "aria-hidden": "true",
                                "class": "profile-section-icon profile-section-icon--account-security"
                            },
                            svg("svg", {
                                    "fill": "none",
                                    "stroke": "currentColor",
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2",
                                    "viewBox": "0 0 24 24"
                                },
                                svg("path", {"d": "M12 3 4 6v5c0 5 3.4 8.7 8 10 4.6-1.3 8-5 8-10V6l-8-3Z"}),
                                svg("path", {"d": "m9 12 2 2 4-4"})
                            )
                        ),
                        el("div", {"class": "profile-section-meta"},
                            el("h3", {
                                "class": "profile-section-title",
                                "data-i18n": "profile.accountSecurityTitle"
                            }, "Account Security"),
                            el("p", {
                                "class": "profile-section-desc",
                                "data-i18n": "profile.accountSecurityDesc"
                            }, "Manage your private login email, password login, and one-time recovery codes.")
                        ),
                        el("renop-icon", {
                            "aria-hidden": "true",
                            "class": "profile-collapse-chevron",
                            "name": "chevronDown"
                        })
                    ),
                    el("div", {"class": "profile-collapsible-content", "hidden": true},
                        el("div", {"class": "profile-section-body profile-account-security-body"},
                            el("p", {
                                "class": "profile-security-hint",
                                "id": "profile-security-hold",
                                "role": "status",
                                "hidden": true
                            }),
                            el("div", {"id": "profile-previous-primary-emails"}),
                            el("form", {
                                    "action": "javascript:void(0);",
                                    "class": "profile-security-password-form",
                                    "id": "profile-password-form"
                                },
                                el("input", {
                                    "autocomplete": "username",
                                    "id": "profile-username-hidden",
                                    "name": "username",
                                    "readonly": true,
                                    "style": "display: none;",
                                    "type": "text"
                                }),
                                el("strong", {"data-i18n": "profile.changePasswordTitle"}, "Change Password"),
                                el("label", {
                                    "data-i18n": "profile.newPasswordLabel",
                                    "for": "profile-new-password"
                                }, "New Password"),
                                el("div", {"class": "profile-security-inline"},
                                    el("input", {
                                        "autocomplete": "new-password",
                                        "class": "profile-input",
                                        "data-i18n-placeholder": "profile.newPasswordPlaceholder",
                                        "id": "profile-new-password",
                                        "placeholder": "Enter new password",
                                        "type": "password"
                                    }),
                                    el("button", {
                                        "class": "pill-btn pill-btn--primary profile-action-btn",
                                        "data-i18n": "profile.updatePasswordBtn",
                                        "id": "btn-update-password",
                                        "type": "submit"
                                    }, "Update Password ")
                                ),
                                el("p", {
                                    "class": "profile-security-hint",
                                    "data-i18n": "profile.changePasswordDesc"
                                }, "Update your account password below.")
                            ),
                            el("form", {
                                    "action": "javascript:void(0);",
                                    "class": "profile-security-email-form",
                                    "id": "profile-private-email-form"
                                },
                                el("label", {
                                    "data-i18n": "profile.privateEmailLabel",
                                    "for": "profile-private-email"
                                }, "Primary email"),
                                el("div", {"class": "profile-security-inline"},
                                    el("input", {
                                        "autocomplete": "email",
                                        "class": "profile-input",
                                        "data-i18n-placeholder": "profile.privateEmailPlaceholder",
                                        "id": "profile-private-email",
                                        "maxlength": "254",
                                        "placeholder": "name@example.com",
                                        "required": true,
                                        "type": "email"
                                    }),
                                    el("button", {
                                        "class": "pill-btn pill-btn--primary profile-action-btn",
                                        "data-i18n": "profile.savePrivateEmail",
                                        "type": "submit"
                                    }, "Save ")
                                ),
                                el("p", {
                                    "class": "profile-security-hint",
                                    "data-i18n": "profile.privateEmailHint",
                                    "id": "profile-private-email-hint"
                                }, "This email is never shown publicly and can be used to sign in or recover your account.")
                            ),
                            el("section", {
                                    "aria-labelledby": "profile-email-aliases-title",
                                    "class": "profile-security-email-form"
                                },
                                el("strong", {
                                    "data-i18n": "profile.emailAliases",
                                    "id": "profile-email-aliases-title"
                                }, "Other login emails"),
                                el("p", {
                                    "class": "profile-security-hint",
                                    "data-i18n": "profile.emailAliasesHint"
                                }, "Other emails remain private and can be used to sign in, but cannot recover the account. Disconnecting a provider keeps its emails; a later provider sign-in can add removed addresses again."),
                                el("ul", {"class": "profile-email-aliases", "id": "profile-email-aliases"}),
                                el("form", {"id": "profile-email-alias-form"},
                                    el("label", {
                                        "data-i18n": "profile.addEmailAlias",
                                        "for": "profile-email-alias"
                                    }, "Add a verified email"),
                                    el("div", {"class": "profile-security-inline"},
                                        el("input", {
                                            "autocomplete": "email",
                                            "class": "profile-input",
                                            "id": "profile-email-alias",
                                            "maxlength": "254",
                                            "required": true,
                                            "type": "email"
                                        }),
                                        el("button", {
                                            "class": "pill-btn pill-btn--primary profile-action-btn",
                                            "data-i18n": "profile.verifyPrivateEmail",
                                            "type": "submit"
                                        }, "Verify email ")
                                    ),
                                    el("p", {
                                        "class": "profile-security-hint",
                                        "data-i18n": "profile.emailAliasMailDisabled",
                                        "hidden": true,
                                        "id": "profile-email-alias-mail-disabled"
                                    }, "Enable email delivery to verify another address, or verify it with your provider and refresh the connection.")
                                )
                            ),
                            el("div", {"class": "profile-security-control"},
                                el("div", {},
                                    el("strong", {"data-i18n": "profile.passwordLoginTitle"}, "Password login"),
                                    el("p", {"class": "profile-security-hint", "id": "profile-password-login-hint"})
                                ),
                                el("label", {"class": "profile-security-switch"},
                                    el("input", {"id": "profile-password-login-toggle", "type": "checkbox"}),
                                    el("span", {"aria-hidden": "true"}),
                                    el("span", {
                                        "class": "sr-only",
                                        "data-i18n": "profile.passwordLoginTitle"
                                    }, "Password login")
                                )
                            ),
                            el("div", {"class": "profile-security-control"},
                                el("div", {},
                                    el("strong", {
                                        "data-i18n": "mfa.passkeyTitle",
                                        "id": "profile-mfa-passkey-label"
                                    }, "Passkey as a second factor"),
                                    el("p", {
                                        "class": "profile-security-hint",
                                        "data-i18n": "mfa.passkeyHint"
                                    }, "Require a Passkey after password or third-party sign-in. A registered Passkey and another sign-in method are required.")
                                ),
                                el("label", {"class": "profile-security-switch"},
                                    el("input", {
                                        "aria-labelledby": "profile-mfa-passkey-label",
                                        "id": "profile-mfa-passkey",
                                        "type": "checkbox"
                                    }),
                                    el("span", {"aria-hidden": "true"})
                                )
                            ),
                            el("div", {"class": "profile-security-control"},
                                el("div", {},
                                    el("strong", {"data-i18n": "mfa.authenticator"}, "Authenticator app"),
                                    el("p", {"class": "profile-security-hint", "id": "profile-mfa-totp-status"})
                                ),
                                el("button", {
                                    "class": "pill-btn pill-btn--soft profile-action-btn",
                                    "data-i18n": "mfa.setup",
                                    "id": "profile-mfa-totp",
                                    "type": "button"
                                }, "Set up ")
                            ),
                            el("p", {
                                "class": "profile-security-hint",
                                "data-i18n": "mfa.recoveryHint"
                            }, "Save your recovery codes. Recovery clears the authenticator, turns off Passkey second-factor verification, and signs out all sessions. Use API tokens for package clients."),
                            el("div", {"class": "profile-security-control profile-recovery-control"},
                                el("div", {},
                                    el("strong", {"data-i18n": "profile.recoveryCodesTitle"}, "Recovery codes"),
                                    el("p", {
                                        "class": "profile-security-hint",
                                        "data-i18n": "profile.recoveryCodesNone",
                                        "id": "profile-recovery-code-status"
                                    }, "No recovery codes have been generated.")
                                ),
                                el("button", {
                                    "class": "pill-btn pill-btn--soft profile-action-btn",
                                    "data-i18n": "profile.generateRecoveryCodes",
                                    "id": "btn-profile-recovery-codes",
                                    "type": "button"
                                }, "Generate codes ")
                            ),
                            el("div", {"class": "profile-security-control profile-retirement-control"},
                                el("div", {},
                                    el("strong", {"data-i18n": "profile.retireAccountTitle"}, "Close account"),
                                    el("p", {
                                        "class": "profile-security-hint",
                                        "data-i18n": "profile.retireAccountHint"
                                    }, "Review ownership requirements and permanently close this account.")
                                ),
                                el("button", {
                                    "class": "pill-btn pill-btn--ghost-danger profile-action-btn",
                                    "data-i18n": "profile.retireAccount",
                                    "id": "btn-profile-retirement",
                                    "type": "button"
                                }, "Close account ")
                            )
                        )
                    )
                ),
                el("div", {"class": "profile-settings-section"},
                    el("div", {"class": "profile-section-card-header"},
                        el("div", {
                                "aria-hidden": "true",
                                "class": "profile-section-icon profile-section-icon--sessions"
                            },
                            svg("svg", {
                                    "fill": "none",
                                    "stroke": "currentColor",
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2",
                                    "viewBox": "0 0 24 24",
                                    "xmlns": "http://www.w3.org/2000/svg"
                                },
                                svg("rect", {"height": "14", "rx": "2", "width": "20", "x": "2", "y": "3"}),
                                svg("path", {"d": "M8 21h8"}),
                                svg("path", {"d": "M12 17v4"}),
                                svg("path", {"d": "M6 8h.01"}),
                                svg("path", {"d": "M9 8h6"})
                            )
                        ),
                        el("div", {"class": "profile-section-meta"},
                            el("h3", {
                                "class": "profile-section-title",
                                "data-i18n": "profile.sessionsTitle"
                            }, "Active Sessions"),
                            el("p", {
                                "class": "profile-section-desc",
                                "data-i18n": "profile.sessionsDesc"
                            }, "Review browser logins, devices, IPs, and revoke sessions you no longer trust.")
                        )
                    ),
                    el("div", {"class": "profile-section-body"},
                        el("button", {
                            "class": "pill-btn pill-btn--primary profile-action-btn",
                            "data-i18n": "profile.sessionsBtn",
                            "id": "btn-profile-sessions",
                            "type": "button"
                        }, "Manage Sessions ")
                    )
                ),
                el("div", {"class": "profile-settings-section"},
                    el("div", {"class": "profile-section-card-header"},
                        el("div", {
                                "aria-hidden": "true",
                                "class": "profile-section-icon profile-section-icon--fido",
                                "style": "background: rgba(16, 185, 129, 0.1); color: #10b981;"
                            },
                            svg("svg", {
                                    "fill": "none",
                                    "height": "20",
                                    "stroke": "currentColor",
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2",
                                    "viewBox": "0 0 24 24",
                                    "width": "20",
                                    "xmlns": "http://www.w3.org/2000/svg"
                                },
                                svg("path", {"d": "M12 2a5 5 0 0 0-5 5v3H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2h-1V7a5 5 0 0 0-5-5z"}),
                                svg("circle", {"cx": "12", "cy": "15", "r": "1.5"})
                            )
                        ),
                        el("div", {"class": "profile-section-meta"},
                            el("h3", {"class": "profile-section-title", "data-i18n": "profile.fidoTitle"}, "Passkeys"),
                            el("p", {
                                "class": "profile-section-desc",
                                "data-i18n": "profile.fidoDesc"
                            }, "Manage Passkeys used for passwordless login.")
                        )
                    ),
                    el("div", {"class": "profile-section-body"},
                        el("button", {
                            "class": "pill-btn pill-btn--primary profile-action-btn",
                            "data-i18n": "profile.fidoBtn",
                            "id": "btn-profile-fido",
                            "type": "button"
                        }, " Manage Passkeys ")
                    )
                ),
                el("div", {"hidden": true, "id": "profile-oauth-providers"}),
                el("div", {"class": "profile-settings-section", "id": "profile-api-token-section"},
                    el("div", {"class": "profile-section-card-header"},
                        el("div", {"aria-hidden": "true", "class": "profile-section-icon profile-section-icon--token"},
                            svg("svg", {
                                    "fill": "none",
                                    "stroke": "currentColor",
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2",
                                    "viewBox": "0 0 24 24",
                                    "xmlns": "http://www.w3.org/2000/svg"
                                },
                                svg("path", {"d": "M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"})
                            )
                        ),
                        el("div", {"class": "profile-section-meta"},
                            el("h3", {
                                "class": "profile-section-title",
                                "data-i18n": "profile.apiTokensTitle"
                            }, "API Tokens"),
                            el("p", {
                                "class": "profile-section-desc",
                                "data-i18n": "profile.apiTokensDesc"
                            }, "Create named, expiring credentials with only the permissions each automation needs.")
                        )
                    ),
                    el("div", {"class": "profile-section-body profile-api-token-summary"},
                        el("p", {
                            "class": "profile-security-hint",
                            "data-i18n": "profile.apiTokensLoading",
                            "id": "profile-api-token-status"
                        }, "Loading API tokens…"),
                        el("button", {
                            "class": "pill-btn pill-btn--primary profile-action-btn",
                            "data-i18n": "profile.manageApiTokens",
                            "id": "btn-manage-api-tokens",
                            "type": "button"
                        }, "Manage API Tokens ")
                    )
                ),
                el("div", {"class": "profile-settings-section"},
                    el("div", {"class": "profile-section-card-header"},
                        el("div", {
                                "aria-hidden": "true",
                                "class": "profile-section-icon profile-section-icon--audit",
                                "style": "background: rgba(99, 102, 241, 0.1); color: #6366f1;"
                            },
                            svg("svg", {
                                    "fill": "none",
                                    "height": "20",
                                    "stroke": "currentColor",
                                    "stroke-linecap": "round",
                                    "stroke-linejoin": "round",
                                    "stroke-width": "2",
                                    "viewBox": "0 0 24 24",
                                    "width": "20"
                                },
                                svg("path", {"d": "M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"}),
                                svg("polyline", {"points": "14 2 14 8 20 8"}),
                                svg("line", {"x1": "16", "x2": "8", "y1": "13", "y2": "13"}),
                                svg("line", {"x1": "16", "x2": "8", "y1": "17", "y2": "17"}),
                                svg("polyline", {"points": "10 9 9 9 8 9"})
                            )
                        ),
                        el("div", {"class": "profile-section-meta"},
                            el("h3", {
                                "class": "profile-section-title",
                                "data-i18n": "profile.auditLogsTitle"
                            }, "Activity Logs"),
                            el("p", {
                                "class": "profile-section-desc",
                                "data-i18n": "profile.auditLogsDesc"
                            }, "View your account activity and operation history.")
                        )
                    ),
                    el("div", {"class": "profile-section-body"},
                        el("button", {
                            "class": "pill-btn pill-btn--primary profile-action-btn",
                            "data-i18n": "profile.auditLogsBtn",
                            "id": "btn-profile-audit-logs",
                            "type": "button"
                        }, "View Activity Logs ")
                    )
                )
            )
        )
    );
}
