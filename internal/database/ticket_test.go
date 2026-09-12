/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"renop/internal/core"
	"renop/internal/database"

	"github.com/stretchr/testify/require"
)

func TestTicketCommentAdmissionAndCursorPages(t *testing.T) {
	db := newMavenDB(t)
	session := ticketSession(t, db, "alice")
	now := time.Now().UnixMilli()
	task, err := db.CreateTicket(core.TicketRequest{Kind: core.TicketKindFeedback, Title: "Help", Body: "Details"}, "alice", session, now)
	require.NoError(t, err)
	for i := range 20 {
		msg := &core.TicketMessage{TaskID: task.ID, Body: fmt.Sprintf("Comment %d", i), CreatedAt: now, AuthorRole: "admin", Kind: "event"}
		require.NoError(t, db.AddTicketMessage(msg, "alice", session))
		require.Equal(t, "author", msg.AuthorRole)
		require.Equal(t, "comment", msg.Kind)
	}
	err = db.AddTicketMessage(&core.TicketMessage{TaskID: task.ID, Body: "Excess comment", CreatedAt: now}, "alice", session)
	require.ErrorIs(t, err, core.ErrReviewFileLimit)
	seen, before := map[string]bool{}, ""
	for {
		messages, next, err := db.ListTicketMessages(task.ID, before, 7)
		require.NoError(t, err)
		require.LessOrEqual(t, len(messages), 7)
		for _, msg := range messages {
			require.False(t, seen[msg.ID], "cursor repeated a message with an equal timestamp")
			seen[msg.ID] = true
		}
		if next == "" {
			break
		}
		before = next
	}
	require.Len(t, seen, 20)
	_, _, err = db.ListTicketMessages(task.ID, "", 101)
	require.ErrorIs(t, err, core.ErrReviewInvalidRequest)
	err = db.AddTicketMessage(&core.TicketMessage{TaskID: task.ID, Body: "Forged session"}, "alice", "stale")
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	_, err = db.TransitionTicket(task.ID, "alice", session, core.TicketAction{Action: "close"}, now+1)
	require.NoError(t, err)
	err = db.AddTicketMessage(&core.TicketMessage{TaskID: task.ID, Body: "Closed comment", CreatedAt: now + 60001}, "alice", session)
	require.ErrorIs(t, err, core.ErrReviewTaskConflict)
}

func TestTicketCreationBurstPersistsAcrossSessionsAndKinds(t *testing.T) {
	db := newMavenDB(t)
	session, now := ticketSession(t, db, "alice"), time.Now().UnixMilli()
	request := core.TicketRequest{Kind: core.TicketKindFeedback, Title: "Help", Body: "Details"}
	for range 6 {
		_, err := db.CreateTicket(request, "alice", session, now)
		require.NoError(t, err)
	}
	request.Kind = core.TicketKindSuggestion
	_, err := db.CreateTicket(request, "alice", ticketSession(t, db, "alice"), now)
	require.ErrorIs(t, err, core.ErrReviewFileLimit)
	_, err = db.CreateTicket(request, "alice", session, now+(10*time.Minute).Milliseconds()+1)
	require.NoError(t, err)
}

func ticketSession(t *testing.T, db *database.DB, name string) string {
	t.Helper()
	now := time.Now().UnixMilli()
	session := &core.Session{PublicID: "ticket-" + name, Username: name, CreatedAt: now}
	session.LastActive.Store(now)
	secret := "ticket-session-" + name
	require.NoError(t, db.SaveSession(session, secret))
	return secret
}

