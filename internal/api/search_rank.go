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

import "strings"

// containsFold matches a normalized query using Unicode case folding.
func containsFold(value, queryLower string) bool {
	return strings.Contains(strings.ToLower(value), queryLower)
}

func repositorySearchRank(name, queryLower string) int {
	normalized := strings.ToLower(name)
	if normalized == queryLower {
		return 0
	}
	if strings.HasPrefix(normalized, queryLower) {
		return 1
	}
	return 2
}
