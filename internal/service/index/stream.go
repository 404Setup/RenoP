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
	"errors"
	"io"

	"github.com/goccy/go-json"
)

type indexStreamHeader struct {
	Version int `json:"version"`
}
type indexStreamRecord struct {
	Kind    string       `json:"kind"`
	Path    string       `json:"path"`
	File    *FileInfo    `json:"file,omitempty"`
	Content *ContentInfo `json:"content,omitempty"`
	Since   int64        `json:"since,omitempty"`
}

func (idx *FileIndex) writeJSONStream(writer io.Writer, persistent bool) error {
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(indexStreamHeader{Version: 2}); err != nil {
		return err
	}
	var streamErr error
	idx.Files.Range(func(path string, info FileInfo) bool {
		if idx.IsBlocked(path) {
			return true
		}
		streamErr = encoder.Encode(indexStreamRecord{Kind: "file", Path: path, File: &info})
		return streamErr == nil
	})
	if streamErr != nil {
		return streamErr
	}
	idx.Dirs.Range(func(path string, _ bool) bool {
		streamErr = encoder.Encode(indexStreamRecord{Kind: "dir", Path: path})
		return streamErr == nil
	})
	if streamErr != nil || !persistent {
		return streamErr
	}
	idx.contents.Range(func(key string, info ContentInfo) bool {
		streamErr = encoder.Encode(indexStreamRecord{Kind: "content", Path: key, Content: &info})
		return streamErr == nil
	})
	if streamErr != nil {
		return streamErr
	}
	idx.contentGarbage.Range(func(key string, since int64) bool {
		streamErr = encoder.Encode(indexStreamRecord{Kind: "content_garbage", Path: key, Since: since})
		return streamErr == nil
	})
	return streamErr
}

func (idx *FileIndex) readJSONStream(decoder *json.Decoder) error {
	for {
		var record indexStreamRecord
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if record.Path == "" {
			return errors.New("index record requires a path")
		}
		switch record.Kind {
		case "file":
			if record.File == nil {
				return errors.New("index file record requires metadata")
			}
			idx.InsertFile(record.Path, *record.File)
		case "dir":
			idx.InsertDir(record.Path)
		case "content":
			if record.Content == nil {
				return errors.New("index content record requires metadata")
			}
			idx.PutContent(record.Path, *record.Content)
		case "content_garbage":
			idx.MarkContentGarbage(record.Path, record.Since)
		default:
			return errors.New("unknown index record kind")
		}
	}
}
