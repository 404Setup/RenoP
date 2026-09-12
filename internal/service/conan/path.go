/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package conan implements Conan's versioned recipe and binary metadata.
package conan

import "strings"

type Path struct {
	Recipe, RecipeRevision, Package, PackageRevision string
	Operation, File, Prefix                          string
}

func component(value string) bool {
	if len(value) == 0 || len(value) > 200 || value == "." || value == ".." {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_+.-", c)) {
			return false
		}
	}
	return true
}

// Parse accepts only protocol-owned routes. Files retain their complete native
// paths so downloads, deletion, capacity and storage backends share one owner.
func Parse(relative string) (Path, bool) {
	parts := strings.Split(relative, "/")
	if len(parts) < 6 || parts[0] != "v2" || parts[1] != "conans" {
		return Path{}, false
	}
	for _, value := range parts[2:6] {
		if !component(value) {
			return Path{}, false
		}
	}
	result := Path{Recipe: parts[2] + "/" + parts[3], Prefix: strings.Join(parts[:6], "/")}
	if parts[4] != "_" || parts[5] != "_" {
		result.Recipe += "@" + parts[4] + "/" + parts[5]
	}
	cursor := 6
	if len(parts) >= 8 && parts[6] == "revisions" && component(parts[7]) {
		result.RecipeRevision = parts[7]
		cursor = 8
		result.Prefix = strings.Join(parts[:cursor], "/")
	}
	if len(parts) > cursor && parts[cursor] == "packages" && result.RecipeRevision != "" {
		cursor++
		result.Prefix = strings.Join(parts[:cursor], "/")
		if len(parts) > cursor && component(parts[cursor]) {
			result.Package = parts[cursor]
			cursor++
			result.Prefix = strings.Join(parts[:cursor], "/")
			if len(parts) > cursor+1 && parts[cursor] == "revisions" && component(parts[cursor+1]) {
				result.PackageRevision = parts[cursor+1]
				cursor += 2
				result.Prefix = strings.Join(parts[:cursor], "/")
			}
		}
	}
	if len(parts) == cursor {
		result.Operation = "root"
		return result, true
	}
	if len(parts) == cursor+1 {
		switch parts[cursor] {
		case "revisions", "latest":
			if result.PackageRevision != "" || result.Package == "" && result.RecipeRevision != "" {
				return Path{}, false
			}
		case "search":
			if result.Package != "" {
				return Path{}, false
			}
		case "files":
			if result.RecipeRevision == "" || result.Package != "" && result.PackageRevision == "" {
				return Path{}, false
			}
		default:
			return Path{}, false
		}
		result.Operation = parts[cursor]
		return result, true
	}
	if parts[cursor] != "files" || result.RecipeRevision == "" || result.Package != "" && result.PackageRevision == "" {
		return Path{}, false
	}
	for _, value := range parts[cursor+1:] {
		if !component(value) {
			return Path{}, false
		}
	}
	result.File = strings.Join(parts[cursor+1:], "/")
	if len(result.File) > 1024 {
		return Path{}, false
	}
	result.Operation = "file"
	return result, true
}

func (p Path) Reference() string {
	ref := p.Recipe
	if p.RecipeRevision != "" {
		ref += "#" + p.RecipeRevision
	}
	if p.Package != "" {
		ref += ":" + p.Package
	}
	return ref
}
