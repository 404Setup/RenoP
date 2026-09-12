/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package database

import (
	"errors"
	"fmt"
	"time"

	"renop/internal/config"
	"renop/internal/core"
)

func checkNativeResources(db *DB, suffix string) error {
	now := time.Now().Add(-10 * time.Second).UnixMilli()
	owner, member, other, reviewer := "native-owner-"+suffix, "native-member-"+suffix, "native-other-"+suffix, "native-reviewer-"+suffix
	for _, account := range []*core.AccessToken{
		{Name: owner, Permissions: []string{"base", "canupdate:nativechecks"}},
		{Name: member, Permissions: []string{"base"}},
		{Name: other, Permissions: []string{"base", "canupdate:nativechecks"}},
		{Name: reviewer, Permissions: []string{"manager"}},
	} {
		if err := db.SaveToken(account); err != nil {
			return err
		}
	}
	name := "example-" + suffix
	resource, err := db.CreateNativeResource("nativechecks", config.RepositoryFormatConda, name, owner, now)
	if err != nil {
		return err
	}
	if resource.PermissionLevel != core.NativePermissionOwner {
		return fmt.Errorf("missing native owner")
	}
	if _, err := db.CreateNativeResource("nativechecks", config.RepositoryFormatConda, name, other, now); !errors.Is(err, core.ErrNativeExists) {
		return fmt.Errorf("native resource takeover: %v", err)
	}
	if err := db.CheckNativePermission("nativechecks", name, other, core.NativePermissionPublish); !errors.Is(err, core.ErrNativePermission) {
		return fmt.Errorf("repository writer bypassed native ownership: %v", err)
	}
	if err := db.SetNativeMember("nativechecks", name, owner, member, core.NativePermissionPublish, now); err != nil {
		return err
	}
	if err := db.CheckNativePermission("nativechecks", name, member, core.NativePermissionPublish); err != nil {
		return err
	}
	if err := db.CheckNativePermission("nativechecks", name, member, core.NativePermissionVersion); !errors.Is(err, core.ErrNativePermission) {
		return fmt.Errorf("L1 bypassed version authority: %v", err)
	}
	if err := db.SetNativeMember("nativechecks", name, owner, owner, -1, now); !errors.Is(err, core.ErrNativeLastOwner) {
		return fmt.Errorf("last native owner removed: %v", err)
	}
	if plan, err := db.GetAccountRetirementPlan(owner); err != nil || plan.PackageOwnerCount != 1 || plan.Eligible {
		return errorsOrMissing(err, "native ownership retirement hold")
	}
	artifact := core.NativeArtifact{Repository: "nativechecks", ResourceID: resource.ID, Name: name, Version: "1.0/noarch/0", Path: "noarch/" + name + "-1.0-0.conda", Size: 42, CreatedAt: now}
	if err := db.SaveNativeArtifact(artifact, other, false); !errors.Is(err, core.ErrNativePermission) {
		return fmt.Errorf("foreign native publication accepted: %v", err)
	}
	if err := db.SaveNativeArtifact(artifact, member, false); err != nil {
		return err
	}
	if public, err := db.ListNativeResources("nativechecks", "", false, 100, 0); err != nil || len(public) != 0 {
		return errorsOrMissing(err, "pending native resource visibility")
	}
	request := core.PublicationReviewRequest{ResourceType: core.ReviewResourceNativePackage, Repository: "nativechecks", ResourceKey: name, ResourceName: name, Version: artifact.Version, RequestedBy: member,
		Policy: config.PublicationReviewEveryVersion, CreatedAt: now, Files: []*core.ReviewFile{{Path: artifact.Path, Size: 42, AddedAt: now, Critical: true}}}
	review, err := db.CreateOrUpdatePublicationReview(request)
	if err != nil || !review.Pending {
		return errorsOrMissing(err, "native publication review")
	}
	if pending, err := db.NativePathHasPendingReview("nativechecks", "noarch"); err != nil || !pending {
		return errorsOrMissing(err, "native review blocks parent deletion")
	}
	if pending, err := db.NativePathHasPendingReview("nativechecks", "noarch-other"); err != nil || pending {
		return errorsOrMissing(err, "native review parent boundary")
	}
	if err := claimDriverTicket(db, review.TaskID, reviewer); err != nil {
		return err
	}
	if err := db.SetNativeMember("nativechecks", name, owner, member, -1, now); err != nil {
		return err
	}
	if _, err := db.DecideReviewTask(review.TaskID, reviewer, core.ReviewStatusApproved, "", time.Now().UnixMilli()); !errors.Is(err, core.ErrNativePermission) {
		return fmt.Errorf("revoked publisher review was approved: %v", err)
	}
	if pending, err := db.GetNativeArtifact("nativechecks", artifact.Path); err != nil || pending.Published {
		return errorsOrMissing(err, "denied native review remained hidden")
	}
	if err := db.SetNativeMember("nativechecks", name, owner, member, core.NativePermissionPublish, now); err != nil {
		return err
	}
	if _, err := db.DecideReviewTask(review.TaskID, reviewer, core.ReviewStatusApproved, "", time.Now().UnixMilli()); err != nil {
		return err
	}
	if published, err := db.GetNativeArtifact("nativechecks", artifact.Path); err != nil || !published.Published {
		return errorsOrMissing(err, "atomic native review visibility")
	}
	if _, err := db.CreateOrUpdatePublicationReview(request); !errors.Is(err, core.ErrReviewPublicationSealed) {
		return fmt.Errorf("approved native version was not sealed: %v", err)
	}
	if err := db.SetNativeMember("nativechecks", name, owner, member, core.NativePermissionOwner, now); err != nil {
		return err
	}
	if err := db.SetNativeMember("nativechecks", name, member, owner, -1, now); err != nil {
		return err
	}
	if err := db.CheckNativePermission("nativechecks", name, owner, core.NativePermissionPublish); !errors.Is(err, core.ErrNativePermission) {
		return fmt.Errorf("removed native owner retained publication access")
	}
	if err := db.DeleteNativeResource("nativechecks", name, member); !errors.Is(err, core.ErrNativeBusy) {
		return fmt.Errorf("nonempty native resource released")
	}
	if err := db.DeleteNativeArtifact("nativechecks", artifact.Path, member); err != nil {
		return err
	}
	return db.DeleteNativeResource("nativechecks", name, member)
}
