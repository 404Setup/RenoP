/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package traffic implements high-concurrency download rate limiting with rolling recovery windows.
package traffic

import (
	"math"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/llxisdsh/pb"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/auth"
	"renop/internal/utils"
	renopPb "renop/pkg/pb"
)

// PreDeductionErrorMessage is the exact client-facing rejection text for insufficient traffic quota.
const PreDeductionErrorMessage = "Traffic pre-deduction failed: insufficient remaining window quota to complete download"

// TrafficManager coordinates download quota tracking, rolling window recovery, and stream accounting.
type TrafficManager struct {
	state              *core.AppState
	buckets            pb.MapOf[string, *Bucket]
	activeDownloads    pb.MapOf[string, *atomic.Int64]
	revokedBreakpoints pb.MapOf[string, int64]
}

// NewManager creates a TrafficManager instance.
func NewManager(state *core.AppState) *TrafficManager {
	return &TrafficManager{
		state: state,
	}
}

// GetManager retrieves or instantiates the TrafficManager on AppState.
func GetManager(state *core.AppState) *TrafficManager {
	if state == nil || state.Inner == nil {
		return nil
	}
	if tm, ok := state.Inner.TrafficManager.(*TrafficManager); ok && tm != nil {
		return tm
	}
	tm := NewManager(state)
	state.Inner.TrafficManager = tm
	return tm
}

// ExtractSubject extracts the quota subject for the request.
// Authenticated requests (Basic, Bearer, Cookie) resolve to "user:<username>".
// Unauthenticated visitor requests resolve to "ip:<ip>".
func (m *TrafficManager) ExtractSubject(c fiber.Ctx) (string, bool) {
	user := auth.GetUser(c)
	if user != nil && user.Username != "" && !strings.EqualFold(user.Username, "guest") {
		return "user:" + strings.ToLower(user.Username), true
	}
	cfg := m.state.Inner.Config.Load()
	ip := utils.ExtractIP(c, &cfg.Server)
	return "ip:" + ip, false
}

// EffectiveTier resolves configuration and database overrides for a given subject.
func (m *TrafficManager) EffectiveTier(subject string) EffectiveTier {
	cfg := m.state.Inner.Config.Load()
	now := time.Now().UnixMilli()
	_ = now

	if after, ok := strings.CutPrefix(subject, "user:"); ok {
		username := after
		baseTier := cfg.TrafficLimit.Authenticated

		tier := EffectiveTier{
			HourlyLimitBytes:    config.MBToBytes(baseTier.HourlyLimitMB),
			DailyLimitBytes:     config.MBToBytes(baseTier.DailyLimitMB),
			MonthlyLimitBytes:   config.MBToBytes(baseTier.MonthlyLimitMB),
			HourlyEnabled:       baseTier.HourlyEnabled,
			DailyEnabled:        baseTier.DailyEnabled,
			MonthlyEnabled:      baseTier.MonthlyEnabled,
			RecoveryAmountBytes: config.MBToBytes(baseTier.RecoveryAmountMB),
			RecoveryIntervalS:   baseTier.RecoveryIntervalS,
			Unlimited:           baseTier.IsUnlimited(),
		}

		if m.state != nil && m.state.GetDB() != nil {
			override, err := m.state.GetDB().GetTrafficQuotaUserOverride(username)
			if err == nil && override != nil {
				if override.Unlimited {
					tier.Unlimited = true
					return tier
				}
				if override.HourlyLimitBytes >= 0 {
					tier.HourlyLimitBytes = override.HourlyLimitBytes
				}
				if override.DailyLimitBytes >= 0 {
					tier.DailyLimitBytes = override.DailyLimitBytes
				}
				if override.MonthlyLimitBytes >= 0 {
					tier.MonthlyLimitBytes = override.MonthlyLimitBytes
				}
				if override.HourlyEnabled >= 0 {
					tier.HourlyEnabled = override.HourlyEnabled == 1
				}
				if override.DailyEnabled >= 0 {
					tier.DailyEnabled = override.DailyEnabled == 1
				}
				if override.MonthlyEnabled >= 0 {
					tier.MonthlyEnabled = override.MonthlyEnabled == 1
				}
				if override.RecoveryAmountBytes >= 0 {
					tier.RecoveryAmountBytes = override.RecoveryAmountBytes
				}
				if override.RecoveryIntervalS >= 0 {
					tier.RecoveryIntervalS = override.RecoveryIntervalS
				}
				tier.Unlimited = !tier.HourlyEnabled && !tier.DailyEnabled && !tier.MonthlyEnabled
			}
		}
		return tier
	}

	// Unauthenticated tier
	baseTier := cfg.TrafficLimit.Unauthenticated
	return EffectiveTier{
		HourlyLimitBytes:    config.MBToBytes(baseTier.HourlyLimitMB),
		DailyLimitBytes:     config.MBToBytes(baseTier.DailyLimitMB),
		MonthlyLimitBytes:   config.MBToBytes(baseTier.MonthlyLimitMB),
		HourlyEnabled:       baseTier.HourlyEnabled,
		DailyEnabled:        baseTier.DailyEnabled,
		MonthlyEnabled:      baseTier.MonthlyEnabled,
		RecoveryAmountBytes: config.MBToBytes(baseTier.RecoveryAmountMB),
		RecoveryIntervalS:   baseTier.RecoveryIntervalS,
		Unlimited:           baseTier.IsUnlimited(),
	}
}

