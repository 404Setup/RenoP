/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package id provides compact, secure, database-friendly, time-ordered unique identifiers.
// IDs are based on 128-bit UUIDv7 (RFC 9562) encoded in a 22-character ASCII-sorted Base62 string.
// This guarantees lexicographical time-ordering, zero B-Tree index fragmentation, high cryptographic entropy,
// and seamless bidirectional conversion with standard 128-bit UUIDs.
package id

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math/bits"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// EncodedLength is the fixed character length of an encoded compact ID.
	EncodedLength = 22

	// base62Chars is sorted in strict ASCII order ('0'-'9', 'A'-'Z', 'a'-'z') so that
	// string comparison matches numeric and chronological order for big-endian 128-bit values.
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

var (
	// ErrInvalidID is returned when an ID string is malformed or out of range.
	ErrInvalidID = errors.New("invalid identifier")

	// decodeMap maps ASCII byte values to their 6-bit Base62 values, or 0xFF for invalid bytes.
	decodeMap [256]uint8
)

func init() {
	for i := range decodeMap {
		decodeMap[i] = 0xFF
	}
	for i := range len(base62Chars) {
		decodeMap[base62Chars[i]] = uint8(i)
	}
}

// New generates a new compact, database-friendly, cryptographically secure 22-character ID.
// The ID is time-ordered with millisecond precision and contains 80 bits of cryptographic randomness.
func New() string {
	u, err := uuid.NewV7()
	if err != nil {
		return newFallback()
	}
	return Encode(u)
}

func newFallback() string {
	var b [16]byte
	now := time.Now().UnixMilli()
	b[0] = byte(now >> 40)
	b[1] = byte(now >> 32)
	b[2] = byte(now >> 24)
	b[3] = byte(now >> 16)
	b[4] = byte(now >> 8)
	b[5] = byte(now)
	_, _ = rand.Read(b[6:])
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	return Encode(b)
}

// Encode encodes a 16-byte UUID / 128-bit integer into a 22-character Base62 string.
func Encode(u [16]byte) string {
	hi := binary.BigEndian.Uint64(u[0:8])
	lo := binary.BigEndian.Uint64(u[8:16])

	var buf [EncodedLength]byte
	for i := EncodedLength - 1; i >= 0; i-- {
		var rem uint64
		hi, rem = hi/62, hi%62
		lo, rem = bits.Div64(rem, lo, 62)
		buf[i] = base62Chars[rem]
	}
	return string(buf[:])
}

// Decode decodes a 22-character Base62 string into a 16-byte UUID.
func Decode(s string) ([16]byte, error) {
	if len(s) != EncodedLength {
		return [16]byte{}, ErrInvalidID
	}
	var hi, lo uint64
	for i := range EncodedLength {
		val := decodeMap[s[i]]
		if val == 0xFF {
			return [16]byte{}, ErrInvalidID
		}
		pLoHi, pLoLo := bits.Mul64(lo, 62)
		pHiHi, pHiLo := bits.Mul64(hi, 62)
		if pHiHi > 0 {
			return [16]byte{}, ErrInvalidID
		}
		var carry uint64
		lo, carry = bits.Add64(pLoLo, uint64(val), 0)
		hi, carry = bits.Add64(pHiLo, pLoHi, carry)
		if carry > 0 {
			return [16]byte{}, ErrInvalidID
		}
	}
	var out [16]byte
	binary.BigEndian.PutUint64(out[0:8], hi)
	binary.BigEndian.PutUint64(out[8:16], lo)
	return out, nil
}

// IsValid reports whether s is a valid 22-character compact ID.
func IsValid(s string) bool {
	if len(s) != EncodedLength {
		return false
	}
	_, err := Decode(s)
	return err == nil
}

// IsValidOrUUID reports whether s is either a valid 22-character compact ID
// or a valid legacy 36-character UUID string.
func IsValidOrUUID(s string) bool {
	s = strings.TrimSpace(s)
	if IsValid(s) {
		return true
	}
	return uuid.Validate(s) == nil
}

// Validate returns nil if s is a valid compact ID or legacy UUID string,
// or ErrInvalidID if invalid.
func Validate(s string) error {
	if IsValidOrUUID(s) {
		return nil
	}
	return ErrInvalidID
}

// FromUUID converts a uuid.UUID into a 22-character compact ID.
func FromUUID(u uuid.UUID) string {
	return Encode(u)
}

// FromUUIDString parses a legacy UUID string and converts it to a 22-character compact ID.
func FromUUIDString(s string) (string, error) {
	u, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return "", err
	}
	return Encode(u), nil
}

// ToUUID decodes a 22-character compact ID into a uuid.UUID.
func ToUUID(s string) (uuid.UUID, error) {
	b, err := Decode(s)
	if err != nil {
		return uuid.Nil, err
	}
	return b, nil
}

// Normalize converts a legacy UUID string into a compact ID if applicable.
// If s is already a valid compact ID, it returns s unchanged.
func Normalize(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if IsValid(s) {
		return s, true
	}
	if u, err := uuid.Parse(s); err == nil {
		return Encode(u), true
	}
	return "", false
}
