/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package legal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"
)

func TestConsentRejectsMissingAndStalePolicies(t *testing.T) {
	cfg := config.DefaultConfig()
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		if err := RequireConsent(c, state); err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	for _, revision := range []string{"", "stale", cfg.Legal.Revision()} {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.Header.Set(ConsentHeader, revision)
		response, err := app.Test(request)
		require.NoError(t, err)
		if revision == cfg.Legal.Revision() {
			require.Equal(t, http.StatusNoContent, response.StatusCode)
		} else {
			require.Equal(t, http.StatusPreconditionRequired, response.StatusCode)
			require.Equal(t, ConsentErrorCode, response.Header.Get("X-Renop-Error-Code"))
		}
		require.NoError(t, response.Body.Close())
	}
	oldRevision := cfg.Legal.Revision()
	next := cfg.DeepCopy()
	next.Legal.TermsOfService = "# New terms"
	require.NoError(t, next.Legal.Normalize())
	state.Inner.Config.Store(next)
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(&http.Cookie{Name: ConsentCookie, Value: oldRevision})
	response, err := app.Test(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusPreconditionRequired, response.StatusCode)
	require.NoError(t, response.Body.Close())
	request = httptest.NewRequest(http.MethodPost, "/", nil)
	request.AddCookie(&http.Cookie{Name: ConsentCookie, Value: next.Legal.Revision()})
	response, err = app.Test(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.StatusCode)
	require.NoError(t, response.Body.Close())
}

func TestServeDocumentETagAndCaching(t *testing.T) {
	cfg := config.DefaultConfig()
	state := core.NewAppState()
	state.Inner.Config.Store(cfg)
	app := fiber.New()
	SetupRoutes(app, state)

	req := httptest.NewRequest(http.MethodGet, "/legal/privacy-policy", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	etag := resp.Header.Get(fiber.HeaderETag)
	require.NotEmpty(t, etag)
	require.Contains(t, resp.Header.Get(fiber.HeaderCacheControl), "public")
	require.NoError(t, resp.Body.Close())

	reqConditional := httptest.NewRequest(http.MethodGet, "/legal/privacy-policy", nil)
	reqConditional.Header.Set(fiber.HeaderIfNoneMatch, etag)
	resp304, err := app.Test(reqConditional)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotModified, resp304.StatusCode)
	require.Equal(t, "public, no-cache", resp304.Header.Get(fiber.HeaderCacheControl))
	require.NoError(t, resp304.Body.Close())
}

func TestLegalSnapshotInvalidatesMetadataAndDocumentsTogether(t *testing.T) {
	state := core.NewAppState()
	cfg := config.DefaultConfig()
	state.Inner.Config.Store(cfg)
	cache := &legalCache{}
	first, err := cache.load(cfg)
	require.NoError(t, err)
	cached, err := cache.load(cfg)
	require.NoError(t, err)
	require.Same(t, first, cached)
	app := fiber.New()
	SetupRoutes(app, state)
	request := func(path, etag string) *http.Response {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set(fiber.HeaderIfNoneMatch, etag)
		resp, err := app.Test(req)
		require.NoError(t, err)
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}
	resp := request("/legal", "")
	require.Equal(t, protohttp.ContentType, resp.Header.Get(fiber.HeaderContentType))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var metadata pb.LegalMetadata
	require.NoError(t, proto.Unmarshal(body, &metadata))
	require.Equal(t, cfg.Legal.Revision(), metadata.Revision)
	oldETag := resp.Header.Get(fiber.HeaderETag)
	require.Equal(t, http.StatusNotModified, request("/legal", `"unrelated", `+strings.TrimPrefix(oldETag, "W/")).StatusCode)
	next := cfg.DeepCopy()
	next.Legal.LegalNotice = "# Updated notice"
	next.Legal.CookieBanner = false
	require.NoError(t, next.Legal.Normalize())
	state.Inner.Config.Store(next)
	resp = request("/legal", oldETag)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	var updated pb.LegalMetadata
	require.NoError(t, proto.Unmarshal(body, &updated))
	require.Equal(t, metadata.Revision, updated.Revision)
	require.NotEqual(t, metadata.ContentRevision, updated.ContentRevision)
	require.False(t, updated.CookieBanner)
	resp = request("/legal/legal-notice", first.documents["legal-notice"].etag)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "# Updated notice", string(body))
}

func BenchmarkLegalSnapshot(b *testing.B) {
	cfg := config.DefaultConfig()
	cfg.Legal.PrivacyPolicy = strings.Repeat("x", config.MaxLegalDocumentBytes)
	cache := &legalCache{}
	if _, err := cache.load(cfg); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := cache.load(cfg); err != nil {
			b.Fatal(err)
		}
	}
}
