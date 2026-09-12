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
	"io"
	"io/fs"
	"log"
	"os"

	"renop/internal/utils"

	"github.com/minio/minio-go/v7"
)

type s3Backend struct{}

func (s3Backend) Open(path string) (io.ReadCloser, int64, bool, error) {
	reader, info, err := DownloadFromS3(utils.GetS3Key(path))
	if isS3NotFound(err) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	return reader, info.Size, true, nil
}

func (s3Backend) Stat(path string) (int64, bool, error) {
	info, err := StatS3(utils.GetS3Key(path))
	if isS3NotFound(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return info.Size, true, nil
}

func (s3Backend) Commit(staged, target string) error {
	if err := UploadToS3(staged, utils.GetS3Key(target)); err != nil {
		return err
	}
	if err := os.Remove(staged); err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Printf("Failed to remove committed staging file %s: %v", staged, err)
	}
	return nil
}

func (s3Backend) Delete(path string) error {
	err := DeleteFromS3(utils.GetS3Key(path))
	if isS3NotFound(err) {
		return nil
	}
	return err
}

func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}
	response := minio.ToErrorResponse(err)
	return response.StatusCode == 404 || response.Code == "NoSuchKey" || response.Code == "NoSuchObject"
}
