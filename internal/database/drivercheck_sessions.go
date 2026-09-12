/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database

import (
	"errors"
	"strings"

	"renop/internal/core"

	"github.com/google/uuid"
)

func checkScopedSessionNotifications(db *DB, username string, now int64) error {
	username += "-session"
	if err := db.SaveToken(&core.AccessToken{Name: username, Permissions: []string{"base"}}); err != nil {
		return err
	}
	first := &core.Session{Username: username, PublicID: uuid.NewString(), CreatedAt: now,
		OAuthGrant: &core.SessionOAuthGrant{ProviderID: "driver", Authority: strings.Repeat("a", 64), Subject: username, SessionID: "first", AuthorizedAt: now - 1000, EncryptedTokens: "opaque-ciphertext"}}
	first.LastActive.Store(now)
	second := &core.Session{Username: username, PublicID: uuid.NewString(), CreatedAt: now - 1,
		OAuthGrant: &core.SessionOAuthGrant{ProviderID: "driver", Authority: strings.Repeat("a", 64), Subject: username, SessionID: "second", AuthorizedAt: now - 1000, EncryptedTokens: "opaque-ciphertext"}}
	second.LastActive.Store(now)
	if err := db.SaveSession(first, first.PublicID); err != nil {
		return err
	}
	if err := db.SaveSession(second, second.PublicID); err != nil {
		return err
	}
	page, err := db.ListActiveUserSessions(username, 0, "", 1, now)
	if err != nil || len(page) != 1 || page[0].PublicID != first.PublicID {
		return errorsOrMissing(err, "active session page")
	}
	page, err = db.ListActiveUserSessions(username, page[0].CreatedAt, page[0].PublicID, 1, now)
	if err != nil || len(page) != 1 || page[0].PublicID != second.PublicID {
		return errorsOrMissing(err, "active session cursor")
	}
	private := &core.UserMessage{ID: uuid.NewString(), Recipient: username, SessionID: first.PublicID, Kind: "announcement", Title: "Private", CreatedAt: now}
	public := &core.UserMessage{ID: uuid.NewString(), Recipient: username, Kind: "announcement", Title: "Public", CreatedAt: now}
	if err := db.SaveMessages([]*core.UserMessage{private, public}); err != nil {
		return err
	}
	if count, err := db.CountUnreadMessages(username, now, ""); err != nil || count != 1 {
		return errorsOrMissing(err, "API message isolation")
	}
	if count, err := db.CountUnreadMessages(username, now, first.PublicID); err != nil || count != 2 {
		return errorsOrMissing(err, "target message scope")
	}
	if deleted, err := db.DeleteUserMessages(username, ""); err != nil || deleted != 1 {
		return errorsOrMissing(err, "scoped message clear")
	}
	if message, err := db.GetUserMessage(private.ID, username, now, first.PublicID); err != nil || message == nil {
		return errorsOrMissing(err, "private message preservation")
	}
	event := core.OAuthRevocation{EventID: uuid.NewString(), ProviderID: "driver", Authority: first.OAuthGrant.Authority, Subject: username, SessionID: "first", IssuedAt: now}
	tokens, err := db.RevokeOAuthSessions(event, now)
	if err != nil || len(tokens) != 1 || tokens[0] != first.PublicID {
		return errorsOrMissing(err, "provider session scope")
	}
	if _, err := db.RevokeOAuthSessions(event, now); err != nil {
		return err
	}
	if err := db.SaveSession(first, first.PublicID); !errors.Is(err, core.ErrMFAInvalid) {
		return errorsOrMissing(err, "revoked proof cannot create a session")
	}
	grant, err := db.GetSessionOAuthGrant(second.PublicID)
	if err != nil || grant == nil || grant.SessionID != "second" {
		return errorsOrMissing(err, "other provider session preserved")
	}
	return nil
}
