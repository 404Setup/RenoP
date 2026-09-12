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

import (
	"path"
	"strings"
)

// IsNativeMetadata identifies replaceable indexes in native repository layouts.
// Package archives remain subject to the repository's redeployment policy.
func (r *Repository) IsNativeMetadata(relativePath string) bool {
	name := path.Base(strings.ReplaceAll(relativePath, "\\", "/"))
	switch r.Engine().Protocol {
	case RepositoryFormatConda:
		for _, base := range []string{"repodata.json", "current_repodata.json", "repodata_from_packages.json", "channeldata.json"} {
			if name == base || name == base+".bz2" || name == base+".zst" || name == base+".jlap" {
				return true
			}
		}
	case RepositoryFormatAPK:
		return name == "APKINDEX.tar.gz" || name == "Packages.adb"
	case RepositoryFormatAPT:
		for _, base := range []string{"InRelease", "Release", "Release.gpg", "Packages", "Sources", "Contents", "Translation"} {
			if name == base || strings.HasPrefix(name, base+".") || strings.HasPrefix(name, base+"-") {
				return true
			}
		}
	case RepositoryFormatRPM:
		return name == "repomd.xml" || name == "repomd.xml.asc" || name == "repomd.xml.key"
	}
	return false
}
