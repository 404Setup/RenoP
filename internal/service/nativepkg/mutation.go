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

import "sync"

var mutationStripes [64]sync.Mutex

// AcquireAllMutations protects administrative deletion of uncatalogued trees.
func AcquireAllMutations() func() {
	for index := range mutationStripes {
		mutationStripes[index].Lock()
	}
	return func() {
		for index := len(mutationStripes) - 1; index >= 0; index-- {
			mutationStripes[index].Unlock()
		}
	}
}

// AcquireMutation serializes resource uploads, permission changes and review
// decisions. Callers acquire the repository gate first. Fixed stripes bound
// bookkeeping even when untrusted clients submit many different identities.
func AcquireMutation(repository, name string) func() {
	hash := uint32(2166136261)
	for _, value := range []string{repository, "\x00", name} {
		for index := range len(value) {
			hash = (hash ^ uint32(value[index])) * 16777619
		}
	}
	lock := &mutationStripes[hash%uint32(len(mutationStripes))]
	lock.Lock()
	return lock.Unlock
}
