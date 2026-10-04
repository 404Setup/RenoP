/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package traffic

import (
	"sync"
	"time"
)

// Bucket holds the dynamic remaining traffic quota and window states for an IP or User.
type Bucket struct {
	mu sync.Mutex

	RemainingHour   int64
	RemainingDay    int64
	RemainingMonth  int64
	LastRecoveredAt int64

	HourWindowStart  int64
	DayWindowStart   int64
	MonthWindowStart int64

	initialized bool
}

// EffectiveTier represents computed limits and rolling recovery settings for a bucket.
type EffectiveTier struct {
	HourlyLimitBytes    int64
	DailyLimitBytes     int64
	MonthlyLimitBytes   int64
	RecoveryAmountBytes int64
	RecoveryIntervalS   int64
	HourlyEnabled       bool
	DailyEnabled        bool
	MonthlyEnabled      bool
	Unlimited           bool
}

// updateLazy locked updates bucket remaining traffic based on elapsed time.
func (b *Bucket) updateLazy(now int64, tier EffectiveTier) {
	if tier.Unlimited {
		return
	}

	if !b.initialized {
		b.initialized = true
		b.RemainingHour = tier.HourlyLimitBytes
		b.RemainingDay = tier.DailyLimitBytes
		b.RemainingMonth = tier.MonthlyLimitBytes
		b.LastRecoveredAt = now
		b.HourWindowStart = now
		b.DayWindowStart = now
		b.MonthWindowStart = now
		return
	}

	// Rolling recovery enabled
	if tier.RecoveryAmountBytes > 0 && tier.RecoveryIntervalS > 0 {
		recoveryIntervalMs := tier.RecoveryIntervalS * 1000
		if recoveryIntervalMs > 0 && now > b.LastRecoveredAt {
			elapsed := now - b.LastRecoveredAt
			steps := elapsed / recoveryIntervalMs
			if steps > 0 {
				recoveredBytes := steps * tier.RecoveryAmountBytes
				if tier.HourlyEnabled && tier.HourlyLimitBytes > 0 {
					b.RemainingHour = min(tier.HourlyLimitBytes, b.RemainingHour+recoveredBytes)
				}
				if tier.DailyEnabled && tier.DailyLimitBytes > 0 {
					b.RemainingDay = min(tier.DailyLimitBytes, b.RemainingDay+recoveredBytes)
				}
				if tier.MonthlyEnabled && tier.MonthlyLimitBytes > 0 {
					b.RemainingMonth = min(tier.MonthlyLimitBytes, b.RemainingMonth+recoveredBytes)
				}
				b.LastRecoveredAt += steps * recoveryIntervalMs
			}
		}
	} else {
		// Rolling recovery disabled (value is 0) -> fixed rolling window reset
		const hourMs = int64(time.Hour / time.Millisecond)
		const dayMs = int64(24 * time.Hour / time.Millisecond)
		const monthMs = int64(30 * 24 * time.Hour / time.Millisecond)

		if now-b.HourWindowStart >= hourMs {
			b.RemainingHour = tier.HourlyLimitBytes
			b.HourWindowStart = now
		}
		if now-b.DayWindowStart >= dayMs {
			b.RemainingDay = tier.DailyLimitBytes
			b.DayWindowStart = now
		}
		if now-b.MonthWindowStart >= monthMs {
			b.RemainingMonth = tier.MonthlyLimitBytes
			b.MonthWindowStart = now
		}
	}
}

// CheckAndDeductAtomic performs atomic pre-deduction and immediate deduction.
func (b *Bucket) CheckAndDeductAtomic(now int64, tier EffectiveTier, requiredBytes int64) bool {
	if tier.Unlimited {
		return true
	}
	if requiredBytes <= 0 {
		return true
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.updateLazy(now, tier)

	// Pre-deduction check: all enabled tiers must have sufficient remaining quota
	if tier.HourlyEnabled && b.RemainingHour-requiredBytes < 0 {
		return false
	}
	if tier.DailyEnabled && b.RemainingDay-requiredBytes < 0 {
		return false
	}
	if tier.MonthlyEnabled && b.RemainingMonth-requiredBytes < 0 {
		return false
	}

	// Atomic deduction
	if tier.HourlyEnabled {
		b.RemainingHour -= requiredBytes
	}
	if tier.DailyEnabled {
		b.RemainingDay -= requiredBytes
	}
	if tier.MonthlyEnabled {
		b.RemainingMonth -= requiredBytes
	}

	return true
}

// RefundAtomic adds refunded bytes back to the remaining quotas (capped at tier limits).
func (b *Bucket) RefundAtomic(now int64, tier EffectiveTier, refundBytes int64) {
	if tier.Unlimited || refundBytes <= 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.updateLazy(now, tier)

	if tier.HourlyEnabled && tier.HourlyLimitBytes > 0 {
		b.RemainingHour = min(tier.HourlyLimitBytes, b.RemainingHour+refundBytes)
	}
	if tier.DailyEnabled && tier.DailyLimitBytes > 0 {
		b.RemainingDay = min(tier.DailyLimitBytes, b.RemainingDay+refundBytes)
	}
	if tier.MonthlyEnabled && tier.MonthlyLimitBytes > 0 {
		b.RemainingMonth = min(tier.MonthlyLimitBytes, b.RemainingMonth+refundBytes)
	}
}

// Snapshot returns the current remaining and used bytes.
func (b *Bucket) Snapshot(now int64, tier EffectiveTier) (remHour, remDay, remMonth, usedHour, usedDay, usedMonth int64) {
	if tier.Unlimited {
		return 0, 0, 0, 0, 0, 0
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.updateLazy(now, tier)

	remHour = max(0, b.RemainingHour)
	remDay = max(0, b.RemainingDay)
	remMonth = max(0, b.RemainingMonth)

	if tier.HourlyLimitBytes > 0 {
		usedHour = max(0, tier.HourlyLimitBytes-remHour)
	}
	if tier.DailyLimitBytes > 0 {
		usedDay = max(0, tier.DailyLimitBytes-remDay)
	}
	if tier.MonthlyLimitBytes > 0 {
		usedMonth = max(0, tier.MonthlyLimitBytes-remMonth)
	}

	return remHour, remDay, remMonth, usedHour, usedDay, usedMonth
}
