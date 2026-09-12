//go:build linux && !purego && (arm64 || (amd64 && goexperiment.simd))

/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package base32

import (
	"bytes"
	stdbase32 "encoding/base32"
	"os"
	"syscall"
	"testing"
)

// Guard pages catch assembly overreads and overwrites that allocation canaries
// cannot detect. Only the exact source and destination bytes are accessible.
func TestBlockGuardPages(t *testing.T) {
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
	for size := 16; size <= 128; size++ {
		src := srcPage[page-size:]
		for i := range src {
			src[i] = byte(i*37 + size)
		}
		want := stdbase32.StdEncoding.EncodeToString(src)
		dst := dstPage[page-len(want):]
		consumed := StdEncoding.encodeBlocks(dst, src)
		if string(dst[:consumed/5*8]) != want[:consumed/5*8] {
			t.Fatalf("encode at guard page, size %d", size)
		}
		// Decode directly against the guard boundary, including an invalid
		// final block to check that it is left for the scalar decoder.
		encoded := srcPage[page-len(want):]
		copy(encoded, want)
		dst = dstPage[page-StdEncoding.DecodedLen(len(encoded)):]
		for _, malformed := range []bool{false, true} {
			if malformed {
				encoded[len(encoded)-1] = '!'
			}
			n := StdEncoding.decodeBlocks(dst, encoded)
			expected, err := stdbase32.StdEncoding.DecodeString(string(encoded[:n]))
			if err != nil || !bytes.Equal(dst[:n/8*5], expected) {
				t.Fatalf("decode at guard page, size %d: %v", size, err)
			}
		}
	}
}
