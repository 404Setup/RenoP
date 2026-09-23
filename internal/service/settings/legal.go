/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package settings

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/audit"
	"renop/internal/utils/protohttp"
	"renop/pkg/pb"
)

func getLegalSettings(c fiber.Ctx, state *core.AppState) error {
	if !isManager(c) {
		return fiber.ErrForbidden
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return protohttp.Write(c, legalSettingsMessage(state.Inner.Config.Load().Legal))
}

func putLegalSettings(c fiber.Ctx, state *core.AppState) error {
	if !isManager(c) {
		return fiber.ErrForbidden
	}
	var payload pb.LegalSettings
	if err := protohttp.ReadLimit(c, &payload, 16*config.MaxLegalDocumentBytes+1024); err != nil {
		if errors.Is(err, fiber.ErrRequestEntityTooLarge) || errors.Is(err, fiber.ErrUnsupportedMediaType) {
			return err
		}
		return cacheSettingsError(c, 400, "legal_settings_invalid")
	}
	request, err := parseLegalSettings(&payload)
	if err != nil {
		return cacheSettingsError(c, 400, "legal_settings_invalid")
	}
	state.Inner.ConfigWriteLock.Lock()
	defer state.Inner.ConfigWriteLock.Unlock()
	current := state.Inner.Config.Load().Legal
	now := time.Now().UTC()
	if request.PrivacyPolicy != current.PrivacyPolicy {
		request.PrivacyPolicy = config.UpdateLastUpdated(request.PrivacyPolicy, now, "en")
	}
	if request.TermsOfService != current.TermsOfService {
		request.TermsOfService = config.UpdateLastUpdated(request.TermsOfService, now, "en")
	}
	if request.Translations != nil {
		for lang, tr := range request.Translations {
			currTr := current.Translations[lang]
			if tr.PrivacyPolicy != currTr.PrivacyPolicy {
				tr.PrivacyPolicy = config.UpdateLastUpdated(tr.PrivacyPolicy, now, lang)
			}
			if tr.TermsOfService != currTr.TermsOfService {
				tr.TermsOfService = config.UpdateLastUpdated(tr.TermsOfService, now, lang)
			}
			request.Translations[lang] = tr
		}
	}
	if err := request.Normalize(); err != nil {
		return cacheSettingsError(c, 400, "legal_settings_invalid")
	}
	next := state.Inner.Config.Load().DeepCopy()
	next.Legal = request.DeepCopy()
	if err := persistConfigSnapshot(next); err != nil {
		return cacheSettingsError(c, 500, "legal_settings_save_failed")
	}
	state.Inner.Config.Store(next)
	username, operator, method, sessionID, ip := audit.ExtractAuthDetails(c, state)
	audit.Log(state, &core.AuditLogEntry{Username: username, Operator: operator, AuthMethod: method,
		SessionID: sessionID, IP: ip, Action: audit.ActionSettingsUpdate, Details: "Updated legal documents and cookie notice"})
	c.Set(fiber.HeaderCacheControl, "no-store")
	return protohttp.Write(c, legalSettingsMessage(next.Legal))
}

func legalSettingsMessage(value config.LegalConfig) *pb.LegalSettings {
	msg := &pb.LegalSettings{
		PrivacyPolicy:  value.PrivacyPolicy,
		TermsOfService: value.TermsOfService,
		LegalNotice:    value.LegalNotice,
		CookieBanner:   value.CookieBanner,
	}
	if len(value.Translations) > 0 {
		msg.Translations = make(map[string]*pb.LegalDocumentSet, len(value.Translations))
		for k, v := range value.Translations {
			msg.Translations[k] = &pb.LegalDocumentSet{
				PrivacyPolicy:  v.PrivacyPolicy,
				TermsOfService: v.TermsOfService,
				LegalNotice:    v.LegalNotice,
			}
		}
	}
	return msg
}

func parseLegalSettings(payload *pb.LegalSettings) (config.LegalConfig, error) {
	value := config.LegalConfig{
		PrivacyPolicy:  payload.PrivacyPolicy,
		TermsOfService: payload.TermsOfService,
		LegalNotice:    payload.LegalNotice,
		CookieBanner:   payload.CookieBanner,
	}
	if len(payload.Translations) > 0 {
		value.Translations = make(map[string]config.LegalDocumentSet, len(payload.Translations))
		for k, v := range payload.Translations {
			if v != nil {
				value.Translations[k] = config.LegalDocumentSet{
					PrivacyPolicy:  v.PrivacyPolicy,
					TermsOfService: v.TermsOfService,
					LegalNotice:    v.LegalNotice,
				}
			}
		}
	}
	err := value.Normalize()
	return value, err
}
