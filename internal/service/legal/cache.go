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
	"crypto/sha256"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/protobuf/proto"

	"renop/internal/config"
	"renop/internal/utils/protohttp"
	"renop/pkg/hex"
	"renop/pkg/pb"
)

type legalDocument struct{ content, etag string }

type legalSnapshot struct {
	config       *config.Config
	documents    map[string]legalDocument
	translations map[string]map[string]legalDocument
	metadata     []byte
	etag         string
}

// legalCache retains one immutable configuration snapshot; edits replace every representation together.
type legalCache struct {
	current atomic.Pointer[legalSnapshot]
	mu      sync.Mutex
}

func contentETag(content string) string {
	sum := sha256.Sum256([]byte(content))
	return `W/"` + hex.EncodeToString(sum[:16]) + `"`
}

func (cache *legalCache) load(cfg *config.Config) (*legalSnapshot, error) {
	if cfg == nil {
		return nil, fiber.ErrServiceUnavailable
	}
	if current := cache.current.Load(); current != nil && current.config == cfg {
		return current, nil
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if current := cache.current.Load(); current != nil && current.config == cfg {
		return current, nil
	}
	value := &legalSnapshot{
		config:       cfg,
		documents:    make(map[string]legalDocument, 3),
		translations: make(map[string]map[string]legalDocument, len(cfg.Legal.Translations)),
	}
	for name, content := range map[string]string{
		"privacy-policy": cfg.Legal.PrivacyPolicy, "terms-of-service": cfg.Legal.TermsOfService, "legal-notice": cfg.Legal.LegalNotice,
	} {
		if content == "" || len(content) > config.MaxLegalDocumentBytes {
			return nil, fiber.ErrServiceUnavailable
		}
		value.documents[name] = legalDocument{content: content, etag: contentETag(content)}
	}
	for lang, set := range cfg.Legal.Translations {
		lowerLang := strings.ToLower(lang)
		trDocs := make(map[string]legalDocument, 3)
		for name, content := range map[string]string{
			"privacy-policy": set.PrivacyPolicy, "terms-of-service": set.TermsOfService, "legal-notice": set.LegalNotice,
		} {
			if content != "" && len(content) <= config.MaxLegalDocumentBytes {
				trDocs[name] = legalDocument{content: content, etag: contentETag(content)}
			}
		}
		if len(trDocs) > 0 {
			value.translations[lowerLang] = trDocs
		}
	}
	contentRevision := contentETag(value.documents["privacy-policy"].etag + value.documents["terms-of-service"].etag + value.documents["legal-notice"].etag)
	var err error
	value.metadata, err = proto.Marshal(&pb.LegalMetadata{Revision: cfg.Legal.Revision(), CookieBanner: cfg.Legal.CookieBanner, ContentRevision: contentRevision})
	if err != nil {
		return nil, err
	}
	value.etag = contentETag(string(value.metadata))
	cache.current.Store(value)
	return value, nil
}

func conditionalResponse(c fiber.Ctx, etag, contentType string) bool {
	c.Set(fiber.HeaderCacheControl, "public, no-cache")
	c.Set(fiber.HeaderETag, etag)
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	for candidate := range strings.SplitSeq(c.Get(fiber.HeaderIfNoneMatch), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
			return true
		}
	}
	return false
}

func (value *legalSnapshot) serveMetadata(c fiber.Ctx) error {
	c.Vary(fiber.HeaderAccept)
	if conditionalResponse(c, value.etag, protohttp.ContentType) {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return c.Send(value.metadata)
}

func parseRequestedLanguage(c fiber.Ctx) string {
	if lang := strings.TrimSpace(c.Query("lang")); lang != "" {
		return strings.ToLower(lang)
	}
	accept := c.Get(fiber.HeaderAcceptLanguage)
	if accept == "" {
		return ""
	}
	semicolon := string(byte(59))
	parts := strings.SplitSeq(accept, ",")
	for part := range parts {
		tag, _, _ := strings.Cut(part, semicolon)
		tag = strings.TrimSpace(tag)
		if tag != "" {
			return strings.ToLower(tag)
		}
	}
	return ""
}

func (value *legalSnapshot) serveDocument(c fiber.Ctx, name string) error {
	document, ok := value.documents[name]
	if !ok {
		return fiber.ErrNotFound
	}
	if reqLang := parseRequestedLanguage(c); reqLang != "" && len(value.translations) > 0 {
		if trDocs, found := value.translations[reqLang]; found {
			if doc, hasDoc := trDocs[name]; hasDoc && doc.content != "" {
				document = doc
			}
		} else {
			primary, _, _ := strings.Cut(reqLang, "-")
			for k, trDocs := range value.translations {
				kPrimary, _, _ := strings.Cut(k, "-")
				if kPrimary == primary {
					if doc, hasDoc := trDocs[name]; hasDoc && doc.content != "" {
						document = doc
						break
					}
				}
			}
		}
	}
	c.Vary(fiber.HeaderAcceptLanguage)
	if conditionalResponse(c, document.etag, "text/plain; charset=utf-8") {
		return c.SendStatus(fiber.StatusNotModified)
	}
	return c.SendString(document.content)
}