func (m *TrafficManager) getOrCreateBucket(subject string) *Bucket {
	if b, ok := m.buckets.Load(subject); ok && b != nil {
		return b
	}
	b := &Bucket{}
	actual, _ := m.buckets.LoadOrStore(subject, b)
	return actual
}

func (m *TrafficManager) activeDownloadKey(subject, fileKey string) string {
	return subject + ":" + fileKey
}

func (m *TrafficManager) incrementActiveDownload(subject, fileKey string) int64 {
	key := m.activeDownloadKey(subject, fileKey)
	counter, _ := m.activeDownloads.LoadOrStore(key, &atomic.Int64{})
	return counter.Add(1)
}

func (m *TrafficManager) decrementActiveDownload(subject, fileKey string) {
	key := m.activeDownloadKey(subject, fileKey)
	if counter, ok := m.activeDownloads.Load(key); ok {
		if counter.Add(-1) <= 0 {
			m.activeDownloads.Delete(key)
		}
	}
}

// IsBreakpointRevoked checks whether breakpoint resumption was revoked due to an aborted download.
func (m *TrafficManager) IsBreakpointRevoked(subject, fileKey string) bool {
	key := m.activeDownloadKey(subject, fileKey)
	_, revoked := m.revokedBreakpoints.Load(key)
	return revoked
}

// RevokeBreakpointResume marks breakpoint resumption as revoked for a subject and file.
func (m *TrafficManager) RevokeBreakpointResume(subject, fileKey string) {
	key := m.activeDownloadKey(subject, fileKey)
	m.revokedBreakpoints.Store(key, time.Now().UnixMilli())
}

// ClearBreakpointRevocation clears any breakpoint revocation when restarting download from byte 0.
func (m *TrafficManager) ClearBreakpointRevocation(subject, fileKey string) {
	key := m.activeDownloadKey(subject, fileKey)
	m.revokedBreakpoints.Delete(key)
}

// Refund atomically adds refunded bytes back to the quota pool.
func (m *TrafficManager) Refund(subject string, refundBytes int64) {
	if refundBytes <= 0 {
		return
	}
	bucket := m.getOrCreateBucket(subject)
	tier := m.EffectiveTier(subject)
	bucket.RefundAtomic(time.Now().UnixMilli(), tier, refundBytes)
}

// CheckAndDeduct performs pre-deduction and download session initialization for a download request.
func (m *TrafficManager) CheckAndDeduct(c fiber.Ctx, requestedSize, totalFileSize int64, fileKey string, isS3Redirect bool) (*DownloadSession, error) {
	if totalFileSize <= 0 {
		totalFileSize = requestedSize
	}
	if requestedSize <= 0 {
		return &DownloadSession{
			Manager: m, FileKey: fileKey, FileSize: totalFileSize, RequestedSize: requestedSize, IsS3Redirect: isS3Redirect,
		}, nil
	}

	subject, _ := m.ExtractSubject(c)
	tier := m.EffectiveTier(subject)
	if tier.Unlimited {
		return &DownloadSession{
			Manager: m, Subject: subject, FileKey: fileKey, FileSize: totalFileSize, RequestedSize: requestedSize, IsS3Redirect: isS3Redirect,
		}, nil
	}

	cfg := m.state.Inner.Config.Load()
	now := time.Now().UnixMilli()

	isMultiThread := false
	multiplier := 1.0

	// Multi-thread detection applies when storage engine is NOT S3-Redirect
	if !isS3Redirect {
		currentActive := m.incrementActiveDownload(subject, fileKey)
		if currentActive > 1 {
			isMultiThread = true
			if cfg.TrafficLimit.MultiThreadMultiplier > 0 {
				multiplier = cfg.TrafficLimit.MultiThreadMultiplier
			} else {
				multiplier = config.DefaultMultiThreadMultiplier
			}
		}
	}

	deductAmount := int64(math.Ceil(float64(requestedSize) * multiplier))

	bucket := m.getOrCreateBucket(subject)
	if !bucket.CheckAndDeductAtomic(now, tier, deductAmount) {
		if !isS3Redirect {
			m.decrementActiveDownload(subject, fileKey)
		}
		c.Set("X-Renop-Error-Code", "traffic_quota_exceeded")
		return nil, fiber.NewError(fiber.StatusTooManyRequests, PreDeductionErrorMessage)
	}

	return &DownloadSession{
		Manager:        m,
		Subject:        subject,
		FileKey:        fileKey,
		FileSize:       totalFileSize,
		RequestedSize:  requestedSize,
		DeductedAmount: deductAmount,
		Multiplier:     multiplier,
		IsMultiThread:  isMultiThread,
		IsS3Redirect:   isS3Redirect,
	}, nil
}

