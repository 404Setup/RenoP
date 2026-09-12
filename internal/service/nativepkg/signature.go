/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package nativepkg

import (
	"io"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/apk"
	"renop/internal/service/gpg"
	"renop/internal/service/rpm"
)

func ValidateSigningKey(format, key string) error {
	if key == "" {
		return nil
	}
	if len(key) > 64<<10 {
		return core.ErrNativeSignature
	}
	switch format {
	case config.RepositoryFormatAPK:
		if _, err := apk.ParsePublicKey(key); err != nil {
			return core.ErrNativeSignature
		}
	case config.RepositoryFormatRPM, config.RepositoryFormatConan:
		if err := gpg.ValidatePublicSigningKeys([]byte(key)); err != nil {
			return core.ErrNativeSignature
		}
	default:
		return core.ErrNativeSignature
	}
	return nil
}

func VerifySignature(format string, reader io.ReaderAt, size int64, key string) error {
	if format == config.RepositoryFormatRPM || format == config.RepositoryFormatConan {
		if key == "" {
			return core.ErrNativeSignature
		}
		if err := ValidateSigningKey(format, key); err != nil {
			return err
		}
	}
	var err error
	switch format {
	case config.RepositoryFormatAPK:
		err = apk.VerifySignature(reader, size, key)
	case config.RepositoryFormatRPM:
		err = rpm.VerifySignature(reader, size, key)
	}
	if err != nil {
		return core.ErrNativeSignature
	}
	return nil
}
