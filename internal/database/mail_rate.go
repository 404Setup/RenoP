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
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/netip"
	"strings"

	"renop/internal/mail"
	"renop/pkg/hex"
)

// debitManualMailTx shares a budget across scenes, addresses, and account aliases.
// The caller holds the durable mail lock and commits these debits with the queued message.
func debitManualMailTx(tx *Tx, job *mail.Job, secret, ip string, rate mail.Rate) error {
	if ip == "" {
		return nil
	}
	address, err := netip.ParseAddr(ip)
	if err != nil || rate.Limit <= 0 || rate.Interval.Duration() <= 0 {
		return errors.New("invalid manual mail rate or IP")
	}
	address = address.Unmap()
	// Keep the existing IP key so a deployment does not reset active limits.
	keys := []string{address.String()}
	key := func(scope, value string) string {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(scope + "\x00" + value))
		return hex.EncodeToString(mac.Sum(nil))
	}
	keys = append(keys, key("recipient", strings.ToLower(job.Message.To)))
	if job.UserID != "" {
		keys = append(keys, key("user", job.UserID))
	}
	if address.Is6() {
		keys = append(keys, key("network", netip.PrefixFrom(address, 64).Masked().String()))
	}
	type debit struct {
		key           string
		used, expires int64
		exists        bool
	}
	debits := make([]debit, 0, len(keys))
	missing := 0
	interval := rate.Interval.Duration().Milliseconds()
	for _, key := range keys {
		d := debit{key: key}
		err := tx.QueryRow(`SELECT used, expires_at FROM mail_rate_limits WHERE ip = ?`, key).Scan(&d.used, &d.expires)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		d.exists = err == nil
		if d.exists && d.expires > job.CreatedAt && d.used >= rate.Limit {
			return mail.ErrRateLimited
		}
		if !d.exists || d.expires <= job.CreatedAt {
			d.used, d.expires = 0, job.CreatedAt+interval
		}
		if !d.exists {
			missing++
		}
		debits = append(debits, d)
	}
	if missing > 0 {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM mail_rate_limits WHERE expires_at > ?`, job.CreatedAt).Scan(&count); err != nil {
			return err
		}
		if count+missing > 40000 {
			return mail.ErrRateLimited
		}
		// Expired keys being reused below must survive this cleanup.
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
		args := []any{job.CreatedAt}
		for _, key := range keys {
			args = append(args, key)
		}
		if _, err := tx.Exec(`DELETE FROM mail_rate_limits WHERE expires_at <= ? AND ip NOT IN (`+placeholders+`)`, args...); err != nil {
			return err
		}
	}
	for _, d := range debits {
		if d.exists {
			_, err = tx.Exec(`UPDATE mail_rate_limits SET used = ?, period_start = ?, expires_at = ? WHERE ip = ?`,
				d.used+1, d.expires-interval, d.expires, d.key)
		} else {
			_, err = tx.Exec(`INSERT INTO mail_rate_limits (ip, period_start, used, expires_at) VALUES (?, ?, 1, ?)`,
				d.key, job.CreatedAt, d.expires)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