func TestTicketClaimsEscalationAndIdentityPrivacy(t *testing.T) {
	db := newMavenDB(t)
	for name, roles := range map[string][]string{
		"mod1": {"canmoderate:releases"}, "mod2": {"canmoderate:releases"}, "outside": {"canmoderate:other"},
		"admin2": {"manager"}, "admin3": {"manager"}, "admin4": {"manager"},
	} {
		require.NoError(t, db.SaveToken(&core.AccessToken{Name: name, Permissions: roles}))
	}
	sessions := make(map[string]string)
	for _, name := range []string{"alice", "bob", "mod1", "mod2", "outside", "admin", "admin2", "admin3", "admin4"} {
		sessions[name] = ticketSession(t, db, name)
	}
	now := time.Now().UnixMilli()
	task, err := db.CreateTicket(core.TicketRequest{Kind: core.TicketKindFeedback, Repository: "releases",
		Title: "Download issue", Body: "First line\nSecond line"}, "alice", sessions["alice"], now)
	require.NoError(t, err)
	act := func(actor, action string, force bool) (*core.ReviewTask, error) {
		return db.TransitionTicket(task.ID, actor, sessions[actor], core.TicketAction{
			Action: action, Force: force, Outcome: "resolved", Response: "Resolved\nPlease try again."}, now+1)
	}
	_, err = act("outside", "claim", false)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	_, err = act("mod1", "complete", false)
	require.ErrorIs(t, err, core.ErrTicketClaimRequired)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, actor := range []string{"mod1", "mod2"} {
		wg.Go(func() {
			_, err := act(actor, "claim", false)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else {
			require.ErrorIs(t, err, core.ErrTicketOccupied)
		}
	}
	require.Equal(t, 1, winners)
	_, err = act("admin", "claim", true)
	require.NoError(t, err)
	_, err = act("admin2", "claim", true)
	require.ErrorIs(t, err, core.ErrTicketOccupied)
	require.NoError(t, db.UpdateToken("admin", func(token *core.AccessToken) { token.Permissions = []string{"base"} }))
	available, err := db.GetTicket(task.ID, "admin2")
	require.NoError(t, err)
	require.Contains(t, available.Actions, "force_claim")
	_, err = act("admin2", "claim", true)
	require.NoError(t, err)
	_, err = act("admin2", "release", false)
	require.NoError(t, err)
	require.NoError(t, db.UpdateToken("admin", func(token *core.AccessToken) { token.Permissions = []string{"manager"} }))
	_, err = act("mod1", "claim", false)
	require.NoError(t, err)
	require.NoError(t, db.UpdateToken("mod1", func(token *core.AccessToken) { token.Permissions = []string{"manager"} }))
	available, err = db.GetTicket(task.ID, "admin2")
	require.NoError(t, err)
	require.NotContains(t, available.Actions, "force_claim")
	_, err = act("admin2", "claim", true)
	require.ErrorIs(t, err, core.ErrTicketOccupied)
	require.NoError(t, db.UpdateToken("mod1", func(token *core.AccessToken) { token.Permissions = []string{"canmoderate:releases"} }))
	_, err = act("mod2", "complete", false)
	require.ErrorIs(t, err, core.ErrTicketClaimRequired)
	_, err = act("mod1", "escalate", false)
	require.NoError(t, err)
	_, err = act("mod2", "claim", false)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	for _, actor := range []string{"admin", "admin2"} {
		_, err = act(actor, "claim", false)
		require.NoError(t, err)
		_, err = act(actor, "escalate", false)
		require.NoError(t, err)
		_, err = act(actor, "claim", false)
		require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	}
	task, err = act("admin3", "claim", false)
	require.NoError(t, err)
	require.Equal(t, 3, task.Escalations)
	_, err = act("admin4", "claim", true)
	require.ErrorIs(t, err, core.ErrTicketOccupied)
	for _, action := range []string{"escalate", "release"} {
		_, err = act("admin3", action, false)
		require.ErrorIs(t, err, core.ErrTicketEscalationLimit)
	}
	task, err = act("admin3", "process", false)
	require.NoError(t, err)
	require.Equal(t, core.TicketProcessed, task.TicketState.Status)
	task, err = act("admin3", "complete", false)
	require.NoError(t, err)
	require.Equal(t, core.TicketCompleted, task.TicketState.Status)
	requester, err := db.GetTicket(task.ID, "alice")
	require.NoError(t, err)
	require.Empty(t, requester.Assignee)
	require.Empty(t, requester.DecidedBy)
	require.Empty(t, requester.EscalatedBy)
	staff, err := db.GetTicket(task.ID, "mod1")
	require.NoError(t, err)
	require.Equal(t, "admin3", staff.DecidedBy)
	_, count, err := db.ListReviewTasks(core.ReviewTaskListOptions{Username: "mod1", TicketStatus: core.TicketCompleted, Limit: 10})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, count, err = db.ListReviewTasks(core.ReviewTaskListOptions{Username: "outside", Administrator: true, TicketStatus: "all", Limit: 10})
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestTicketReportsRejectSelfDuplicatesAndHiddenResources(t *testing.T) {
	db := newMavenDB(t)
	alice, bob := ticketSession(t, db, "alice"), ticketSession(t, db, "bob")
	now := time.Now().UnixMilli()
	request := core.TicketRequest{Kind: core.TicketKindReport, Title: "Report", Body: "Please investigate.",
		Target: core.ResourceLockTarget{Format: "user", Name: "alice"}}
	_, err := db.CreateTicket(request, "alice", alice, now)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	task, err := db.CreateTicket(request, "bob", bob, now)
	require.NoError(t, err)
	_, err = db.CreateTicket(request, "bob", bob, now)
	require.ErrorIs(t, err, core.ErrReviewTaskExists)
	_, err = db.GetTicket(task.ID, "alice")
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	report, err := db.GetTicket(task.ID, "bob")
	require.NoError(t, err)
	require.Empty(t, report.ResourceVersion)
	require.Equal(t, "alice", report.ResourceKey)
	require.NoError(t, db.CreateMavenDomain(&core.MavenDomain{Domain: "com.example", VerificationType: "dns",
		VerificationHost: "example.com", VerificationCode: "proof", CreatedAt: now}, "alice"))
	require.NoError(t, db.MarkMavenDomainVerified("com.example", "proof", now, nil))
	require.NoError(t, db.RecordMavenPublication(&core.MavenArtifact{Repository: "maven", Domain: "com.example",
		GroupID: "com.example", ArtifactID: "demo", Publisher: "alice", CreatedAt: now},
		&core.MavenVersion{Version: "1.0", Publisher: "alice", CreatedAt: now}))
	request.Target = core.ResourceLockTarget{Format: "maven", Repository: "maven", Name: "com.example:demo", Version: "1.0"}
	_, err = db.CreateTicket(request, "alice", alice, now)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
	task, err = db.CreateTicket(request, "bob", bob, now)
	require.NoError(t, err)
	report, err = db.GetTicket(task.ID, "bob")
	require.NoError(t, err)
	require.Equal(t, "1.0", report.ResourceVersion)
	require.NoError(t, db.SetResourceLock(&core.ResourceLock{ResourceLockTarget: request.Target,
		Source: core.ResourceLockSystem, Mode: core.ResourceLockRead, Reason: "trojan", LockedAt: now}, "", ""))
	_, err = db.CreateTicket(request, "bob", bob, now)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)
}

func TestTicketCloseReasonsAndConversationMessages(t *testing.T) {
	db := newMavenDB(t)
	for name, roles := range map[string][]string{
		"mod1": {"canmoderate:releases"}, "teamlead": {"base"},
	} {
		require.NoError(t, db.SaveToken(&core.AccessToken{Name: name, Permissions: roles}))
	}
	sessions := make(map[string]string)
	for _, name := range []string{"alice", "mod1", "teamlead"} {
		sessions[name] = ticketSession(t, db, name)
	}
	now := time.Now().UnixMilli()
	task, err := db.CreateTicket(core.TicketRequest{Kind: core.TicketKindFeedback, Repository: "releases",
		Title: "Feedback with close", Body: "Initial feedback"}, "alice", sessions["alice"], now)
	require.NoError(t, err)

	// Add messages to ticket
	err = db.AddTicketMessage(&core.TicketMessage{
		TaskID:     task.ID,
		AuthorID:   "uid-alice",
		AuthorName: "alice",
		AuthorRole: "author",
		Kind:       "comment",
		Body:       "Here is more detail about the issue.",
		CreatedAt:  now + 10,
	}, "alice", sessions["alice"])
	require.NoError(t, err)

	err = db.AddTicketMessage(&core.TicketMessage{
		TaskID:     task.ID,
		AuthorID:   "uid-mod1",
		AuthorName: "mod1",
		AuthorRole: "moderator",
		Kind:       "comment",
		Body:       "Thanks, looking into this now.",
		CreatedAt:  now + 20,
	}, "mod1", sessions["mod1"])
	require.NoError(t, err)

	messages, _, err := db.ListTicketMessages(task.ID, "", 100)
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.Equal(t, "alice", messages[0].AuthorName)
	require.Equal(t, "Here is more detail about the issue.", messages[0].Body)
	require.Equal(t, "mod1", messages[1].AuthorName)

	// Requester closes ticket with "resolved"
	closed, err := db.TransitionTicket(task.ID, "alice", sessions["alice"], core.TicketAction{
		Action:   "close",
		Outcome:  core.TicketCloseReasonResolved,
		Response: "Fixed on my end, closing.",
	}, now+30)
	require.NoError(t, err)
	require.Equal(t, core.TicketClosed, closed.TicketState.Status)
	require.Equal(t, core.TicketCloseReasonResolved, closed.Outcome)
	require.Equal(t, core.ReviewStatusCancelled, closed.Status)

	// Messages should now include the close event
	messagesAfterClose, _, err := db.ListTicketMessages(task.ID, "", 100)
	require.NoError(t, err)
	require.Len(t, messagesAfterClose, 3)
	require.Equal(t, "event", messagesAfterClose[2].Kind)
	require.Contains(t, messagesAfterClose[2].Body, "resolved")
}

func TestNativePackageReportTicket(t *testing.T) {
	db := newMavenDB(t)
	require.NoError(t, db.SaveToken(&core.AccessToken{Name: "alice", Permissions: []string{"base", "canupdate:native"}}))
	alice, bob := ticketSession(t, db, "alice"), ticketSession(t, db, "bob")
	now := time.Now().UnixMilli()

	// Create native resource with owner alice
	res, err := db.CreateNativeResource("native", "apk", "demo-pkg", "alice", now)
	require.NoError(t, err)

	// Save artifact for version 1.0.0
	require.NoError(t, db.SaveNativeArtifact(core.NativeArtifact{
		Repository: "native", ResourceID: res.ID, Name: "demo-pkg", Version: "1.0.0/x86_64",
		Path: "x86_64/demo-pkg-1.0.0.apk", Size: 1024, Published: true, CreatedAt: now,
	}, "alice", true))

	// Owner alice cannot report her own package
	request := core.TicketRequest{
		Kind: core.TicketKindReport, Title: "Malware report", Body: "Suspected malware in package.",
		Target: core.ResourceLockTarget{Format: "apk", Repository: "native", Name: "demo-pkg", Version: "1.0.0"},
	}
	_, err = db.CreateTicket(request, "alice", alice, now)
	require.ErrorIs(t, err, core.ErrReviewPermissionDenied)

	// Bob reports the package
	task, err := db.CreateTicket(request, "bob", bob, now)
	require.NoError(t, err)
	require.NotEmpty(t, task.ID)

	report, err := db.GetTicket(task.ID, "bob")
	require.NoError(t, err)
	require.Equal(t, "demo-pkg", report.ResourceKey)
	require.Equal(t, "1.0.0", report.ResourceVersion)
	require.Equal(t, "apk", report.ResourceType)
	require.Equal(t, "native", report.Repository)
	aliceProfile, err := db.GetUserProfile("alice")
	require.NoError(t, err)
	require.Contains(t, report.TargetUserIDs, aliceProfile.UserID)
}
