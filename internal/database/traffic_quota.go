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
	"database/sql"
	"errors"
	"strings"

	"renop/internal/core"
)

// GetTrafficQuotaUserOverride retrieves a user's traffic quota overrides if present.
func (db *DB) GetTrafficQuotaUserOverride(username string) (*core.TrafficQuotaUserOverride, error) {
	if db == nil || db.SQLDB == nil {
		return nil, core.ErrDatabaseUnavailable
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return nil, nil
	}

	row := db.QueryRow(`
		SELECT username, hourly_limit_bytes, daily_limit_bytes, monthly_limit_bytes,
		       hourly_enabled, daily_enabled, monthly_enabled,
		       recovery_amount_bytes, recovery_interval_s, unlimited, updated_at
		FROM traffic_quota_user_overrides
		WHERE username = ?
	`, username)

	var override core.TrafficQuotaUserOverride
	var unlimitedInt int
	err := row.Scan(
		&override.Username,
		&override.HourlyLimitBytes,
		&override.DailyLimitBytes,
		&override.MonthlyLimitBytes,
		&override.HourlyEnabled,
		&override.DailyEnabled,
		&override.MonthlyEnabled,
		&override.RecoveryAmountBytes,
		&override.RecoveryIntervalS,
		&unlimitedInt,
		&override.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	override.Unlimited = unlimitedInt == 1
	return &override, nil
}

// SetTrafficQuotaUserOverride inserts or updates a user's traffic quota overrides.
func (db *DB) SetTrafficQuotaUserOverride(username string, override *core.TrafficQuotaUserOverride, updatedAt int64) error {
	if db == nil || db.SQLDB == nil {
		return core.ErrDatabaseUnavailable
	}
	if override == nil {
		return db.DeleteTrafficQuotaUserOverride(username)
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return core.ErrUserProfileNotFound
	}

	unlimitedInt := 0
	if override.Unlimited {
		unlimitedInt = 1
	}

	query := `
		INSERT INTO traffic_quota_user_overrides
		(username, hourly_limit_bytes, daily_limit_bytes, monthly_limit_bytes,
		 hourly_enabled, daily_enabled, monthly_enabled,
		 recovery_amount_bytes, recovery_interval_s, unlimited, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET
		 hourly_limit_bytes = excluded.hourly_limit_bytes,
		 daily_limit_bytes = excluded.daily_limit_bytes,
		 monthly_limit_bytes = excluded.monthly_limit_bytes,
		 hourly_enabled = excluded.hourly_enabled,
		 daily_enabled = excluded.daily_enabled,
		 monthly_enabled = excluded.monthly_enabled,
		 recovery_amount_bytes = excluded.recovery_amount_bytes,
		 recovery_interval_s = excluded.recovery_interval_s,
		 unlimited = excluded.unlimited,
		 updated_at = excluded.updated_at
	`
	if db.Dialect.Name() == "mysql" {
		query = `
			INSERT INTO traffic_quota_user_overrides
			(username, hourly_limit_bytes, daily_limit_bytes, monthly_limit_bytes,
			 hourly_enabled, daily_enabled, monthly_enabled,
			 recovery_amount_bytes, recovery_interval_s, unlimited, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			 hourly_limit_bytes = VALUES(hourly_limit_bytes),
			 daily_limit_bytes = VALUES(daily_limit_bytes),
			 monthly_limit_bytes = VALUES(monthly_limit_bytes),
			 hourly_enabled = VALUES(hourly_enabled),
			 daily_enabled = VALUES(daily_enabled),
			 monthly_enabled = VALUES(monthly_enabled),
			 recovery_amount_bytes = VALUES(recovery_amount_bytes),
			 recovery_interval_s = VALUES(recovery_interval_s),
			 unlimited = VALUES(unlimited),
			 updated_at = VALUES(updated_at)
		`
	}

	_, err := db.Exec(query,
		username,
		override.HourlyLimitBytes,
		override.DailyLimitBytes,
		override.MonthlyLimitBytes,
		override.HourlyEnabled,
		override.DailyEnabled,
		override.MonthlyEnabled,
		override.RecoveryAmountBytes,
		override.RecoveryIntervalS,
		unlimitedInt,
		updatedAt,
	)
	return err
}

// DeleteTrafficQuotaUserOverride removes any override for a user.
func (db *DB) DeleteTrafficQuotaUserOverride(username string) error {
	if db == nil || db.SQLDB == nil {
		return core.ErrDatabaseUnavailable
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return nil
	}
	_, err := db.Exec(`DELETE FROM traffic_quota_user_overrides WHERE username = ?`, username)
	return err
}
