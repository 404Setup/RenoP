/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package apt

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/gzip"

	"renop/pkg/hex"
)

// PackageLocation maps the conventional pool and dists layouts to a component.
// Files uploaded at the repository root belong to main in every suite.
func PackageLocation(filename string) (suite, component string) {
	parts := strings.Split(filename, "/")
	if len(parts) >= 4 && parts[0] == "dists" {
		return parts[1], parts[2]
	}
	if len(parts) >= 3 && parts[0] == "pool" {
		return "", parts[1]
	}
	return "", "main"
}

func PackageIndex(packages []*Package, suite, component, architecture string) ([]byte, error) {
	if len(packages) > 10000 {
		return nil, ErrInvalidPackage
	}
	var output strings.Builder
	for _, pkg := range packages {
		if pkg == nil {
			return nil, ErrInvalidPackage
		}
		pkgSuite, pkgComponent := PackageLocation(pkg.Filename)
		if (pkgSuite != "" && pkgSuite != suite) || (component != "" && pkgComponent != component) {
			continue
		}
		if architecture != "" && pkg.Fields["Architecture"] != architecture && pkg.Fields["Architecture"] != "all" {
			continue
		}
		keys := make([]string, 0, len(pkg.Fields))
		for key := range pkg.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := pkg.Fields[key]
			if output.Len()+len(key)+len(value)+4 > 32<<20 {
				return nil, ErrInvalidPackage
			}
			output.WriteString(key + ": " + value + "\n")
		}
		output.WriteByte('\n')
	}
	return []byte(output.String()), nil
}

func Gzip(data []byte) ([]byte, error) {
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// Release builds checksums from the exact same deterministic PackageIndex bytes
// served to clients. Publisher-uploaded Release/InRelease files remain opaque.
func Release(packages []*Package, suite string) ([]byte, error) {
	return ReleaseWithIndexes(packages, suite, nil)
}

// ReleaseWithIndexes lets the storage owner substitute publisher-uploaded
// index bytes, or omit an unavailable representation, before checksumming.
func ReleaseWithIndexes(packages []*Package, suite string, resolve func(string, []byte) ([]byte, bool, error)) ([]byte, error) {
	if suite == "" || strings.ContainsAny(suite, " \t\r\n/\\") {
		return nil, ErrInvalidPackage
	}
	components := map[string]bool{"main": true}
	architectures := map[string]bool{"amd64": true, "arm64": true}
	for _, pkg := range packages {
		if pkg == nil {
			return nil, ErrInvalidPackage
		}
		pkgSuite, component := PackageLocation(pkg.Filename)
		if component == "" || strings.ContainsAny(component, " \t\r\n") {
			return nil, ErrInvalidPackage
		}
		if pkgSuite != "" && pkgSuite != suite {
			continue
		}
		components[component] = true
		arch := pkg.Fields["Architecture"]
		if arch != "all" {
			architectures[arch] = true
		}
	}
	if len(components) > 32 || len(architectures) > 32 {
		return nil, ErrInvalidPackage
	}
	componentNames := sortedKeys(components)
	architectureNames := sortedKeys(architectures)
	var output strings.Builder
	fmt.Fprintf(&output, "Origin: RenoP\nLabel: RenoP\nSuite: %s\nCodename: %s\nDate: %s\nArchitectures: %s\nComponents: %s\nSHA256:\n", suite, suite, time.Now().UTC().Truncate(time.Hour).Format(time.RFC1123Z), strings.Join(architectureNames, " "), strings.Join(componentNames, " "))
	totalBytes := 0
	for _, component := range componentNames {
		for _, architecture := range architectureNames {
			data, err := PackageIndex(packages, suite, component, architecture)
			if err != nil {
				return nil, err
			}
			compressed, err := Gzip(data)
			if err != nil {
				return nil, err
			}
			for _, file := range []struct {
				name string
				data []byte
			}{{"Packages", data}, {"Packages.gz", compressed}} {
				filename := path.Join(component, "binary-"+architecture, file.name)
				if resolve != nil {
					var include bool
					file.data, include, err = resolve(filename, file.data)
					if err != nil {
						return nil, err
					}
					if !include {
						continue
					}
				}
				totalBytes += len(file.data)
				if totalBytes > 64<<20 {
					return nil, ErrInvalidPackage
				}
				sum := sha256.Sum256(file.data)
				fmt.Fprintf(&output, " %s %d %s\n", hex.EncodeToString(sum[:]), len(file.data), filename)
			}
		}
	}
	return []byte(output.String()), nil
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
