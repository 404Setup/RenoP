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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/mail"
	"renop/internal/testutil"
)

func TestPrimaryEmailRecoveryAndSecurityHold(t *testing.T) {
	db, err := InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "primary-email.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	now := time.Now().UnixMilli()
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "alice", EncryptedSecret: "original-password"}))
	_, err = db.UpdateAccountEmail("alice", "original@example.com", now)
	require.NoError(t, err)
	hashes := testRecoveryHashes(now)
	require.NoError(t, db.ReplaceRecoveryCodes("alice", hashes))
	selectors := []string{hashes[0].SelectorHash, hashes[1].SelectorHash, hashes[2].SelectorHash, hashes[3].SelectorHash}
	session := &core.Session{Username: "alice", PublicID: "browser", CreatedAt: now, LoginMethod: "password"}
	session.LastActive.Store(now)
	require.NoError(t, db.SaveSession(session, "session"))
	device := &core.FidoDevice{Username: "alice", ID: "key", Name: "Existing key", CredentialID: []byte("key"), PublicKey: []byte("public"), CreatedAt: now}
	require.NoError(t, db.SaveFidoDevice(device))
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	require.NoError(t, db.UpdateMFA("alice", mfa.Snapshot, "encrypted-secret", true, 0, "session"))
	mfa, err = db.GetMFAState("alice")
	require.NoError(t, err)
	_, err = db.UpdateAccountEmailFromSession("alice", "session", "original@example.com", mfa.Snapshot, now+1,
		core.ProviderEmail{Email: "alias@example.com", Verified: true})
	require.NoError(t, err)
	_, _, err = db.GetRecoveryCodes("alias@example.com", selectors)
	require.ErrorIs(t, err, core.ErrRecoveryCodesInvalid)
	_, _, err = db.GetRecoveryCodes("alice", selectors)
	require.ErrorIs(t, err, core.ErrRecoveryCodesInvalid)
	_, err = db.UpdateAccountEmail("alice", "middle@example.com", now+2)
	require.NoError(t, err)
	security, err := db.UpdateAccountEmail("alice", "replacement@example.com", now+3)
	require.NoError(t, err)
	require.Equal(t, now+3+core.PrimaryEmailRecoveryPeriod.Milliseconds(), security.SecurityHoldUntil)
	require.Len(t, security.PreviousPrimaryEmails, 2)
	require.NotContains(t, security.EmailAliases, "original@example.com")
	oldLogin, err := db.GetTokenByEmail("original@example.com")
	require.NoError(t, err)
	require.Nil(t, oldLogin, "a protected recovery address is not a new login alias")
	_, candidates, err := db.GetRecoveryCodes("original@example.com", selectors)
	require.NoError(t, err)
	require.Len(t, candidates, core.RecoveryCodesRequired)
	require.ErrorIs(t, db.ReplaceRecoveryCodes("alice", testRecoveryHashes(now+5)), core.ErrSecurityHold)
	device.ID, device.CredentialID = "new-key", []byte("new-key")
	require.ErrorIs(t, db.SaveFidoDevice(device), core.ErrSecurityHold)
	require.ErrorIs(t, db.DeleteFidoDevice("alice", "key"), core.ErrSecurityHold)
	require.ErrorIs(t, db.DeleteFidoDevicesByUsername("alice"), core.ErrSecurityHold)
	mfa, err = db.GetMFAState("alice")
	require.NoError(t, err)
	require.ErrorIs(t, db.UpdateMFA("alice", mfa.Snapshot, "", false, 0, "session"), core.ErrSecurityHold)
	_, err = db.DeleteAccountEmailAlias("alice", "session", "original@example.com", now+4)
	require.ErrorIs(t, err, core.ErrSecurityHold)
	// Authentication counters still work while adding/removing credentials is frozen.
	require.NoError(t, db.UpdateFidoDeviceState([]byte("key"), 1, false, false))
	mailConfig := mail.DefaultConfig()
	require.NoError(t, mailConfig.EnsureKey())
	for _, email := range []string{"alias@example.com", "original@example.com", "middle@example.com"} {
		job := &mail.Job{ID: "reset", AccountID: "sender", Scene: "password_reset", TicketHash: "ticket",
			CreatedAt: now + 4, ExpiresAt: now + 600000, Message: mail.Message{ID: "reset", To: email, Subject: "Reset", Text: "Code"}}
		_, err := db.QueueEmailPasswordReset(job, strings.Repeat("a", 64), mailConfig.EncryptionKey, "192.0.2.1", mailConfig.ManualRate)
		require.ErrorIs(t, err, core.ErrEmailCodeInvalid)
	}
	deadline := now + 2 + core.PrimaryEmailRecoveryPeriod.Milliseconds()
	_, err = db.ResetPasswordWithRecoveryCodes("original@example.com", selectors, "must-not-apply", deadline)
	require.ErrorIs(t, err, core.ErrRecoveryCodesInvalid)
	beforeRecovery, err := db.GetAccountSecurity("alice")
	require.NoError(t, err)
	require.Equal(t, core.RecoveryCodeCount, beforeRecovery.RecoveryCodesRemaining)
	_, err = db.ResetPasswordWithRecoveryCodes("original@example.com", selectors, "recovered-password", deadline-1)
	require.NoError(t, err)
	restored, err := db.GetAccountSecurity("alice")
	require.NoError(t, err)
	require.Equal(t, "original@example.com", restored.Email)
	require.NotContains(t, restored.EmailAliases, "replacement@example.com")
	require.Contains(t, restored.EmailAliases, "alias@example.com")
	require.Zero(t, restored.SecurityHoldUntil)
	require.Empty(t, restored.PreviousPrimaryEmails)
	require.False(t, restored.TOTPEnabled)
	oldSession, err := db.GetSession("session")
	require.NoError(t, err)
	require.Nil(t, oldSession)
	require.NoError(t, db.ReplaceRecoveryCodes("alice", testRecoveryHashes(deadline)))
	_, _, err = db.GetRecoveryCodes("middle@example.com", selectors)
	require.ErrorIs(t, err, core.ErrRecoveryCodesInvalid)
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "bobby", EncryptedSecret: "password"}))
	_, err = db.UpdateAccountEmail("bobby", "middle@example.com", deadline)
	require.NoError(t, err, "recovery must not leave an invisible reservation for an abandoned primary")
}

