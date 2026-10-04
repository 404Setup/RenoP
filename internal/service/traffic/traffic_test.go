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
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/database"
	"renop/internal/testutil"
	"renop/internal/utils/protohttp"
	renopPb "renop/pkg/pb"
)

func dummyCtx(app *fiber.App) fiber.Ctx {
	fastCtx := &fasthttp.RequestCtx{}
	return app.AcquireCtx(fastCtx)
}

func setupTestApp(t *testing.T) (*fiber.App, *core.AppState) {
	t.Helper()
	dbPath := filepath.Join(testutil.TempDir(t), "traffic-test.db")
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: "sqlite", Dsn: dbPath, MaxOpenConns: 1, MaxIdleConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	state := core.NewAppState()
	state.Inner.DB = db
	cfg := config.DefaultConfig()
	cfg.Server.TrustedProxies = []string{"0.0.0.0"}
	cfg.Server.ParseTrustedProxies()
	state.Inner.Config.Store(cfg)

	app := fiber.New(fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
	})

	app.Use(func(c fiber.Ctx) error {
		testUser := c.Get("X-Test-User")
		if testUser == "admin" {
			c.Locals("user", &config.User{Username: "admin", Roles: []string{"base", "manager"}})
		} else if testUser != "" {
			c.Locals("user", &config.User{Username: testUser, Roles: []string{"base"}})
		}
		return c.Next()
	})

	apiGroup := app.Group("/api")
	SetupRoutes(apiGroup, state)

	return app, state
}

func TestBucket_AtomicDeductAndWindow(t *testing.T) {
	bucket := &Bucket{}
	tier := EffectiveTier{
		HourlyLimitBytes:    1000,
		DailyLimitBytes:     5000,
		MonthlyLimitBytes:   20000,
		HourlyEnabled:       true,
		DailyEnabled:        true,
		MonthlyEnabled:      true,
		RecoveryAmountBytes: 100,
		RecoveryIntervalS:   60,
	}

	now := time.Now().UnixMilli()
	// Initial deduction of 400 bytes should succeed
	ok := bucket.CheckAndDeductAtomic(now, tier, 400)
	assert.True(t, ok)
	assert.Equal(t, int64(600), bucket.RemainingHour)
	assert.Equal(t, int64(4600), bucket.RemainingDay)
	assert.Equal(t, int64(19600), bucket.RemainingMonth)

	// Deducting 700 bytes exceeds remaining 600 bytes in hourly window: should fail and not mutate
	ok = bucket.CheckAndDeductAtomic(now, tier, 700)
	assert.False(t, ok)
	assert.Equal(t, int64(600), bucket.RemainingHour)

	// Deducting remaining 600 bytes exactly should succeed
	ok = bucket.CheckAndDeductAtomic(now, tier, 600)
	assert.True(t, ok)
	assert.Equal(t, int64(0), bucket.RemainingHour)

	// Another 1 byte should fail
	ok = bucket.CheckAndDeductAtomic(now, tier, 1)
	assert.False(t, ok)
}

func TestBucket_RollingRecovery(t *testing.T) {
	bucket := &Bucket{}
	tier := EffectiveTier{
		HourlyLimitBytes:    1000,
		DailyLimitBytes:     5000,
		MonthlyLimitBytes:   20000,
		HourlyEnabled:       true,
		DailyEnabled:        true,
		MonthlyEnabled:      true,
		RecoveryAmountBytes: 200,
		RecoveryIntervalS:   10, // 200 bytes every 10s
	}

	now := int64(10000000)
	ok := bucket.CheckAndDeductAtomic(now, tier, 800)
	assert.True(t, ok)
	assert.Equal(t, int64(200), bucket.RemainingHour)

	// 5 seconds later: interval (10s) not reached yet -> no recovery
	ok = bucket.CheckAndDeductAtomic(now+5000, tier, 300)
	assert.False(t, ok) // only 200 remaining

	// 10 seconds later: recovered 200 bytes -> 400 available
	ok = bucket.CheckAndDeductAtomic(now+10000, tier, 350)
	assert.True(t, ok) // 400 - 350 = 50 remaining
	assert.Equal(t, int64(50), bucket.RemainingHour)

	// 50 seconds later: recovered 5 * 200 = 1000 bytes, but capped at HourlyLimitBytes (1000)
	remHour, _, _, _, _, _ := bucket.Snapshot(now+60000, tier)
	assert.Equal(t, int64(1000), remHour)
}

