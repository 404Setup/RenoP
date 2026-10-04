/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package config

import "math"

const (
	DefaultUnauthenticatedHourlyMB  float64 = 1024.0   // 1 GB
	DefaultUnauthenticatedDailyMB   float64 = 6144.0   // 6 GB
	DefaultUnauthenticatedMonthlyMB float64 = 122880.0 // 120 GB
	DefaultUnauthenticatedRecoverMB float64 = 300.0    // 300 MB / hour
	DefaultUnauthenticatedRecoverS  int64   = 3600     // 1 hour

	DefaultAuthenticatedHourlyMB  float64 = 5120.0   // 5 GB
	DefaultAuthenticatedDailyMB   float64 = 35840.0  // 35 GB
	DefaultAuthenticatedMonthlyMB float64 = 614400.0 // 600 GB
	DefaultAuthenticatedRecoverMB float64 = 2048.0   // 2 GB / hour
	DefaultAuthenticatedRecoverS  int64   = 3600     // 1 hour

	DefaultMultiThreadMultiplier float64 = 1.25
)

// RoundMB rounds an MB value to 3 decimal places (KB resolution).
func RoundMB(mb float64) float64 {
	return math.Round(mb*1000.0) / 1000.0
}

// MBToBytes converts an MB value (rounded to 3 decimals) to exact bytes.
func MBToBytes(mb float64) int64 {
	rounded := RoundMB(mb)
	if rounded <= 0 {
		return 0
	}
	return int64(math.Round(rounded * 1024.0 * 1024.0))
}

// BytesToMB converts byte count to MB rounded to 3 decimal places.
func BytesToMB(b int64) float64 {
	if b <= 0 {
		return 0
	}
	mb := float64(b) / (1024.0 * 1024.0)
	return RoundMB(mb)
}

// TrafficQuotaTier configures download limits and rolling recovery for a client tier.
type TrafficQuotaTier struct {
	HourlyLimitMB     float64 `json:"hourly_limit_mb" yaml:"hourly_limit_mb"`
	DailyLimitMB      float64 `json:"daily_limit_mb" yaml:"daily_limit_mb"`
	MonthlyLimitMB    float64 `json:"monthly_limit_mb" yaml:"monthly_limit_mb"`
	HourlyEnabled     bool    `json:"hourly_enabled" yaml:"hourly_enabled"`
	DailyEnabled      bool    `json:"daily_enabled" yaml:"daily_enabled"`
	MonthlyEnabled    bool    `json:"monthly_enabled" yaml:"monthly_enabled"`
	RecoveryAmountMB  float64 `json:"recovery_amount_mb" yaml:"recovery_amount_mb"`
	RecoveryIntervalS int64   `json:"recovery_interval_s" yaml:"recovery_interval_s"`
}

// IsUnlimited returns true if all 3 tiers are disabled.
func (t *TrafficQuotaTier) IsUnlimited() bool {
	return !t.HourlyEnabled && !t.DailyEnabled && !t.MonthlyEnabled
}

// DeepCopy returns an independent copy of TrafficQuotaTier.
func (t TrafficQuotaTier) DeepCopy() TrafficQuotaTier {
	return t
}

// TrafficLimitConfig governs repository download rate limiting.
type TrafficLimitConfig struct {
	Unauthenticated       TrafficQuotaTier `json:"unauthenticated" yaml:"unauthenticated"`
	Authenticated         TrafficQuotaTier `json:"authenticated" yaml:"authenticated"`
	MultiThreadMultiplier float64          `json:"multi_thread_multiplier" yaml:"multi_thread_multiplier"`
}

