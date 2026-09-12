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
	"io"
	"strings"
	"time"

	"github.com/goccy/go-json"
)

// ContentInfo is private persistence metadata, separate from the hot file/search
// index. Digests are learned during ordinary transfers, never by a remote scan.
type ContentInfo struct {
	Digest        string `json:"sha256,omitempty"`
	ETag          string `json:"etag,omitempty"`
	Size          int64  `json:"size"`
	ObjectSize    int64  `json:"object_size"`
	ModTime       int64  `json:"mod_time"`
	Reference     bool   `json:"reference,omitempty"`
	PreservedTime bool   `json:"preserved_time,omitempty"`
	Verified      bool   `json:"-"`
}

type ContentGarbage struct {
	Digest string
	Since  int64
}

func contentDigestKey(key, digest string) string {
	scope, _, _ := strings.Cut(key, "\x00")
	return scope + "\x00" + digest
}

func (idx *FileIndex) GetContent(key string) (ContentInfo, bool) { return idx.contents.Load(key) }

func (idx *FileIndex) PutContent(key string, value ContentInfo) {
	idx.contentMu.Lock()
	defer idx.contentMu.Unlock()
	if idx.contentRefs == nil {
		idx.contentRefs = map[string]int{}
		idx.contentVerifiedRefs = map[string]int{}
	}
	old, exists := idx.contents.Load(key)
	if exists && old == value {
		return
	}
	if exists {
		idx.dropContentReference(key, old)
	}
	idx.contents.Store(key, value)
	if value.Reference && value.Digest != "" {
		digestKey := contentDigestKey(key, value.Digest)
		idx.contentRefs[digestKey]++
		if value.Verified {
			idx.contentVerifiedRefs[digestKey]++
		}
		idx.contentGarbage.Delete(digestKey)
	}
	idx.IsDirty.Store(true)
}

func (idx *FileIndex) dropContentReference(key string, value ContentInfo) {
	if !value.Reference || value.Digest == "" {
		return
	}
	digestKey := contentDigestKey(key, value.Digest)
	idx.contentRefs[digestKey]--
	if value.Verified {
		idx.contentVerifiedRefs[digestKey]--
		if idx.contentVerifiedRefs[digestKey] <= 0 {
			delete(idx.contentVerifiedRefs, digestKey)
		}
	}
	if idx.contentRefs[digestKey] <= 0 {
		delete(idx.contentRefs, digestKey)
		idx.contentGarbage.Store(digestKey, time.Now().Unix())
	}
}

func (idx *FileIndex) RemoveContent(key string) {
	idx.contentMu.Lock()
	defer idx.contentMu.Unlock()
	if value, ok := idx.contents.LoadAndDelete(key); ok {
		idx.dropContentReference(key, value)
		idx.IsDirty.Store(true)
	}
}

func (idx *FileIndex) RemoveContentIfMatch(key string, expected ContentInfo) {
	idx.contentMu.Lock()
	defer idx.contentMu.Unlock()
	if current, ok := idx.contents.Load(key); ok && current == expected {
		idx.contents.Delete(key)
		idx.dropContentReference(key, current)
		idx.IsDirty.Store(true)
	}
}

func (idx *FileIndex) HasContent(digestKey string) bool {
	idx.contentMu.RLock()
	defer idx.contentMu.RUnlock()
	return idx.contentVerifiedRefs[digestKey] > 0
}

func (idx *FileIndex) MarkContentGarbage(key string, since int64) {
	idx.contentMu.Lock()
	defer idx.contentMu.Unlock()
	if idx.contentRefs[key] > 0 {
		return
	}
	if previous, ok := idx.contentGarbage.Load(key); !ok || since > previous {
		idx.contentGarbage.Store(key, since)
		idx.IsDirty.Store(true)
	}
}

func (idx *FileIndex) EnsureContentGarbage(key string, since int64) {
	idx.contentMu.Lock()
	defer idx.contentMu.Unlock()
	if idx.contentRefs[key] > 0 {
		return
	}
	if _, exists := idx.contentGarbage.LoadOrStore(key, since); !exists {
		idx.IsDirty.Store(true)
	}
}

func (idx *FileIndex) ForgetContentGarbage(key string, expected int64) {
	idx.contentMu.Lock()
	defer idx.contentMu.Unlock()
	if current, ok := idx.contentGarbage.Load(key); ok && current == expected {
		idx.contentGarbage.Delete(key)
		idx.IsDirty.Store(true)
	}
}

func (idx *FileIndex) ContentGarbage(scope string, limit int, before int64) []ContentGarbage {
	result := make([]ContentGarbage, 0, limit)
	prefix := scope + "\x00"
	idx.contentGarbage.Range(func(key string, since int64) bool {
		if since <= before && strings.HasPrefix(key, prefix) {
			result = append(result, ContentGarbage{strings.TrimPrefix(key, prefix), since})
		}
		return len(result) < limit
	})
	return result
}

func (idx *FileIndex) RangeContent(scope string, visit func(string, ContentInfo) bool) {
	prefix := scope + "\x00"
	idx.contents.Range(func(key string, value ContentInfo) bool {
		if after, ok := strings.CutPrefix(key, prefix); ok {
			return visit(after, value)
		}
		return true
	})
}

func (idx *FileIndex) readContentJSON(decoder *json.Decoder, garbage bool) error {
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if opening == nil {
		return nil
	}
	if opening != json.Delim('{') {
		return io.ErrUnexpectedEOF
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return io.ErrUnexpectedEOF
		}
		if garbage {
			var since int64
			if err := decoder.Decode(&since); err != nil {
				return err
			}
			idx.MarkContentGarbage(key, since)
		} else {
			var value ContentInfo
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			idx.PutContent(key, value)
		}
	}
	_, err = decoder.Token()
	return err
}
