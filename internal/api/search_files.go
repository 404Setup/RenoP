/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package api

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/index"
	"renop/internal/service/maven"
	"renop/pkg/pb"
)

// fileSearchMatch holds only the best bounded candidates, before allocating wire messages.
type fileSearchMatch struct {
	name, path, foldedPath string
	rank                   int
	directory              bool
	info                   index.FileInfo
}

func (left fileSearchMatch) betterThan(right fileSearchMatch) bool {
	if left.rank != right.rank {
		return left.rank < right.rank
	}
	if len(left.path) != len(right.path) {
		return len(left.path) < len(right.path)
	}
	if left.foldedPath != right.foldedPath {
		return left.foldedPath < right.foldedPath
	}
	return left.path < right.path
}

func searchFileTreeRepository(state *core.AppState, storagePath string, repo *config.Repository, user *config.User, query string, limit int) (*pb.RepositorySearchResponse, error) {
	var visible func(string) bool
	if repo.NormalizedFormat() == config.RepositoryFormatMaven {
		var err error
		visible, err = maven.MetadataPathFilter(state, user, repo.Name)
		if err != nil {
			return nil, err
		}
	}
	root := filepath.ToSlash(filepath.Clean(filepath.Join(storagePath, repo.Name)))
	rootPrefix := root + "/"
	needle := strings.ToLower(query)
	matches := make([]fileSearchMatch, 0, limit)
	total, visited := 0, 0
	scanLimitReached := false
	state.Inner.FileIndex.Walk(root, func(indexedPath string, info index.FileInfo, directory bool) bool {
		visited++
		if visited > maxRepositorySearchScan {
			scanLimitReached = true
			return false
		}
		if indexedPath == root || state.Inner.FileIndex.IsBlocked(indexedPath) {
			return true
		}
		if !strings.HasPrefix(indexedPath, rootPrefix) {
			return true
		}
		relative := indexedPath[len(rootPrefix):]
		folded := strings.ToLower(relative)
		if relative == "" || !strings.Contains(folded, needle) ||
			!user.CheckReadPermission(repo.Name, relative, repo.Visibility, directory) || visible != nil && !visible(relative) {
			return true
		}
		total++
		name := relative[strings.LastIndexByte(relative, '/')+1:]
		candidate := fileSearchMatch{name: name, path: relative, foldedPath: folded,
			rank: repositorySearchRank(name, needle), directory: directory, info: info}
		at := sort.Search(len(matches), func(i int) bool { return candidate.betterThan(matches[i]) })
		if at >= limit {
			return true
		}
		if len(matches) < limit {
			matches = append(matches, fileSearchMatch{})
		}
		copy(matches[at+1:], matches[at:len(matches)-1])
		matches[at] = candidate
		return true
	})
	results := make([]*pb.RepositorySearchResult, 0, len(matches))
	for _, match := range matches {
		result := &pb.RepositorySearchResult{Name: match.name, Path: match.path, Type: "DIRECTORY"}
		if !match.directory {
			result.Type = "FILE"
			result.Size = match.info.Size
			result.ModifiedAt = time.Unix(0, match.info.ModTime).UnixMilli()
		}
		results = append(results, result)
	}
	return &pb.RepositorySearchResponse{Format: repo.ConfiguredFormat(), Results: results, Total: int32(total),
		HasMore: scanLimitReached || total > len(results)}, nil
}
