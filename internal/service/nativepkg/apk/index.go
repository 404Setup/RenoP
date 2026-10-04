/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package apk

import (
	"archive/tar"
	"bytes"
	"strings"

	"github.com/klauspost/compress/gzip"
)

// Index produces the native APKINDEX tar member. Existing signed index archives
// can be supplied directly; this renderer never changes their signature bytes.
func Index(packages []*Package) ([]byte, error) {
	if len(packages) > 10000 {
		return nil, ErrInvalidPackage
	}
	var index strings.Builder
	for _, pkg := range packages {
		if pkg == nil {
			return nil, ErrInvalidPackage
		}
		for _, field := range []string{"C", "P", "V", "A", "S", "I", "T", "U", "L", "o", "m", "t", "c", "k", "D", "p", "i"} {
			value, exists := pkg.Fields[field]
			if !exists {
				continue
			}
			if strings.ContainsAny(value, "\r\n\x00") || index.Len()+len(value)+4 > 32<<20 {
				return nil, ErrInvalidPackage
			}
			index.WriteString(field + ":" + value + "\n")
		}
		index.WriteByte('\n')
	}
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	for _, file := range []struct{ name, content string }{{"DESCRIPTION", "RenoP repository\n"}, {"APKINDEX", index.String()}} {
		if err := archive.WriteHeader(&tar.Header{Name: file.name, Mode: 0644, Size: int64(len(file.content))}); err != nil {
			return nil, err
		}
		if _, err := archive.Write([]byte(file.content)); err != nil {
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	if err := compressed.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
