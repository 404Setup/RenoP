/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package rpm implements RPM package metadata and rpm-md repository indexes.
package rpm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var ErrInvalidPackage = errors.New("invalid RPM package metadata")

type headerEntry struct{ kind, offset, count uint32 }
type header struct {
	entries map[uint32]headerEntry
	data    []byte
	end     int64
}

func readHeader(reader io.ReaderAt, size, offset int64) (*header, error) {
	var prefix [16]byte
	if offset < 0 || offset > size-16 {
		return nil, ErrInvalidPackage
	}
	if _, err := reader.ReadAt(prefix[:], offset); err != nil {
		return nil, err
	}
	if !bytes.Equal(prefix[:4], []byte{0x8e, 0xad, 0xe8, 1}) {
		return nil, ErrInvalidPackage
	}
	count, length := binary.BigEndian.Uint32(prefix[8:12]), binary.BigEndian.Uint32(prefix[12:16])
	if count > 4096 || length > 16<<20 || int64(count)*16+int64(length) > size-offset-16 {
		return nil, ErrInvalidPackage
	}
	data := make([]byte, int(count)*16+int(length))
	if _, err := reader.ReadAt(data, offset+16); err != nil {
		return nil, err
	}
	result := &header{entries: make(map[uint32]headerEntry, count), data: data[int(count)*16:], end: offset + 16 + int64(len(data))}
	for i := range count {
		entry := data[i*16 : i*16+16]
		tag := binary.BigEndian.Uint32(entry[:4])
		value := headerEntry{binary.BigEndian.Uint32(entry[4:8]), binary.BigEndian.Uint32(entry[8:12]), binary.BigEndian.Uint32(entry[12:])}
		if _, duplicate := result.entries[tag]; duplicate || value.offset > length || value.count > 65536 {
			return nil, ErrInvalidPackage
		}
		result.entries[tag] = value
	}
	return result, nil
}

func (h *header) strings(tag uint32) ([]string, error) {
	entry, ok := h.entries[tag]
	if !ok {
		return nil, nil
	}
	if entry.kind != 6 && entry.kind != 8 && entry.kind != 9 {
		return nil, ErrInvalidPackage
	}
	if entry.kind == 6 && entry.count != 1 {
		return nil, ErrInvalidPackage
	}
	data := h.data[entry.offset:]
	values := make([]string, 0, entry.count)
	for i := uint32(0); i < entry.count; i++ {
		end := bytes.IndexByte(data, 0)
		if end < 0 || end > 1<<20 {
			return nil, ErrInvalidPackage
		}
		values = append(values, string(data[:end]))
		data = data[end+1:]
	}
	return values, nil
}

func (h *header) numbers(tag uint32) ([]uint64, error) {
	entry, ok := h.entries[tag]
	if !ok {
		return nil, nil
	}
	var width uint32
	switch entry.kind {
	case 2:
		width = 1
	case 3:
		width = 2
	case 4:
		width = 4
	case 5:
		width = 8
	default:
		return nil, ErrInvalidPackage
	}
	if uint64(entry.count)*uint64(width) > uint64(len(h.data))-uint64(entry.offset) {
		return nil, ErrInvalidPackage
	}
	values := make([]uint64, entry.count)
	for i := range values {
		data := h.data[entry.offset+uint32(i)*width:]
		switch width {
		case 1:
			values[i] = uint64(data[0])
		case 2:
			values[i] = uint64(binary.BigEndian.Uint16(data))
		case 4:
			values[i] = uint64(binary.BigEndian.Uint32(data))
		case 8:
			values[i] = binary.BigEndian.Uint64(data)
		}
	}
	return values, nil
}
