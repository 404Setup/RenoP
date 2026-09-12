/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package index

import (
	"time"

	"renop/internal/cache"
	"renop/internal/cache/ttl"
)

const maxNegativeEntries = 10000

func (idx *FileIndex) negativeCache() *ttl.TTLCache[string, int64] {
	idx.negativeOnce.Do(func() {
		idx.negative = ttl.NewTTLCacheWithCapacity[string, int64](time.Minute, maxNegativeEntries)
	})
	return idx.negative
}

// UseRemoteCache binds disposable index lookup data before serving requests.
// The file/directory index and private content references remain authoritative
// locally; losing the remote cache only causes another storage lookup.
func (idx *FileIndex) UseRemoteCache(remote *cache.Remote) {
	idx.negativeCache().UseRemote(remote, func(expiry int64) int64 { return expiry })
}

func (idx *FileIndex) storeNotFound(path string, expires int64) {
	c := idx.negativeCache()
	generation := c.Generation()
	if expires <= time.Now().Unix() || idx.HasFile(path) || idx.HasDir(path) {
		return
	}
	c.SetIfGeneration(path, expires, time.Until(time.Unix(expires, 0)), generation)
}
