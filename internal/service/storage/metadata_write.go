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

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"renop/internal/core"
	"renop/internal/utils"
)

// writeMavenMetadata shares artifact staging, capacity admission, checksums, and index updates.
func writeMavenMetadata(state *core.AppState, path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".renop.tmp.metadata.*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(data)
	if err = errors.Join(err, file.Close()); err != nil {
		return err
	}
	return CommitUploadedFile(state, path, file.Name(), int64(len(data)), time.Now().UnixNano(), true, true,
		&ContentDigests{MD5: utils.MD5(data), SHA1: utils.SHA1(data), SHA256: utils.SHA256(data), SHA512: utils.SHA512(data)})
}
