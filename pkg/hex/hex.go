/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package hex implements hexadecimal encoding and decoding compatible with
// encoding/hex, with platform-specific acceleration for complete blocks.
package hex

import (
	stdhex "encoding/hex"
	"io"
	"slices"
)

// ErrLength reports an attempt to decode an odd-length input using Decode or
// DecodeString. NewDecoder returns io.ErrUnexpectedEOF instead.
var ErrLength = stdhex.ErrLength

// InvalidByteError describes an invalid byte in a hexadecimal string.
type InvalidByteError = stdhex.InvalidByteError

// EncodedLen returns the length of an encoding of n source bytes: n * 2.
func EncodedLen(n int) int { return stdhex.EncodedLen(n) }

// DecodedLen returns the length of a decoding of n source bytes: n / 2.
func DecodedLen(n int) int { return stdhex.DecodedLen(n) }

// Encode encodes src into EncodedLen(len(src)) bytes of dst and returns the
// number of bytes written. It writes lowercase hexadecimal characters.
func Encode(dst, src []byte) int {
	n := encodeBlocks(dst, src)
	return n*2 + stdhex.Encode(dst[n*2:], src[n:])
}

// Decode decodes src into DecodedLen(len(src)) bytes of dst. It accepts upper-
// and lowercase hexadecimal characters and requires an even input length.
// On malformed input, it returns the number of bytes decoded before the error.
func Decode(dst, src []byte) (int, error) {
	n := decodeBlocks(dst, src)
	// The backend never writes an invalid block. Let the standard decoder
	// determine its exact valid prefix and error, including odd-length input.
	written, err := stdhex.Decode(dst[n/2:], src[n:])
	return n/2 + written, err
}

// AppendEncode appends the hexadecimally encoded src to dst.
func AppendEncode(dst, src []byte) []byte {
	n := EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}

// AppendDecode appends the hexadecimally decoded src to dst. On malformed
// input, it returns the extended buffer containing the valid prefix and an error.
func AppendDecode(dst, src []byte) ([]byte, error) {
	n := DecodedLen(len(src))
	dst = slices.Grow(dst, n)
	n, err := Decode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n], err
}

// EncodeToString returns the hexadecimal encoding of src.
func EncodeToString(src []byte) string {
	dst := make([]byte, EncodedLen(len(src)))
	Encode(dst, src)
	return string(dst)
}

// DecodeString returns the bytes represented by the hexadecimal string s.
// On malformed input, it returns the bytes decoded before the error.
func DecodeString(s string) ([]byte, error) {
	dst := make([]byte, DecodedLen(len(s)))
	n, err := Decode(dst, []byte(s))
	return dst[:n], err
}

// Dump returns a hex dump of data in the format of hexdump -C.
func Dump(data []byte) string { return stdhex.Dump(data) }

// Dumper returns a writer that writes a hex dump to w in the format of hexdump -C.
func Dumper(w io.Writer) io.WriteCloser { return stdhex.Dumper(w) }
