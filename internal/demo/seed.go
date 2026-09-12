/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package demo

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/emmansun/base64"
	"golang.org/x/crypto/bcrypt"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/pkg/hex"
)

type seedContext struct {
	db    *database.DB
	cfg   *config.Config
	now   int64
	users map[string]string
}

func seed(db *database.DB, cfg *config.Config) error {
	for _, statement := range []string{
		`CREATE TABLE demo_state (name VARCHAR(64) PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE demo_files (repository VARCHAR(64) NOT NULL, path TEXT NOT NULL, size BIGINT NOT NULL, modified_at BIGINT NOT NULL, PRIMARY KEY (repository, path))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	// Repository definitions live in the application database, independently of
	// the configuration snapshot, whose serialization deliberately excludes them.
	repositories := defaultRepositories()
	if err := repositories.Normalize(); err != nil {
		return err
	}
	if err := db.SaveRepositorySettings(repositories); err != nil {
		return err
	}
	s := &seedContext{db: db, cfg: cfg, now: time.Now().UnixMilli(), users: make(map[string]string)}
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"accounts", s.accounts}, {"teams", s.teams}, {"packages", s.packages},
		{"workflows", s.workflows}, {"activity", s.activity},
	} {
		if err := step.run(); err != nil {
			return fmt.Errorf("seed %s: %w", step.name, err)
		}
	}
	_, err := db.Exec(`INSERT INTO demo_state (name, value) VALUES ('seed_version', ?)`, seedVersion)
	return err
}

func (s *seedContext) file(repository, path string, size int64) error {
	_, err := s.db.Exec(`INSERT INTO demo_files (repository, path, size, modified_at) VALUES (?, ?, ?, ?)`, repository, path, size, s.now-3600000)
	return err
}

func digest(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func (s *seedContext) accounts() error {
	for i, person := range []struct {
		name, nickname string
		permissions    []string
	}{
		{"admin", "Demo Administrator", []string{"admin"}},
		{"alice", "Alice Chen", []string{"base", "canupdate:releases", "canupdate:cargo", "canupdate:npm", "canupdate:docker"}},
		{"bobby", "Bob Martin", []string{"base"}},
		{"carol", "Carol Garcia", []string{"base", "canmoderate:*"}},
		{"david", "David Kim", []string{"base"}},
		{"suspended", "Suspended example", []string{"base"}},
		{"retired", "Former contributor", []string{"base"}},
	} {
		password := rand.Text()
		if person.name == "admin" {
			password = "12345678"
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		created := s.now - int64(90-i*7)*24*time.Hour.Milliseconds()
		if err := s.db.CreateToken(&core.AccessToken{Name: person.name, EncryptedSecret: string(hash),
			CreatedAt: time.UnixMilli(created).UTC().Format(time.RFC3339), Description: "Preset demonstration account", Permissions: person.permissions}, person.nickname, created); err != nil {
			return err
		}
		if _, err := s.db.UpdateAccountEmail(person.name, person.name+"@example.com", created); err != nil {
			return err
		}
		profile, err := s.db.GetUserProfile(person.name)
		if err != nil {
			return err
		}
		s.users[person.name] = profile.UserID
		session := &core.Session{Username: person.name, PublicID: "demo-session-" + person.name,
			IP: fmt.Sprintf("192.0.2.%d", 10+i), UserAgent: "Mozilla/5.0 RenoP demonstration browser",
			CreatedAt: s.now - int64(i+1)*time.Hour.Milliseconds(), LoginMethod: "password"}
		session.LastActive.Store(s.now - int64(i+1)*time.Minute.Milliseconds())
		if err := s.db.SaveSession(session, "preset-session-"+person.name); err != nil {
			return err
		}
	}
	if err := s.db.SaveFidoDevice(&core.FidoDevice{ID: "demo-passkey", Username: "admin", Name: "Example security key",
		CredentialID: []byte("demo-display-key"), PublicKey: []byte("demo-display-public-key"), CreatedAt: s.now - 10*24*time.Hour.Milliseconds(), UserPresent: true, UserVerified: true}); err != nil {
		return err
	}
	verifiers := make([]core.RecoveryCodeHash, core.RecoveryCodeCount)
	for i := range verifiers {
		// Random verifiers have no corresponding published recovery credential.
		salt, verifier := make([]byte, 16), make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
		if _, err := rand.Read(verifier); err != nil {
			return err
		}
		verifiers[i] = core.RecoveryCodeHash{SelectorHash: digest(rand.Text()), CreatedAt: s.now - 24*time.Hour.Milliseconds(),
			PasswordHash: "$argon2id$v=19$m=19456,t=2,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(verifier)}
	}
	if err := s.db.ReplaceRecoveryCodes("admin", verifiers); err != nil {
		return err
	}
	if err := s.db.CreateAPIToken("admin", &core.APIToken{ID: "00000000-0000-4000-8000-000000000001", Name: "CI package publishing",
		Scopes: []string{core.APITokenScopeRepositoryRead, core.APITokenScopeRepositoryPublish}, CreatedAt: s.now - 3*24*time.Hour.Milliseconds()}, digest(rand.Text())); err != nil {
		return err
	}
	until := s.now + 7*24*time.Hour.Milliseconds()
	if err := s.db.SetAccountBan("suspended", &core.AccountBan{ReasonCode: "spam_misleading", CreatedAt: s.now - time.Hour.Milliseconds(), ExpiresAt: &until}, true); err != nil {
		return err
	}
	return s.db.RetireAccount("retired", s.now-2*24*time.Hour.Milliseconds())
}

func (s *seedContext) teams() error {
	for i, definition := range []struct{ prefix, name, description string }{
		{"platform", "Platform Engineering", "Shared libraries, developer tools and deployment images."},
		{"labs", "Research Lab", "Experimental packages and early previews."},
	} {
		at := s.now - int64(30-i*5)*24*time.Hour.Milliseconds()
		team := &core.SuperTeam{Prefix: definition.prefix, Name: definition.name, Description: definition.description,
			CreatedAt: at, UpdatedAt: at, Links: core.PublicLinks{Website: "https://example.com"}}
		if err := s.db.CreateSuperTeam(team, "admin", 10, 20); err != nil {
			return err
		}
		if err := s.db.ForceAddSuperTeamMembers(definition.prefix, "admin", []string{"alice", "bobby"}, core.SuperTeamRoleManage, 10, 20, at+1000); err != nil {
			return err
		}
	}
	bytes := int64(5 << 30)
	files := int64(2000)
	return s.db.SetPublicationQuotaOverride(core.PublicationQuotaSubject{OwnerType: core.PublicationQuotaOwnerSuperTeam, OwnerKey: "platform"},
		core.PublicationQuotaOverride{ByteLimit: &bytes, FileLimit: &files}, s.now)
}
