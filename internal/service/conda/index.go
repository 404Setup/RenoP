/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package conda

import (
	"strings"

	"github.com/goccy/go-json"
)

const MaxIndexPackages = 10000

// Repodata implements CEP 36. Both package containers coexist in one channel;
// current_repodata may use this complete index without changing solver results.
func Repodata(subdir string, packages []*Package) ([]byte, error) {
	if subdir == "" || strings.ContainsAny(subdir, "/\\\x00\r\n") || len(packages) > MaxIndexPackages {
		return nil, ErrInvalidPackage
	}
	legacy := make(map[string]map[string]json.RawMessage)
	native := make(map[string]map[string]json.RawMessage)
	totalBytes := 0
	for _, pkg := range packages {
		if pkg == nil {
			return nil, ErrInvalidPackage
		}
		for key, value := range pkg.Record {
			totalBytes += len(key) + len(value) + 8
		}
		if totalBytes > 32<<20 {
			return nil, ErrInvalidPackage
		}
		if raw, ok := pkg.Record["subdir"]; ok {
			var declared string
			if json.Unmarshal(raw, &declared) != nil || (declared != "" && declared != subdir) {
				return nil, ErrInvalidPackage
			}
		}
		if strings.HasSuffix(pkg.Filename, ".conda") {
			native[pkg.Filename] = pkg.Record
		} else {
			legacy[pkg.Filename] = pkg.Record
		}
	}
	return json.Marshal(struct {
		Info     map[string]string                     `json:"info"`
		Packages map[string]map[string]json.RawMessage `json:"packages"`
		Conda    map[string]map[string]json.RawMessage `json:"packages.conda"`
		Removed  []string                              `json:"removed"`
		Version  int                                   `json:"repodata_version"`
	}{map[string]string{"subdir": subdir}, legacy, native, []string{}, 1})
}