func (c *TrafficLimitConfig) setDefaults() {
	if c.MultiThreadMultiplier <= 0 {
		c.MultiThreadMultiplier = DefaultMultiThreadMultiplier
	}

	// Unauthenticated defaults
	if c.Unauthenticated.HourlyLimitMB <= 0 && !c.Unauthenticated.HourlyEnabled {
		c.Unauthenticated.HourlyLimitMB = DefaultUnauthenticatedHourlyMB
		c.Unauthenticated.HourlyEnabled = true
	}
	if c.Unauthenticated.DailyLimitMB <= 0 && !c.Unauthenticated.DailyEnabled {
		c.Unauthenticated.DailyLimitMB = DefaultUnauthenticatedDailyMB
		c.Unauthenticated.DailyEnabled = true
	}
	if c.Unauthenticated.MonthlyLimitMB <= 0 && !c.Unauthenticated.MonthlyEnabled {
		c.Unauthenticated.MonthlyLimitMB = DefaultUnauthenticatedMonthlyMB
		c.Unauthenticated.MonthlyEnabled = true
	}
	if c.Unauthenticated.RecoveryIntervalS <= 0 {
		c.Unauthenticated.RecoveryIntervalS = DefaultUnauthenticatedRecoverS
	}
	if c.Unauthenticated.RecoveryAmountMB <= 0 && c.Unauthenticated.RecoveryAmountMB != -1 {
		c.Unauthenticated.RecoveryAmountMB = DefaultUnauthenticatedRecoverMB
	}
	if c.Unauthenticated.RecoveryAmountMB == -1 {
		c.Unauthenticated.RecoveryAmountMB = 0 // explicitly disabled
	}

	// Authenticated defaults
	if c.Authenticated.HourlyLimitMB <= 0 && !c.Authenticated.HourlyEnabled {
		c.Authenticated.HourlyLimitMB = DefaultAuthenticatedHourlyMB
		c.Authenticated.HourlyEnabled = true
	}
	if c.Authenticated.DailyLimitMB <= 0 && !c.Authenticated.DailyEnabled {
		c.Authenticated.DailyLimitMB = DefaultAuthenticatedDailyMB
		c.Authenticated.DailyEnabled = true
	}
	if c.Authenticated.MonthlyLimitMB <= 0 && !c.Authenticated.MonthlyEnabled {
		c.Authenticated.MonthlyLimitMB = DefaultAuthenticatedMonthlyMB
		c.Authenticated.MonthlyEnabled = true
	}
	if c.Authenticated.RecoveryIntervalS <= 0 {
		c.Authenticated.RecoveryIntervalS = DefaultAuthenticatedRecoverS
	}
	if c.Authenticated.RecoveryAmountMB <= 0 && c.Authenticated.RecoveryAmountMB != -1 {
		c.Authenticated.RecoveryAmountMB = DefaultAuthenticatedRecoverMB
	}
	if c.Authenticated.RecoveryAmountMB == -1 {
		c.Authenticated.RecoveryAmountMB = 0 // explicitly disabled
	}

	c.Unauthenticated.HourlyLimitMB = RoundMB(c.Unauthenticated.HourlyLimitMB)
	c.Unauthenticated.DailyLimitMB = RoundMB(c.Unauthenticated.DailyLimitMB)
	c.Unauthenticated.MonthlyLimitMB = RoundMB(c.Unauthenticated.MonthlyLimitMB)
	c.Unauthenticated.RecoveryAmountMB = RoundMB(c.Unauthenticated.RecoveryAmountMB)

	c.Authenticated.HourlyLimitMB = RoundMB(c.Authenticated.HourlyLimitMB)
	c.Authenticated.DailyLimitMB = RoundMB(c.Authenticated.DailyLimitMB)
	c.Authenticated.MonthlyLimitMB = RoundMB(c.Authenticated.MonthlyLimitMB)
	c.Authenticated.RecoveryAmountMB = RoundMB(c.Authenticated.RecoveryAmountMB)
}

// DeepCopy returns an independent copy of TrafficLimitConfig.
func (c TrafficLimitConfig) DeepCopy() TrafficLimitConfig {
	return c
}

// DefaultTrafficLimitConfig returns the standard traffic limiting settings.
func DefaultTrafficLimitConfig() TrafficLimitConfig {
	cfg := TrafficLimitConfig{
		Unauthenticated: TrafficQuotaTier{
			HourlyLimitMB:     DefaultUnauthenticatedHourlyMB,
			DailyLimitMB:      DefaultUnauthenticatedDailyMB,
			MonthlyLimitMB:    DefaultUnauthenticatedMonthlyMB,
			HourlyEnabled:     true,
			DailyEnabled:      true,
			MonthlyEnabled:    true,
			RecoveryAmountMB:  DefaultUnauthenticatedRecoverMB,
			RecoveryIntervalS: DefaultUnauthenticatedRecoverS,
		},
		Authenticated: TrafficQuotaTier{
			HourlyLimitMB:     DefaultAuthenticatedHourlyMB,
			DailyLimitMB:      DefaultAuthenticatedDailyMB,
			MonthlyLimitMB:    DefaultAuthenticatedMonthlyMB,
			HourlyEnabled:     true,
			DailyEnabled:      true,
			MonthlyEnabled:    true,
			RecoveryAmountMB:  DefaultAuthenticatedRecoverMB,
			RecoveryIntervalS: DefaultAuthenticatedRecoverS,
		},
		MultiThreadMultiplier: DefaultMultiThreadMultiplier,
	}
	return cfg
}
