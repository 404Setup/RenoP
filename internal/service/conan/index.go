/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package conan

import (
	"errors"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("Conan metadata is unavailable")
var ErrNotFound = errors.New("Conan revision not found")

type File struct {
	Path          string
	Size, ModTime int64
}
type Revision struct {
	Revision string `json:"revision"`
	Time     string `json:"time"`
}
type Store interface {
	Files(prefix string) ([]File, error)
	ReadFile(path string, limit int64) ([]byte, error)
}

func Search(store Store, pattern string, ignoreCase bool) ([]string, error) {
	if len(pattern) > 512 {
		return nil, ErrUnavailable
	}
	if pattern == "" {
		pattern = "*"
	}
	expression := "^" + strings.NewReplacer(`\*`, ".*", `\?`, ".").Replace(regexp.QuoteMeta(pattern)) + "$"
	if ignoreCase {
		expression = "(?i)" + expression
	}
	matcher, err := regexp.Compile(expression)
	if err != nil {
		return nil, ErrUnavailable
	}
	files, err := store.Files("v2/conans")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, file := range files {
		p, ok := Parse(file.Path)
		if !ok || p.File != "conanmanifest.txt" || p.Package != "" {
			continue
		}
		// Conan wildcards span reference separators, unlike filesystem globs.
		if matcher.MatchString(p.Recipe) {
			set[p.Recipe] = true
		}
		if len(set) > 10000 {
			return nil, ErrUnavailable
		}
	}
	result := make([]string, 0, len(set))
	for ref := range set {
		result = append(result, ref)
	}
	sort.Strings(result)
	return result, nil
}

func Revisions(store Store, p Path) ([]Revision, error) {
	files, err := store.Files(p.Prefix + "/revisions")
	if err != nil {
		return nil, err
	}
	stamps := map[string]int64{}
	for _, file := range files {
		candidate, ok := Parse(file.Path)
		if !ok || candidate.File != "conanmanifest.txt" {
			continue
		}
		revision := candidate.RecipeRevision
		if p.Package != "" {
			if candidate.Package != p.Package {
				continue
			}
			revision = candidate.PackageRevision
		} else if candidate.Package != "" {
			continue
		}
		if revision != "" && file.ModTime > stamps[revision] {
			stamps[revision] = file.ModTime
		}
	}
	if len(stamps) == 0 {
		return nil, ErrNotFound
	}
	result := make([]Revision, 0, len(stamps))
	for revision, stamp := range stamps {
		result = append(result, Revision{revision, time.Unix(0, stamp).UTC().Format(time.RFC3339Nano)})
	}
	sort.Slice(result, func(i, j int) bool {
		if stamps[result[i].Revision] != stamps[result[j].Revision] {
			return stamps[result[i].Revision] > stamps[result[j].Revision]
		}
		return result[i].Revision > result[j].Revision
	})
	return result, nil
}

func FileList(store Store, p Path) (map[string]struct{}, error) {
	files, err := store.Files(p.Prefix + "/files")
	if err != nil {
		return nil, err
	}
	result := map[string]struct{}{}
	for _, file := range files {
		candidate, ok := Parse(file.Path)
		if !ok || candidate.File == "" {
			continue
		}
		if isChecksum(candidate.File) {
			continue
		}
		result[candidate.File] = struct{}{}
	}
	if _, ok := result["conanmanifest.txt"]; !ok {
		return nil, ErrNotFound
	}
	return result, nil
}

func isChecksum(filename string) bool {
	switch path.Ext(filename) {
	case ".md5", ".sha1", ".sha256", ".sha512":
		return true
	}
	return false
}

func Packages(store Store, p Path, listOnly bool) (map[string]any, error) {
	if p.RecipeRevision == "" {
		revisions, err := Revisions(store, p)
		if err != nil {
			return nil, err
		}
		p.RecipeRevision = revisions[0].Revision
		p.Prefix += "/revisions/" + p.RecipeRevision
	}
	files, err := store.Files(p.Prefix + "/packages")
	if err != nil {
		return nil, err
	}
	latest := map[string]File{}
	for _, file := range files {
		candidate, ok := Parse(file.Path)
		if !ok || candidate.Package == "" || candidate.File != "conanmanifest.txt" {
			continue
		}
		old, exists := latest[candidate.Package]
		if !exists || file.ModTime > old.ModTime {
			latest[candidate.Package] = file
		}
	}
	result := make(map[string]any, len(latest))
	if !listOnly && len(latest) > 1000 {
		return nil, ErrUnavailable
	}
	for id, file := range latest {
		if listOnly {
			result[id] = struct{}{}
			continue
		}
		data, err := store.ReadFile(strings.TrimSuffix(file.Path, "conanmanifest.txt")+"conaninfo.txt", 1<<20)
		if err != nil {
			return nil, err
		}
		result[id] = packageInfo(string(data))
	}
	return result, nil
}

func packageInfo(data string) map[string]any {
	settings, options := map[string]string{}, map[string]string{}
	requires := []string{}
	section := ""
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}
		if line == "" {
			continue
		}
		switch section {
		case "settings", "options":
			key, value, ok := strings.Cut(line, "=")
			if ok {
				if section == "settings" {
					settings[key] = value
				} else {
					options[key] = value
				}
			}
		case "requires":
			requires = append(requires, line)
		}
	}
	return map[string]any{"settings": settings, "options": options, "requires": requires}
}
