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
	"context"
	"strings"
	"testing"
	"time"

	"renop/internal/core"

	"github.com/stretchr/testify/require"
)

func TestMySQLDriverContract(t *testing.T) {
	db, _ := newMySQLTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	results, err := RunDriverCheck(ctx, db)
	require.NoError(t, err)
	require.Len(t, results, 24)
}

func TestMySQLMavenFullLengthKeysAndUnicodeMetadata(t *testing.T) {
	db, cfg := newMySQLTestDatabase(t)
	var charset string
	require.NoError(t, db.QueryRow(`SELECT @@character_set_database`).Scan(&charset))
	require.Equal(t, "utf8mb4", charset)
	require.NoError(t, db.QueryRow(`SELECT CHARACTER_SET_NAME FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'maven_versions' AND COLUMN_NAME = 'version'`).Scan(&charset))
	require.Equal(t, "ascii", charset)
	var prefixes int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE()
		AND TABLE_NAME = 'maven_versions' AND INDEX_NAME = 'PRIMARY' AND SUB_PART IS NOT NULL`).Scan(&prefixes))
	require.Zero(t, prefixes, "identity keys must never use a truncated prefix")

	now := time.Now().UnixMilli()
	repository := strings.Repeat("r", 64)
	group := "example.test." + strings.Repeat("g", 240)
	artifact := &core.MavenArtifact{Repository: repository, Domain: "example.test", GroupID: group,
		ArtifactID: strings.Repeat("a", 255), Description: "Unicode metadata \U0001F680", Readme: "# Unicode \U0001F600", CreatedAt: now}
	versions := []string{strings.Repeat("v", 254) + "A", strings.Repeat("v", 254) + "B", "1.0-RC", "1.0-rc"}
	for _, name := range versions {
		require.True(t, core.ValidMavenCoordinatePart(name))
		require.NoError(t, db.RecordMavenMirrorPublication(artifact, &core.MavenVersion{Version: name, Size: 42, CreatedAt: now}))
	}
	require.NoError(t, db.RecordMavenMirrorPublication(artifact, &core.MavenVersion{Version: versions[0], Size: 43, CreatedAt: now}))
	details, err := db.GetMavenArtifactDetails(repository, group, artifact.ArtifactID)
	require.NoError(t, err)
	require.Equal(t, artifact.Description, details.Artifact.Description)
	require.Equal(t, artifact.Readme, details.Artifact.Readme)
	require.Len(t, details.Versions, len(versions))
	for _, name := range versions {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM maven_versions WHERE repository = ? AND group_id = ? AND artifact_id = ? AND version = ?`, repository, group, artifact.ArtifactID, name).Scan(&count))
		require.Equal(t, 1, count)
	}
	require.NoError(t, db.DeleteMavenVersionMetadata(repository, group, artifact.ArtifactID, versions[0]))
	require.NoError(t, db.Close())
	reopened, err := InitDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	details, err = reopened.GetMavenArtifactDetails(repository, group, artifact.ArtifactID)
	require.NoError(t, err)
	require.Len(t, details.Versions, len(versions)-1)
	require.Equal(t, artifact.Readme, details.Artifact.Readme)
}

func TestMySQLExistingMavenVersionTableIsPreserved(t *testing.T) {
	db, cfg := newMySQLTestDatabase(t)
	// Simulate a working legacy table; startup must not transcode or discard its stored values.
	_, err := db.Exec(`ALTER TABLE maven_versions MODIFY version VARCHAR(255) CHARACTER SET utf8mb3 COLLATE utf8mb3_bin NOT NULL`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO maven_versions (repository, group_id, artifact_id, version, publisher, size, created_at)
		VALUES ('legacy', 'example.test', 'artifact', ?, '', 1, 1)`, "版本-1")
	require.NoError(t, err)
	require.NoError(t, db.Close())
	reopened, err := InitDB(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	var version string
	require.NoError(t, reopened.QueryRow(`SELECT version FROM maven_versions WHERE repository = 'legacy' AND group_id = 'example.test' AND artifact_id = 'artifact'`).Scan(&version))
	require.Equal(t, "版本-1", version)
}

func TestMySQLNPMNoOpDeprecations(t *testing.T) {
	db, _ := newMySQLTestDatabase(t)
	now := time.Now().UnixMilli()
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "alice", Permissions: []string{"base"}}))
	pkg, err := db.CreateNPMPackage("npm", "noop", "alice", false, now)
	require.NoError(t, err)
	require.NoError(t, db.RecordNPMPublication(pkg, &core.NPMVersion{Repository: "npm", Package: "noop", Version: "1.0.0",
		ManifestJSON: `{"name":"noop","version":"1.0.0"}`, CreatedAt: now}, map[string]string{"latest": "1.0.0"}, "alice"))
	for _, deprecated := range []string{"", "Use the next version", "Use the next version", "", ""} {
		require.NoError(t, db.SetNPMVersionDeprecated("npm", "noop", "1.0.0", deprecated, "alice", 0))
		require.NoError(t, db.UpdateNPMPackument("npm", "noop", "alice", 0, map[string]string{"1.0.0": deprecated}, map[string]string{"latest": "1.0.0"}))
	}
	require.ErrorIs(t, db.SetNPMVersionDeprecated("npm", "noop", "missing", "", "alice", 0), core.ErrNPMVersionNotFound)
	require.ErrorIs(t, db.UpdateNPMPackument("npm", "noop", "alice", 0, map[string]string{"missing": ""}, nil), core.ErrNPMVersionNotFound)
}