func TestPrimaryEmailReservationExpiryPreservesAliasesAndRetirement(t *testing.T) {
	db, err := InitDB(config.DatabaseConfig{Driver: "sqlite", Dsn: filepath.Join(testutil.TempDir(t), "email-expiry.db")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	now := time.Now().UnixMilli()
	for _, name := range []string{"alice", "bobby", "retiree"} {
		require.NoError(t, db.SaveToken(&core.AccessToken{Name: name, EncryptedSecret: "password"}))
	}
	_, err = db.UpdateAccountEmail("alice", "former@example.com", now)
	require.NoError(t, err)
	session := &core.Session{Username: "alice", PublicID: "browser", CreatedAt: now, LoginMethod: "password"}
	session.LastActive.Store(now)
	require.NoError(t, db.SaveSession(session, "session"))
	mfa, err := db.GetMFAState("alice")
	require.NoError(t, err)
	_, err = db.UpdateAccountEmailFromSession("alice", "session", "former@example.com", mfa.Snapshot, now+1,
		core.ProviderEmail{Email: "kept@example.com", Verified: true})
	require.NoError(t, err)
	_, err = db.UpdateAccountEmail("alice", "kept@example.com", now+2)
	require.NoError(t, err)
	_, err = db.UpdateAccountEmail("alice", "current@example.com", now+3)
	require.NoError(t, err)
	deadline := now + 3 + core.PrimaryEmailRecoveryPeriod.Milliseconds()
	require.NoError(t, db.CleanupRetiredAccountData(now+4, 10))
	_, err = db.UpdateAccountEmail("bobby", "former@example.com", now+5)
	require.ErrorIs(t, err, core.ErrEmailAlreadyExists)
	require.NoError(t, db.CleanupRetiredAccountData(deadline, 10))
	_, err = db.UpdateAccountEmail("bobby", "former@example.com", deadline)
	require.NoError(t, err, "a recovery-only reservation must be released after expiry")
	for _, email := range []string{"current@example.com", "kept@example.com"} {
		owner, err := db.GetTokenByEmail(email)
		require.NoError(t, err)
		require.NotNil(t, owner)
		require.Equal(t, "alice", owner.Name)
	}
	_, err = db.UpdateAccountEmail("retiree", "retired-former@example.com", now)
	require.NoError(t, err)
	_, err = db.UpdateAccountEmail("retiree", "retired-current@example.com", now+1)
	require.NoError(t, err)
	retiredAt := now + core.PrimaryEmailRecoveryPeriod.Milliseconds() - 1
	require.NoError(t, db.RetireAccount("retiree", retiredAt))
	require.NoError(t, db.CleanupRetiredAccountData(deadline, 10))
	_, err = db.UpdateAccountEmail("bobby", "retired-former@example.com", deadline)
	require.ErrorIs(t, err, core.ErrEmailAlreadyExists, "retirement has its own retention deadline")
	require.NoError(t, db.CleanupRetiredAccountData(retiredAt+core.AccountEmailHoldMillis, 10))
	_, err = db.UpdateAccountEmail("bobby", "retired-former@example.com", retiredAt+core.AccountEmailHoldMillis)
	require.NoError(t, err)
}