func TestBucket_FixedWindowResetWhenRecoveryDisabled(t *testing.T) {
	bucket := &Bucket{}
	tier := EffectiveTier{
		HourlyLimitBytes:    1000,
		DailyLimitBytes:     5000,
		MonthlyLimitBytes:   20000,
		HourlyEnabled:       true,
		DailyEnabled:        true,
		MonthlyEnabled:      true,
		RecoveryAmountBytes: 0, // Disabled rolling recovery
		RecoveryIntervalS:   0,
	}

	now := int64(10000000)
	ok := bucket.CheckAndDeductAtomic(now, tier, 1000)
	assert.True(t, ok)
	assert.Equal(t, int64(0), bucket.RemainingHour)

	// After 30 minutes, not expired
	ok = bucket.CheckAndDeductAtomic(now+30*60*1000, tier, 100)
	assert.False(t, ok)

	// After 1 hour (3600000ms), hourly window resets to 1000
	ok = bucket.CheckAndDeductAtomic(now+3600*1000+1, tier, 500)
	assert.True(t, ok)
	assert.Equal(t, int64(500), bucket.RemainingHour)
}

func TestDownloadSession_StreamingRefundUnder70Percent(t *testing.T) {
	app, state := setupTestApp(t)
	tm := GetManager(state)

	totalFileSize := int64(10000)
	requestedSize := int64(10000)

	// Initial quota deduction for subject
	session, err := tm.CheckAndDeduct(dummyCtx(app), requestedSize, totalFileSize, "file123", false)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Track initial remaining
	tier := tm.EffectiveTier(session.Subject)
	bucket := tm.getOrCreateBucket(session.Subject)
	remBefore, _, _, _, _, _ := bucket.Snapshot(time.Now().UnixMilli(), tier)

	// Read only 5000 bytes (50% <= 70%)
	dummyData := make([]byte, 5000)
	rc := session.WrapReadCloser(io.NopCloser(bytes.NewReader(dummyData)))
	buf := make([]byte, 1024)
	for {
		n, rErr := rc.Read(buf)
		if rErr != nil || n == 0 {
			break
		}
	}
	require.NoError(t, rc.Close())

	// Unused quota: 10000 - 5000 = 5000 bytes.
	// Refund: 80% of 5000 = 4000 bytes.
	remAfter, _, _, _, _, _ := bucket.Snapshot(time.Now().UnixMilli(), tier)
	assert.Equal(t, remBefore+4000, remAfter)

	// Breakpoint resumption should be revoked!
	assert.True(t, tm.IsBreakpointRevoked(session.Subject, "file123"))

	// Starting download from byte 0 clears revocation
	tm.ClearBreakpointRevocation(session.Subject, "file123")
	assert.False(t, tm.IsBreakpointRevoked(session.Subject, "file123"))
}

func TestDownloadSession_NoRefundOver70Percent(t *testing.T) {
	app, state := setupTestApp(t)
	tm := GetManager(state)

	totalFileSize := int64(10000)
	requestedSize := int64(10000)

	session, err := tm.CheckAndDeduct(dummyCtx(app), requestedSize, totalFileSize, "file456", false)
	require.NoError(t, err)

	tier := tm.EffectiveTier(session.Subject)
	bucket := tm.getOrCreateBucket(session.Subject)
	remBefore, _, _, _, _, _ := bucket.Snapshot(time.Now().UnixMilli(), tier)

	// Read 7500 bytes (75% > 70%)
	dummyData := make([]byte, 7500)
	rc := session.WrapReadCloser(io.NopCloser(bytes.NewReader(dummyData)))
	buf := make([]byte, 1024)
	for {
		n, rErr := rc.Read(buf)
		if rErr != nil || n == 0 {
			break
		}
	}
	require.NoError(t, rc.Close())

	// No refund issued!
	remAfter, _, _, _, _, _ := bucket.Snapshot(time.Now().UnixMilli(), tier)
	assert.Equal(t, remBefore, remAfter)
	assert.False(t, tm.IsBreakpointRevoked(session.Subject, "file456"))
}

