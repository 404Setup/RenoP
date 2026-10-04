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
	"strings"
	"time"

	"renop/internal/core"
)

var candidateTables = []struct {
	name        string
	reclaimable bool
}{
	{"sessions", true},
	{"audit_logs", true},
	{"mail_jobs", true},
	{"account_registrations", true},
	{"registration_ips", true},
	{"user_email_changes", true},
	{"mail_attempts", true},
	{"user_messages", false},
	{"tokens", false},
	{"user_profiles", false},
	{"account_emails", false},
	{"fido_devices", false},
	{"gpg_public_keys", false},
	{"gpg_signatures", false},
	{"maven_domains", false},
	{"maven_domain_members", false},
	{"maven_artifacts", false},
	{"maven_versions", false},
	{"npm_packages", false},
	{"npm_versions", false},
	{"npm_dist_tags", false},
	{"cargo_packages", false},
	{"cargo_versions", false},
	{"docker_images", false},
	{"docker_tags", false},
	{"docker_manifests", false},
	{"docker_blobs", false},
	{"review_tasks", false},
	{"ticket_state", false},
	{"ticket_messages", false},
	{"super_teams", false},
	{"super_team_members", false},
	{"resource_locks", false},
	{"package_deprecations", false},
}

func (db *DB) GetDatabaseStats() (*core.DatabaseStats, error) {
	if db == nil {
		return &core.DatabaseStats{}, nil
	}
	driverName := "sqlite"
	if db.Dialect != nil {
		driverName = db.Dialect.Name()
	}

	stats := &core.DatabaseStats{
		Driver: driverName,
		Tables: make([]core.TableStat, 0, len(candidateTables)),
	}

	// Calculate SQLite database size if sqlite
	if strings.Contains(strings.ToLower(driverName), "sqlite") && db.SQLDB != nil {
		var pageCount, pageSize int64
		_ = db.SQLDB.QueryRow("PRAGMA page_count;").Scan(&pageCount)
		_ = db.SQLDB.QueryRow("PRAGMA page_size;").Scan(&pageSize)
		stats.TotalSizeBytes = uint64(pageCount * pageSize)
	}

	now := time.Now().UnixMilli()
	var totalRows int64
	for _, t := range candidateTables {
		if db.SQLDB == nil {
			continue
		}
		var count int64
		row := db.SQLDB.QueryRow("SELECT COUNT(*) FROM " + t.name)
		if err := row.Scan(&count); err != nil {
			// Table may not exist in some configurations or drivers
			continue
		}
		totalRows += count

		var reclaimableCount int64
		if t.reclaimable {
			switch t.name {
			case "sessions":
				_ = db.SQLDB.QueryRow("SELECT COUNT(*) FROM sessions WHERE last_active < ?", now-core.SessionIdleTimeoutMillis).Scan(&reclaimableCount)
			case "audit_logs":
				cutoff := now - int64(30*24*time.Hour/time.Millisecond)
				_ = db.SQLDB.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE created_at < ?", cutoff).Scan(&reclaimableCount)
			case "mail_jobs":
				_ = db.SQLDB.QueryRow("SELECT COUNT(*) FROM mail_jobs WHERE status IN ('sent', 'failed', 'delivered')").Scan(&reclaimableCount)
			case "account_registrations":
				_ = db.SQLDB.QueryRow("SELECT COUNT(*) FROM account_registrations WHERE cooldown_until <= ? OR (expires_at > 0 AND expires_at <= ?)", now, now).Scan(&reclaimableCount)
			case "registration_ips":
				_ = db.SQLDB.QueryRow("SELECT COUNT(*) FROM registration_ips WHERE expires_at <= ?", now).Scan(&reclaimableCount)
			case "user_email_changes":
				_ = db.SQLDB.QueryRow("SELECT COUNT(*) FROM user_email_changes WHERE expires_at <= ?", now).Scan(&reclaimableCount)
			default:
				reclaimableCount = count
			}
			stats.ReclaimableRows += reclaimableCount
		}

		stats.Tables = append(stats.Tables, core.TableStat{
			Name:        t.name,
			RowCount:    count,
			Reclaimable: t.reclaimable && reclaimableCount > 0,
		})
	}

	// Estimate per-table size if totalSizeBytes > 0
	if stats.TotalSizeBytes > 0 && totalRows > 0 {
		avgRowBytes := float64(stats.TotalSizeBytes) / float64(totalRows)
		for i := range stats.Tables {
			stats.Tables[i].SizeBytes = int64(float64(stats.Tables[i].RowCount) * avgRowBytes)
		}
		stats.ReclaimableBytes = uint64(float64(stats.ReclaimableRows) * avgRowBytes)
	} else if stats.TotalSizeBytes == 0 {
		stats.TotalSizeBytes = uint64(totalRows * 256)
		for i := range stats.Tables {
			stats.Tables[i].SizeBytes = stats.Tables[i].RowCount * 256
		}
		stats.ReclaimableBytes = uint64(stats.ReclaimableRows * 256)
	}

	return stats, nil
}

