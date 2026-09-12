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

import "strings"

// RepositoryEngine describes configuration capabilities independently of HTTP
// handlers. Returning values keeps the built-in catalog immutable to callers.
type RepositoryEngine struct {
	ID                string
	Protocol          string
	PublicationReview bool
	Redeployment      bool
	GPG               bool
	ArtifactTemplate  bool
	DirectUpload      bool
	NativeFileLayout  bool
	ManagedNative     bool
}

var repositoryEngineCatalog = map[string]RepositoryEngine{
	RepositoryFormatMaven:        {ID: RepositoryFormatMaven, Protocol: RepositoryFormatMaven, PublicationReview: true, Redeployment: true, GPG: true, DirectUpload: true},
	RepositoryFormatMavenClassic: {ID: RepositoryFormatMavenClassic, Protocol: RepositoryFormatMaven, PublicationReview: true, Redeployment: true, GPG: true, DirectUpload: true},
	RepositoryFormatFiles:        {ID: RepositoryFormatFiles, Protocol: RepositoryFormatFiles, Redeployment: true, DirectUpload: true, NativeFileLayout: true},
	RepositoryFormatCargo:        {ID: RepositoryFormatCargo, Protocol: RepositoryFormatCargo, PublicationReview: true, ArtifactTemplate: true},
	RepositoryFormatDocker:       {ID: RepositoryFormatDocker, Protocol: RepositoryFormatDocker, PublicationReview: true, Redeployment: true},
	RepositoryFormatNPM:          {ID: RepositoryFormatNPM, Protocol: RepositoryFormatNPM, PublicationReview: true},
	RepositoryFormatConan:        nativeFileEngine(RepositoryFormatConan, RepositoryFormatConan),
	RepositoryFormatConda:        nativeFileEngine(RepositoryFormatConda, RepositoryFormatConda),
	RepositoryFormatCondaNative:  nativeFileEngine(RepositoryFormatCondaNative, RepositoryFormatConda),
	RepositoryFormatAPK:          nativeFileEngine(RepositoryFormatAPK, RepositoryFormatAPK),
	RepositoryFormatAPT:          nativeFileEngine(RepositoryFormatAPT, RepositoryFormatAPT),
	RepositoryFormatRPM:          nativeFileEngine(RepositoryFormatRPM, RepositoryFormatRPM),
	RepositoryFormatYUM:          nativeFileEngine(RepositoryFormatYUM, RepositoryFormatRPM),
}

func nativeFileEngine(id, protocol string) RepositoryEngine {
	return RepositoryEngine{ID: id, Protocol: protocol, PublicationReview: true, Redeployment: true, DirectUpload: true, NativeFileLayout: true, ManagedNative: true}
}

func LookupRepositoryEngine(format string) (RepositoryEngine, bool) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = RepositoryFormatMaven
	}
	engine, ok := repositoryEngineCatalog[format]
	return engine, ok
}

func (r *Repository) Engine() RepositoryEngine {
	engine, _ := LookupRepositoryEngine(r.ConfiguredFormat())
	return engine
}