func TestMultiThread_Multiplier(t *testing.T) {
	app, state := setupTestApp(t)
	tm := GetManager(state)

	// First thread starts download
	sess1, err := tm.CheckAndDeduct(dummyCtx(app), 1000, 1000, "multi_test_file", false)
	require.NoError(t, err)
	assert.False(t, sess1.IsMultiThread)
	assert.Equal(t, float64(1.0), sess1.Multiplier)
	assert.Equal(t, int64(1000), sess1.DeductedAmount)

	// Second concurrent thread for the SAME file and SAME subject
	sess2, err := tm.CheckAndDeduct(dummyCtx(app), 1000, 1000, "multi_test_file", false)
	require.NoError(t, err)
	assert.True(t, sess2.IsMultiThread)
	assert.Equal(t, 1.25, sess2.Multiplier)
	assert.Equal(t, int64(1250), sess2.DeductedAmount)

	// Third download with S3-Redirect bypasses multi-thread multiplier
	sess3, err := tm.CheckAndDeduct(dummyCtx(app), 1000, 1000, "multi_test_file", true)
	require.NoError(t, err)
	assert.False(t, sess3.IsMultiThread)
	assert.Equal(t, float64(1.0), sess3.Multiplier)
	assert.Equal(t, int64(1000), sess3.DeductedAmount)

	// Close streams to clean up active download counters
	sess1.onStreamFinish()
	sess2.onStreamFinish()
}

func TestTrafficManager_PreDeductionFailure(t *testing.T) {
	app, state := setupTestApp(t)
	tm := GetManager(state)

	// Default unauthenticated hourly quota is 1GB (1073741824 bytes)
	// Attempt to download 2GB (> 1GB)
	twoGB := int64(2 * 1024 * 1024 * 1024)
	sess, err := tm.CheckAndDeduct(dummyCtx(app), twoGB, twoGB, "bigfile", false)
	assert.Error(t, err)
	assert.Nil(t, sess)
	assert.Contains(t, err.Error(), PreDeductionErrorMessage)
}

func TestTrafficRoutes_PublicAndUser(t *testing.T) {
	app, _ := setupTestApp(t)

	// 1. Unauthenticated GET /api/traffic-quota -> returns IP quota
	req := httptest.NewRequest(http.MethodGet, "/api/traffic-quota", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var ipStatus renopPb.TrafficQuotaStatus
	err = json.NewDecoder(resp.Body).Decode(&ipStatus)
	require.NoError(t, err)
	assert.Equal(t, "ip", ipStatus.SubjectType)
	assert.Equal(t, "192.168.1.100", ipStatus.SubjectKey)
	assert.InDelta(t, 1024.0, ipStatus.Hourly.LimitMb, 0.001)    // 1GB
	assert.InDelta(t, 6144.0, ipStatus.Daily.LimitMb, 0.001)     // 6GB
	assert.InDelta(t, 122880.0, ipStatus.Monthly.LimitMb, 0.001) // 120GB
	assert.InDelta(t, 300.0, ipStatus.RecoveryAmountMb, 0.001)   // 300MB

	// 2. Authenticated GET /api/traffic-quota -> returns User quota
	reqAuth := httptest.NewRequest(http.MethodGet, "/api/traffic-quota", nil)
	reqAuth.Header.Set("X-Test-User", "bob")
	respAuth, err := app.Test(reqAuth)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respAuth.StatusCode)

	var userStatus renopPb.TrafficQuotaStatus
	err = json.NewDecoder(respAuth.Body).Decode(&userStatus)
	require.NoError(t, err)
	assert.Equal(t, "user", userStatus.SubjectType)
	assert.Equal(t, "bob", userStatus.SubjectKey)
	assert.InDelta(t, 5120.0, userStatus.Hourly.LimitMb, 0.001)    // 5GB
	assert.InDelta(t, 35840.0, userStatus.Daily.LimitMb, 0.001)    // 35GB
	assert.InDelta(t, 614400.0, userStatus.Monthly.LimitMb, 0.001) // 600GB
	assert.InDelta(t, 2048.0, userStatus.RecoveryAmountMb, 0.001)  // 2GB
}

