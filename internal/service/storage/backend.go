/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package storage

import "io"

// artifactBackend owns physical persistence. Protocol validation, authorization,
// capacity reservations and index publication stay with the calling service.
// Commit consumes a closed staging file only after installing its contents.
type artifactBackend interface {
	Open(path string) (io.ReadCloser, int64, bool, error)
	Stat(path string) (int64, bool, error)
	Commit(staged, target string) error
	Delete(path string) error
}

func backendFor(path string) artifactBackend {
	if IsS3Enabled(path) {
		return s3Backend{}
	}
	return diskBackend{}
}
