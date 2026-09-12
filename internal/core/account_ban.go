/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package core

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxAccountBanReasonRunes = 512

var (
	ErrAccountBanned       = errors.New("account is banned")
	ErrAccountBanInvalid   = errors.New("account ban is invalid")
	ErrAccountBanProtected = errors.New("administrator or moderator account cannot be banned")
	ErrAccountBanIPUnknown = errors.New("account has no recorded login IP addresses")
)

// AccountBan is a durable administrator suspension. A nil ExpiresAt is permanent.
type AccountBan struct {
	ReasonCode string `json:"reason_code,omitempty" yaml:"reason_code,omitempty"`
	Reason     string `json:"reason" yaml:"reason"`
	CreatedAt  int64  `json:"created_at" yaml:"created_at"`
	ExpiresAt  *int64 `json:"expires_at,omitempty" yaml:"expires_at,omitempty"`
}

// AccountBanReasonText returns the canonical fallback for an explicit localized preset.
func AccountBanReasonText(code string) (string, bool) {
	switch code {
	case "harassment_abuse":
		return "Harassment or abuse", true
	case "spam_misleading":
		return "Spam or misleading content", true
	case "automation":
		return "Automation", true
	case "alternate_accounts":
		return "Alternate accounts", true
	case "security_rules":
		return "Security rules triggered (protective suspension)", true
	case "harmful_content":
		return "Malware or harmful content", true
	case "terms_violation":
		return "Terms of service violation", true
	case "impersonation":
		return "Impersonation", true
	case "copyright":
		return "Copyright infringement", true
	default:
		return "", false
	}
}

// NormalizeConfiguredBanReason keeps custom text distinct from localized presets.
func NormalizeConfiguredBanReason(reason, code string) (string, bool) {
	if code != "" {
		return AccountBanReasonText(code)
	}
	return NormalizeAccountBanReason(reason)
}

// AccountBanStatus is the private administrator view of an account suspension.
type AccountBanStatus struct {
	Ban           *AccountBan `json:"ban"`
	IPCount       int         `json:"ip_count"`
	ProtectedRole bool        `json:"protected_role"`
}

// IsActive reports whether the suspension applies at now.
func (ban *AccountBan) IsActive(now int64) bool {
	return ban != nil && ban.CreatedAt > 0 && (ban.ExpiresAt == nil || now < *ban.ExpiresAt)
}

// Clone returns an independent ban value.
func (ban *AccountBan) Clone() *AccountBan {
	if ban == nil {
		return nil
	}
	cloned := *ban
	if ban.ExpiresAt != nil {
		expiresAt := *ban.ExpiresAt
		cloned.ExpiresAt = &expiresAt
	}
	return &cloned
}

// NormalizeAccountBanReason trims and validates one administrator-visible reason.
func NormalizeAccountBanReason(reason string) (string, bool) {
	reason = strings.TrimSpace(reason)
	if reason == "" || !utf8.ValidString(reason) || utf8.RuneCountInString(reason) > MaxAccountBanReasonRunes {
		return "", false
	}
	for _, character := range reason {
		if unicode.IsControl(character) {
			return "", false
		}
	}
	return reason, true
}