func TestTrafficRoutes_Permissions(t *testing.T) {
	app, _ := setupTestApp(t)

	// 1. Bob viewing Alice's quota -> Forbidden 403
	req := httptest.NewRequest(http.MethodGet, "/api/traffic-quota/users/alice", nil)
	req.Header.Set("X-Test-User", "bob")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	// 2. Alice viewing her own quota -> OK 200
	reqSelf := httptest.NewRequest(http.MethodGet, "/api/traffic-quota/users/alice", nil)
	reqSelf.Header.Set("X-Test-User", "alice")
	respSelf, err := app.Test(reqSelf)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respSelf.StatusCode)

	// 3. Admin viewing Alice's quota -> OK 200
	reqAdmin := httptest.NewRequest(http.MethodGet, "/api/traffic-quota/users/alice", nil)
	reqAdmin.Header.Set("X-Test-User", "admin")
	respAdmin, err := app.Test(reqAdmin)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respAdmin.StatusCode)

	// 4. Bob viewing IP quota -> Forbidden 403
	reqIPBob := httptest.NewRequest(http.MethodGet, "/api/traffic-quota/ips/1.2.3.4", nil)
	reqIPBob.Header.Set("X-Test-User", "bob")
	respIPBob, err := app.Test(reqIPBob)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respIPBob.StatusCode)

	// 5. Admin viewing IP quota -> OK 200
	reqIPAdmin := httptest.NewRequest(http.MethodGet, "/api/traffic-quota/ips/1.2.3.4", nil)
	reqIPAdmin.Header.Set("X-Test-User", "admin")
	respIPAdmin, err := app.Test(reqIPAdmin)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respIPAdmin.StatusCode)
}

func TestTrafficRoutes_AdminOverrideUserQuota(t *testing.T) {
	app, _ := setupTestApp(t)

	// Non-admin attempting override -> 403
	overrideReq := renopPb.TrafficQuotaUserOverrideRequest{
		HourlyLimitMb: new(10240.0), // 10GB
	}
	body, _ := json.MarshalOf(&overrideReq)
	reqBob := httptest.NewRequest(http.MethodPut, "/api/traffic-quota/users/alice", bytes.NewReader(body))
	reqBob.Header.Set("Content-Type", "application/json")
	reqBob.Header.Set("X-Test-User", "bob")
	respBob, err := app.Test(reqBob)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, respBob.StatusCode)

	// Admin setting override -> 200
	reqAdmin := httptest.NewRequest(http.MethodPut, "/api/traffic-quota/users/alice", bytes.NewReader(body))
	reqAdmin.Header.Set("Content-Type", "application/json")
	reqAdmin.Header.Set("X-Test-User", "admin")
	respAdmin, err := app.Test(reqAdmin)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respAdmin.StatusCode)

	var updated renopPb.TrafficQuotaStatus
	err = json.NewDecoder(respAdmin.Body).Decode(&updated)
	require.NoError(t, err)
	assert.InDelta(t, 10240.0, updated.Hourly.LimitMb, 0.001)

	// Verify Alice now sees updated limit
	reqAlice := httptest.NewRequest(http.MethodGet, "/api/traffic-quota/users/alice", nil)
	reqAlice.Header.Set("X-Test-User", "alice")
	respAlice, err := app.Test(reqAlice)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, respAlice.StatusCode)
	var aliceStatus renopPb.TrafficQuotaStatus
	require.NoError(t, json.NewDecoder(respAlice.Body).Decode(&aliceStatus))
	assert.InDelta(t, 10240.0, aliceStatus.Hourly.LimitMb, 0.001)
}

func TestTrafficRoutes_ProtobufAccept(t *testing.T) {
	app, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/traffic-quota", nil)
	req.Header.Set("Accept", protohttp.ContentType)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, strings.Contains(resp.Header.Get("Content-Type"), protohttp.ContentType))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var status renopPb.TrafficQuotaStatus
	require.NoError(t, proto.Unmarshal(body, &status))
	assert.Equal(t, "ip", status.SubjectType)
	assert.Equal(t, "10.0.0.1", status.SubjectKey)
}
