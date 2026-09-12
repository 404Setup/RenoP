/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package frontend

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"

	"github.com/gofiber/fiber/v3"
	"github.com/klauspost/compress/zstd"
	"github.com/molecule-man/go-brrr"
)

func decodeAssetResponse(t *testing.T, encoding string, body []byte) []byte {
	t.Helper()
	var reader io.Reader
	closeReader := func() error { return nil }
	var err error
	switch encoding {
	case "br":
		reader = brrr.NewReader(bytes.NewReader(body))
	case "zstd":
		var decoder *zstd.Decoder
		decoder, err = zstd.NewReader(bytes.NewReader(body))
		reader = decoder
		closeReader = func() error {
			decoder.Close()
			return nil
		}
	case "gzip":
		var decoder io.ReadCloser
		decoder, err = gzip.NewReader(bytes.NewReader(body))
		reader, closeReader = decoder, decoder.Close
	case "deflate":
		var decoder io.ReadCloser
		decoder, err = zlib.NewReader(bytes.NewReader(body))
		reader, closeReader = decoder, decoder.Close
	default:
		return body
	}
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	closeErr := closeReader()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return decoded
}

func TestLocaleAssetsUseInlineProtobufRepresentations(t *testing.T) {
	files, err := Asset.ReadDir("renop-html/dist/assets/i18n")
	if err != nil {
		t.Fatal(err)
	}
	var path string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "en-US-") && strings.HasSuffix(file.Name(), ".pb") {
			path = "assets/i18n/" + file.Name()
			break
		}
	}
	if path == "" {
		t.Fatal("generated protobuf locale asset missing")
	}
	app := fiber.New()
	app.Get("/locale", func(c fiber.Ctx) error { return ServeEmbeddedFile(c, path) })
	for _, encoding := range []string{"identity", "br", "gzip"} {
		req := httptest.NewRequest(http.MethodGet, "/locale", nil)
		req.Header.Set(fiber.HeaderAcceptEncoding, encoding)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || resp.Header.Get(fiber.HeaderContentType) != "application/x-protobuf" || resp.Header.Get(fiber.HeaderContentDisposition) != "inline" {
			t.Fatalf("unexpected locale representation: %d %v", resp.StatusCode, resp.Header)
		}
		if !strings.Contains(resp.Header.Get(fiber.HeaderCacheControl), "immutable") || len(decodeAssetResponse(t, resp.Header.Get(fiber.HeaderContentEncoding), body)) == 0 {
			t.Fatal("locale is not cacheable protobuf data")
		}
	}
}

func TestEmbeddedAssetsNegotiatePrecompressedRepresentations(t *testing.T) {
	entries, err := Asset.ReadDir("renop-html/dist/js")
	if err != nil {
		t.Fatal(err)
	}
	var assetPath string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "app-") && strings.HasSuffix(entry.Name(), ".js") {
			assetPath = "/js/" + entry.Name()
		}
	}
	if assetPath == "" {
		t.Fatal("application bundle is missing")
	}
	want, err := readAsset(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/js/*", ServeJs)
	etags := make(map[string]string, len(embeddedAssetEncodings)+1)
	for _, test := range []struct {
		header   string
		encoding string
	}{
		{header: "br", encoding: "br"},
		{header: "zstd", encoding: "zstd"},
		{header: "gzip", encoding: "gzip"},
		{header: "deflate", encoding: "deflate"},
		{header: "gzip;q=0.5", encoding: ""},
		{header: "gzip;q=0.7, br;q=0.9, identity;q=0.5", encoding: "br"},
	} {
		t.Run(test.header, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, assetPath, nil)
			request.Header.Set(fiber.HeaderAcceptEncoding, test.header)
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status = %d", response.StatusCode)
			}
			if got := response.Header.Get(fiber.HeaderContentEncoding); got != test.encoding {
				t.Fatalf("Content-Encoding = %q, want %q", got, test.encoding)
			}
			if vary := response.Header.Get(fiber.HeaderVary); !strings.Contains(vary, fiber.HeaderAcceptEncoding) {
				t.Fatalf("Vary = %q", vary)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if got := decodeAssetResponse(t, test.encoding, body); !bytes.Equal(got, want) {
				t.Fatal("decoded asset differs from the identity representation")
			}
			etag := response.Header.Get(fiber.HeaderETag)
			if etag == "" {
				t.Fatal("asset response is missing an ETag")
			}
			etags[test.encoding] = etag
			head := httptest.NewRequest(http.MethodHead, assetPath, nil)
			head.Header.Set(fiber.HeaderAcceptEncoding, test.header)
			headResponse, err := app.Test(head)
			if err != nil {
				t.Fatal(err)
			}
			headBody, err := io.ReadAll(headResponse.Body)
			_ = headResponse.Body.Close()
			if err != nil || len(headBody) != 0 || headResponse.ContentLength != int64(len(body)) ||
				headResponse.Header.Get(fiber.HeaderETag) != etag {
				t.Fatalf("HEAD metadata/body mismatch: length=%d body=%d err=%v", headResponse.ContentLength, len(headBody), err)
			}
			if test.encoding != "" && len(body) >= len(want) {
				t.Fatalf("%s representation did not reduce the response size", test.encoding)
			}
		})
	}
	if etags["br"] == etags["gzip"] {
		t.Fatal("Brotli and gzip representations share an ETag")
	}
	conditional := httptest.NewRequest(http.MethodGet, assetPath, nil)
	conditional.Header.Set(fiber.HeaderAcceptEncoding, "br")
	conditional.Header.Set(fiber.HeaderIfNoneMatch, etags["br"])
	response, err := app.Test(conditional)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotModified || response.Header.Get(fiber.HeaderContentEncoding) != "br" {
		t.Fatalf("conditional Brotli response = %d %q", response.StatusCode,
			response.Header.Get(fiber.HeaderContentEncoding))
	}
}

func TestEmbeddedAssetNegotiationRejectsUnacceptableAndSidecarPaths(t *testing.T) {
	app := fiber.New()
	app.Get("/js/*", ServeJs)

	request := httptest.NewRequest(http.MethodGet, "/js/main.js", nil)
	request.Header.Set(fiber.HeaderAcceptEncoding,
		"br;q=0, zstd;q=0, gzip;q=0, deflate;q=0, identity;q=0")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotAcceptable {
		t.Fatalf("unacceptable encoding status = %d", response.StatusCode)
	}

	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/js/main.js.br", nil))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("direct sidecar status = %d", response.StatusCode)
	}
}

func BenchmarkEmbeddedAssetLoad(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		embeddedFileCache.Delete("js/main.js")
		if _, err := loadEmbeddedIdentity("js/main.js"); err != nil {
			b.Fatal(err)
		}
	}
}
