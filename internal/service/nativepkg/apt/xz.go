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
	"encoding/binary"
	"io"

	"github.com/ulikunitz/xz"
)

// boundedXZ validates every block's dictionary before allocating the decoder.
// xz.ReaderConfig.DictCap is a minimum, not a maximum; limiting decoded bytes
// alone does not prevent a tiny archive from requesting a multi-gigabyte window.
func boundedXZ(reader io.ReaderAt, size int64) (io.Reader, error) {
	if size < 32 || size > maxControlBytes {
		return nil, ErrInvalidPackage
	}
	var footer [12]byte
	if _, err := reader.ReadAt(footer[:], size-12); err != nil {
		return nil, err
	}
	if footer[10] != 'Y' || footer[11] != 'Z' {
		return nil, ErrInvalidPackage
	}
	indexSize := (int64(binary.LittleEndian.Uint32(footer[4:8])) + 1) * 4
	if indexSize < 8 || indexSize > size-24 || indexSize > 128<<10 {
		return nil, ErrInvalidPackage
	}
	indexOffset := size - 12 - indexSize
	data := make([]byte, indexSize)
	if _, err := reader.ReadAt(data, indexOffset); err != nil {
		return nil, err
	}
	if data[0] != 0 {
		return nil, ErrInvalidPackage
	}
	position := 1
	count, ok := xzVLI(data[:len(data)-4], &position)
	if !ok || count > 4096 {
		return nil, ErrInvalidPackage
	}
	blockOffset := int64(12)
	uncompressedTotal := uint64(0)
	for range count {
		unpadded, ok := xzVLI(data[:len(data)-4], &position)
		if !ok || unpadded < 8 || unpadded > uint64(indexOffset-blockOffset) {
			return nil, ErrInvalidPackage
		}
		uncompressed, ok := xzVLI(data[:len(data)-4], &position)
		if !ok || uncompressed > maxControlBytes || uncompressedTotal > maxControlBytes-uncompressed {
			return nil, ErrInvalidPackage
		}
		uncompressedTotal += uncompressed
		var first [1]byte
		if _, err := reader.ReadAt(first[:], blockOffset); err != nil {
			return nil, err
		}
		headerSize := (int(first[0]) + 1) * 4
		if first[0] == 0 || uint64(headerSize) > unpadded {
			return nil, ErrInvalidPackage
		}
		header := make([]byte, headerSize)
		if _, err := reader.ReadAt(header, blockOffset); err != nil {
			return nil, err
		}
		flags := header[1]
		if flags&0x3c != 0 {
			return nil, ErrInvalidPackage
		}
		cursor := 2
		for _, flag := range []byte{0x40, 0x80} {
			if flags&flag != 0 {
				if _, ok := xzVLI(header[:len(header)-4], &cursor); !ok {
					return nil, ErrInvalidPackage
				}
			}
		}
		for filter := 0; filter <= int(flags&3); filter++ {
			id, ok := xzVLI(header[:len(header)-4], &cursor)
			if !ok {
				return nil, ErrInvalidPackage
			}
			length, ok := xzVLI(header[:len(header)-4], &cursor)
			if !ok || length > uint64(len(header)-4-cursor) {
				return nil, ErrInvalidPackage
			}
			// This decoder supports LZMA2. A dictionary property above 28 is
			// larger than the complete 64 MiB control archive work budget.
			if id != 0x21 || length != 1 || header[cursor] > 28 {
				return nil, ErrInvalidPackage
			}
			cursor += int(length)
		}
		blockOffset += int64((unpadded + 3) &^ 3)
	}
	if blockOffset != indexOffset {
		return nil, ErrInvalidPackage
	}
	return (xz.ReaderConfig{SingleStream: true}).NewReader(io.NewSectionReader(reader, 0, size))
}

func xzVLI(data []byte, position *int) (uint64, bool) {
	var value uint64
	for shift := uint(0); shift < 63 && *position < len(data); shift += 7 {
		b := data[*position]
		*position++
		value |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, shift == 0 || b != 0
		}
	}
	return 0, false
}