// GetStatus returns the current quota status for an IP or User.
func (m *TrafficManager) GetStatus(subject string) *renopPb.TrafficQuotaStatus {
	tier := m.EffectiveTier(subject)
	cfg := m.state.Inner.Config.Load()
	now := time.Now().UnixMilli()

	subjectType := "ip"
	subjectKey := strings.TrimPrefix(subject, "ip:")
	if strings.HasPrefix(subject, "user:") {
		subjectType = "user"
		subjectKey = strings.TrimPrefix(subject, "user:")
	}

	multiplier := cfg.TrafficLimit.MultiThreadMultiplier
	if multiplier <= 0 {
		multiplier = config.DefaultMultiThreadMultiplier
	}

	if tier.Unlimited {
		return &renopPb.TrafficQuotaStatus{
			SubjectType:           subjectType,
			SubjectKey:            subjectKey,
			Unlimited:             true,
			Hourly:                &renopPb.TrafficTierDto{Enabled: false},
			Daily:                 &renopPb.TrafficTierDto{Enabled: false},
			Monthly:               &renopPb.TrafficTierDto{Enabled: false},
			RecoveryAmountMb:      config.BytesToMB(tier.RecoveryAmountBytes),
			RecoveryIntervalS:     tier.RecoveryIntervalS,
			MultiThreadMultiplier: multiplier,
		}
	}

	bucket := m.getOrCreateBucket(subject)
	remHour, remDay, remMonth, usedHour, usedDay, usedMonth := bucket.Snapshot(now, tier)

	return &renopPb.TrafficQuotaStatus{
		SubjectType: subjectType,
		SubjectKey:  subjectKey,
		Unlimited:   false,
		Hourly: &renopPb.TrafficTierDto{
			LimitMb:     config.BytesToMB(tier.HourlyLimitBytes),
			RemainingMb: config.BytesToMB(remHour),
			UsedMb:      config.BytesToMB(usedHour),
			Enabled:     tier.HourlyEnabled,
		},
		Daily: &renopPb.TrafficTierDto{
			LimitMb:     config.BytesToMB(tier.DailyLimitBytes),
			RemainingMb: config.BytesToMB(remDay),
			UsedMb:      config.BytesToMB(usedDay),
			Enabled:     tier.DailyEnabled,
		},
		Monthly: &renopPb.TrafficTierDto{
			LimitMb:     config.BytesToMB(tier.MonthlyLimitBytes),
			RemainingMb: config.BytesToMB(remMonth),
			UsedMb:      config.BytesToMB(usedMonth),
			Enabled:     tier.MonthlyEnabled,
		},
		RecoveryAmountMb:      config.BytesToMB(tier.RecoveryAmountBytes),
		RecoveryIntervalS:     tier.RecoveryIntervalS,
		MultiThreadMultiplier: multiplier,
	}
}

// SetUserOverride updates a user's quota overrides in the database.
func (m *TrafficManager) SetUserOverride(username string, req *renopPb.TrafficQuotaUserOverrideRequest) error {
	if m.state == nil || m.state.GetDB() == nil {
		return core.ErrDatabaseUnavailable
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return core.ErrUserProfileNotFound
	}

	override := &core.TrafficQuotaUserOverride{
		Username:            username,
		HourlyLimitBytes:    -1,
		DailyLimitBytes:     -1,
		MonthlyLimitBytes:   -1,
		HourlyEnabled:       -1,
		DailyEnabled:        -1,
		MonthlyEnabled:      -1,
		RecoveryAmountBytes: -1,
		RecoveryIntervalS:   -1,
		Unlimited:           req.GetUnlimited(),
		UpdatedAt:           time.Now().UnixMilli(),
	}

	if req.HourlyLimitMb != nil {
		override.HourlyLimitBytes = config.MBToBytes(*req.HourlyLimitMb)
	}
	if req.DailyLimitMb != nil {
		override.DailyLimitBytes = config.MBToBytes(*req.DailyLimitMb)
	}
	if req.MonthlyLimitMb != nil {
		override.MonthlyLimitBytes = config.MBToBytes(*req.MonthlyLimitMb)
	}
	if req.HourlyEnabled != nil {
		if *req.HourlyEnabled {
			override.HourlyEnabled = 1
		} else {
			override.HourlyEnabled = 0
		}
	}
	if req.DailyEnabled != nil {
		if *req.DailyEnabled {
			override.DailyEnabled = 1
		} else {
			override.DailyEnabled = 0
		}
	}
	if req.MonthlyEnabled != nil {
		if *req.MonthlyEnabled {
			override.MonthlyEnabled = 1
		} else {
			override.MonthlyEnabled = 0
		}
	}
	if req.RecoveryAmountMb != nil {
		override.RecoveryAmountBytes = config.MBToBytes(*req.RecoveryAmountMb)
	}
	if req.RecoveryIntervalS != nil {
		override.RecoveryIntervalS = *req.RecoveryIntervalS
	}

	err := m.state.GetDB().SetTrafficQuotaUserOverride(username, override, time.Now().UnixMilli())
	if err == nil {
		// Invalidate bucket state so changes take effect immediately
		m.buckets.Delete("user:" + username)
	}
	return err
}
