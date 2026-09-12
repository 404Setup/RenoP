/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package demo

import (
	"fmt"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/mail"
)

func (s *seedContext) workflows() error {
	for i, request := range []core.TicketRequest{
		{Kind: core.TicketKindFeedback, Title: "Documentation preview feedback", Body: "The package browser makes it easy to compare versions. Could the installation example be more prominent?"},
		{Kind: core.TicketKindSuggestion, Title: "Improve the onboarding guide", Body: "Please add an example covering private repositories and scoped API tokens."},
		{Kind: core.TicketKindFeedback, Title: "Repository access restored", Body: "The repository permission issue has been resolved. Thank you for the quick response."},
		{Kind: core.TicketKindReport, Title: "Review a suspicious account", Body: "This is a demonstration report for the moderation workflow.", Target: core.ResourceLockTarget{Format: "user", Name: "david"}},
	} {
		at := s.now - int64(i+2)*time.Hour.Milliseconds()
		task, err := s.db.CreateTicket(request, "alice", "preset-session-alice", at)
		if err != nil {
			return err
		}
		if i == 0 {
			if err := s.db.AddTicketMessage(&core.TicketMessage{TaskID: task.ID, Body: "The current layout is already helpful; this is a small usability suggestion.", CreatedAt: at + 1000}, "alice", "preset-session-alice"); err != nil {
				return err
			}
		} else {
			if _, err := s.db.TransitionTicket(task.ID, "admin", "preset-session-admin", core.TicketAction{Action: "claim"}, at+1000); err != nil {
				return err
			}
			if err := s.db.AddTicketMessage(&core.TicketMessage{TaskID: task.ID, Body: "We have reviewed the details and are following up on this request.", CreatedAt: at + 2000}, "admin", "preset-session-admin"); err != nil {
				return err
			}
			if i == 2 {
				if _, err := s.db.TransitionTicket(task.ID, "admin", "preset-session-admin", core.TicketAction{Action: "complete", Outcome: "resolved", Response: "Access is restored and the documentation is updated."}, at+3000); err != nil {
					return err
				}
			}
		}
	}
	if _, err := s.db.CreateOrUpdatePublicationReview(core.PublicationReviewRequest{
		ResourceType: core.ReviewResourceMavenArtifact, Repository: "review", ResourceKey: "com.example.demo:demo-plugin",
		ResourceName: "com.example.demo:demo-plugin", Version: "2.0.0", RequestedBy: "alice", Policy: config.PublicationReviewEveryVersion,
		CreatedAt: s.now - 1800000, Files: []*core.ReviewFile{{Path: "com/example/demo/demo-plugin/2.0.0/demo-plugin-2.0.0.jar", Name: "demo-plugin-2.0.0.jar", Size: 384 << 10, Critical: true}},
	}); err != nil {
		return err
	}
	if err := s.db.SetResourceLock(&core.ResourceLock{Format: "cargo", Repository: "cargo", Name: "renop_demo", Version: "0.9.0",
		Source: core.ResourceLockManual, Mode: core.ResourceLockWrite, Reason: "quality", LockedAt: s.now - 3600000}, "admin", "preset-session-admin"); err != nil {
		return err
	}
	defaults := core.PublicationQuotaLimits{FileLimit: 5000, ByteLimit: 10 << 30, PublicationLimit: 1000, Period: core.PublicationQuotaPeriodMonth}
	for _, subject := range []core.PublicationQuotaSubject{
		{OwnerType: core.PublicationQuotaOwnerUser, OwnerKey: "alice"},
		{OwnerType: core.PublicationQuotaOwnerSuperTeam, OwnerKey: "platform"},
	} {
		reservation, err := s.db.ReservePublicationQuota(subject, defaults, core.PublicationQuotaDelta{Files: 48, Bytes: 128 << 20, Publications: 12}, s.now, s.now+3600000)
		if err != nil {
			return err
		}
		if !reservation.Unlimited {
			if err := s.db.CommitPublicationQuotaReservation(reservation.ID, s.now); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *seedContext) activity() error {
	var messages []*core.UserMessage
	for i, item := range []struct{ title, body, severity string }{
		{"Welcome to RenoP Demo", "Browse repositories, inspect package versions and explore system settings. This dataset is read-only.", "info"},
		{"Publication approved", "The demo-client 1.2.0 release is available in the releases repository.", "success"},
		{"Security review", "A demonstration report is awaiting review in the ticket center.", "warning"},
		{"Scheduled maintenance", "This is a preset announcement. No maintenance will run in demonstration mode.", "info"},
	} {
		message := &core.UserMessage{ID: fmt.Sprintf("demo-message-%d", i), Recipient: "admin", Sender: "carol", Kind: "notification",
			Severity: item.severity, Title: item.title, Body: item.body, CreatedAt: s.now - int64(i+1)*1800000}
		if i == 3 {
			message.ReadAt = s.now - 600000
		}
		messages = append(messages, message)
	}
	if err := s.db.SaveMessages(messages); err != nil {
		return err
	}
	for i := range 36 {
		kind, severity, action, details := "activity", "info", "LOGIN", "Signed in with a browser session"
		username := []string{"admin", "alice", "bobby"}[i%3]
		if i%3 == 1 {
			action, details = "UPLOAD", "Published a preset package version in the releases repository"
		}
		if i%3 == 2 {
			action, details = "PROFILE_UPDATE", "Updated public profile information"
		}
		if i%4 == 0 {
			kind, action, username = "system", "SYSTEM_LOG", "system"
			severity = []string{"info", "warning", "error"}[(i/4)%3]
			details = map[string]string{"info": "Repository metadata cache is ready", "warning": "Example mirror response was slow; a cached result was served", "error": "Example outbound request timed out; the next attempt succeeded"}[severity]
		}
		if err := s.db.SaveAuditLog(&core.AuditLogEntry{Username: username, Operator: username, Action: action, Details: details,
			Kind: kind, Severity: severity, Trigger: "web", AuthMethod: "Session", IP: "192.0.2.10", CreatedAt: s.now - int64(i+1)*600000}); err != nil {
			return err
		}
	}
	var events []*core.DownloadStatisticDelta
	for i, item := range []struct{ repo, format, namespace, name, version string }{
		{"releases", "maven", "com.example.demo", "demo-client", "1.2.0"},
		{"cargo", "cargo", "", "renop_demo", "1.0.0"},
		{"npm", "npm", "platform", "@platform/ui", "1.1.0"},
		{"docker", "docker", "platform", "platform/api", "1.1.0"},
		{"downloads", "files", "", "releases/demo-tool-1.2.0.zip", ""},
	} {
		events = append(events, &core.DownloadStatisticDelta{Username: "admin", Repository: item.repo, Format: item.format,
			Namespace: item.namespace, Package: item.name, Version: item.version, Count: int64(1200 - i*170), Bytes: int64(256-i*30) << 20, UpdatedAt: s.now - 3600000})
	}
	if err := s.db.BatchIncrementDownloadStatistics(events); err != nil {
		return err
	}
	return s.mailHistory()
}

func (s *seedContext) mailHistory() error {
	const owner = "demo-seed"
	_, acquired, err := s.db.AcquireMailLease(owner, s.now)
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("demo mail seed could not acquire its isolated lease")
	}
	defer s.db.ReleaseMailLease(owner)
	for i, status := range []string{"delivered", "accepted", "failed", "unknown"} {
		id := fmt.Sprintf("demo-mail-%d", i)
		job := &mail.Job{ID: id, AccountID: "demo-api", UserID: s.users["admin"], Actor: "admin", Scene: "test", TicketHash: digest(fmt.Sprintf("display-only-demo-mail-ticket-%d", i)),
			CreatedAt: s.now - int64(i+1)*60000, ExpiresAt: s.now + 3600000, UpdatedAt: s.now,
			Message: mail.Message{ID: id, To: "admin@example.com", Subject: "Demonstration delivery status", Text: "Preset email record", CreatedAt: s.now}}
		if _, err := s.db.QueueMailJob(job, s.cfg.Mail.EncryptionKey, "", s.cfg.Mail.ManualRate); err != nil {
			return err
		}
		job.Status = status
		job.Result = mail.Result{Status: status, MessageID: "provider-" + id}
		if status == "failed" {
			job.Result.Code = "mail_recipient_rejected"
		}
		if status == "unknown" {
			job.Result.Code = "mail_status_unknown"
		}
		job.Message.Text = ""
		if err := s.db.SaveMailAttempt(owner, s.cfg.Mail.EncryptionKey, job, nil, 0); err != nil {
			return err
		}
	}
	account := mail.AccountState{Attempts: 420, Charged: 416, SpentMicros: 125000, CalibrationAt: s.now - 3600000}
	account.Normalize(s.cfg.Mail.Accounts[0], s.cfg.Mail.AccountRate, time.UnixMilli(s.now))
	balance := int64(25000000)
	account.BalanceMicros = &balance
	return s.db.SaveMailAccount(owner, "demo-api", s.cfg.Mail.EncryptionKey, account, s.now)
}