func (db *DB) CleanupUnimportantData() (*core.DatabaseCleanupResult, error) {
	if db == nil {
		return &core.DatabaseCleanupResult{}, nil
	}
	now := time.Now().UnixMilli()
	var cleanedRows int64

	driverName := ""
	if db.Dialect != nil {
		driverName = strings.ToLower(db.Dialect.Name())
	}
	isSQLite := strings.Contains(driverName, "sqlite") && db.SQLDB != nil

	var sizeBefore int64
	if isSQLite {
		var pageCount, pageSize int64
		_ = db.SQLDB.QueryRow("PRAGMA page_count;").Scan(&pageCount)
		_ = db.SQLDB.QueryRow("PRAGMA page_size;").Scan(&pageSize)
		sizeBefore = pageCount * pageSize
	}

	// 1. Expired sessions
	sessionCutoff := now - core.SessionIdleTimeoutMillis
	if res, err := db.Exec("DELETE FROM sessions WHERE last_active < ?", sessionCutoff); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}
	if db.sessionCache != nil {
		db.sessionCache.EvictExpired()
	}
	_ = db.pruneOAuthRevocations(now)

	// 2. Expired pending registrations & temporary tables
	if res, err := db.Exec("DELETE FROM account_registrations WHERE cooldown_until <= ? OR (expires_at > 0 AND expires_at <= ?)", now, now); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}
	if res, err := db.Exec("DELETE FROM registration_ips WHERE expires_at <= ?", now); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}
	if res, err := db.Exec("DELETE FROM user_email_changes WHERE expires_at <= ?", now); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}

	// 3. Sent / failed mail jobs
	if res, err := db.Exec("DELETE FROM mail_jobs WHERE status IN ('sent', 'failed', 'delivered')"); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}

	// 4. Old audit logs (keep last 30 days)
	cutoff := now - int64(30*24*time.Hour/time.Millisecond)
	if res, err := db.Exec("DELETE FROM audit_logs WHERE created_at < ?", cutoff); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}

	// 5. Expired user messages
	if res, err := db.Exec("DELETE FROM user_messages WHERE expires_at > 0 AND expires_at <= ?", now); err == nil && res != nil {
		if n, _ := res.RowsAffected(); n > 0 {
			cleanedRows += n
		}
	}

	// 6. Invalidate caches
	db.InvalidateCaches()

	// 7. VACUUM SQLite to reclaim file space
	var reclaimedBytes uint64
	if isSQLite {
		_, _ = db.SQLDB.Exec("VACUUM;")
		var pageCountAfter, pageSizeAfter int64
		_ = db.SQLDB.QueryRow("PRAGMA page_count;").Scan(&pageCountAfter)
		_ = db.SQLDB.QueryRow("PRAGMA page_size;").Scan(&pageSizeAfter)
		sizeAfter := pageCountAfter * pageSizeAfter
		if sizeBefore > sizeAfter {
			reclaimedBytes = uint64(sizeBefore - sizeAfter)
		}
	}

	if reclaimedBytes == 0 && cleanedRows > 0 {
		reclaimedBytes = uint64(cleanedRows * 256)
	}

	return &core.DatabaseCleanupResult{
		CleanedRows:    cleanedRows,
		ReclaimedBytes: reclaimedBytes,
	}, nil
}
