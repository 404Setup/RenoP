//go:build linux && (amd64 || arm64)

/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package hex

import (
	"bytes"
	stdhex "encoding/hex"
	"os"
	"syscall"
	"testing"
)

// Guard pages detect vector overreads and overwrites, including assembly
// accesses that the race detector cannot instrument.
func TestGuardPages(t *testing.T) {
	page := os.Getpagesize()
	guarded := func() []byte {
		data, err := syscall.Mmap(-1, 0, page*3, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_PRIVATE|syscall.MAP_ANON)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := syscall.Munmap(data); err != nil {
				t.Error(err)
			}
		})
		if err := syscall.Mprotect(data[:page], syscall.PROT_NONE); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mprotect(data[page*2:], syscall.PROT_NONE); err != nil {
			t.Fatal(err)
		}
		return data[page : page*2 : page*2]
	}
	srcPage, dstPage := guarded(), guarded()
	for size := 0; size <= 129; size++ {
		for _, atEnd := range []bool{false, true} {
			window := func(data []byte, size int) []byte {
				if atEnd {
					return data[page-size:]
				}
				return data[:size:size]
			}
			src, dst := window(srcPage, size), window(dstPage, size*2)
			for i := range src {
				src[i] = byte(i*37 + size)
			}
			want := stdhex.EncodeToString(src)
			if n := Encode(dst, src); n != len(want) || string(dst) != want {
				t.Fatalf("encode size %d atEnd %v", size, atEnd)
			}
			// Odd and invalid inputs must also stay within exact slice bounds.
			for _, trim := range []int{0, 1} {
				encoded := window(srcPage, max(0, len(want)-trim))
				copy(encoded, want)
				dst = window(dstPage, len(encoded)/2)
				for _, invalid := range []bool{false, true} {
					if invalid && len(encoded) > 0 {
						encoded[len(encoded)-1] = '!'
					}
					want, we := stdhex.DecodeString(string(encoded))
					n, err := Decode(dst, encoded)
					if n != len(want) || err != we || !bytes.Equal(dst[:n], want) {
						t.Fatalf("decode size %d atEnd %v trim %d invalid %v: %v", size, atEnd, trim, invalid, err)
					}
				}
			}
		}
	}
}
