/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package id

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewID(t *testing.T) {
	seen := make(map[string]bool)
	var prev string
	for i := range 1000 {
		cur := New()
		if len(cur) != EncodedLength {
			t.Fatalf("expected length %d, got %d (%s)", EncodedLength, len(cur), cur)
		}
		if !IsValid(cur) {
			t.Fatalf("expected IsValid to be true for %s", cur)
		}
		if seen[cur] {
			t.Fatalf("duplicate ID generated: %s", cur)
		}
		seen[cur] = true
		if prev != "" && i%100 == 0 {
			time.Sleep(2 * time.Millisecond)
			next := New()
			if strings.Compare(prev, next) >= 0 {
				t.Fatalf("expected monotonic time ordering: prev %s >= next %s", prev, next)
			}
			prev = next
		} else if prev == "" {
			prev = cur
		}
	}
}

func TestRoundTrip(t *testing.T) {
	testUUIDs := []uuid.UUID{
		uuid.Nil,
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff"),
		uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		uuid.MustParse("0189a690-6c9b-7901-b5cb-7ff2308c3d90"),
		uuid.New(),
	}

	for _, u := range testUUIDs {
		encoded := Encode(u)
		if len(encoded) != EncodedLength {
			t.Fatalf("encoded length = %d, want %d", len(encoded), EncodedLength)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("failed to decode %s: %v", encoded, err)
		}
		if uuid.UUID(decoded) != u {
			t.Fatalf("roundtrip mismatch: want %s, got %s", u, uuid.UUID(decoded))
		}

		// Also check FromUUID / ToUUID
		fromU := FromUUID(u)
		if fromU != encoded {
			t.Fatalf("FromUUID != Encode: %s vs %s", fromU, encoded)
		}
		toU, err := ToUUID(encoded)
		if err != nil || toU != u {
			t.Fatalf("ToUUID mismatch: %v, %v", toU, err)
		}
	}
}

func TestFromUUIDStringAndNormalize(t *testing.T) {
	orig := "550e8400-e29b-41d4-a716-446655440000"
	compact, err := FromUUIDString(orig)
	if err != nil {
		t.Fatalf("FromUUIDString failed: %v", err)
	}
	if len(compact) != EncodedLength {
		t.Fatalf("length = %d, want %d", len(compact), EncodedLength)
	}
	if !IsValid(compact) {
		t.Fatalf("expected valid compact ID: %s", compact)
	}

	norm, ok := Normalize(orig)
	if !ok || norm != compact {
		t.Fatalf("Normalize(%s) = %s, %v; want %s, true", orig, norm, ok, compact)
	}

	norm2, ok2 := Normalize(compact)
	if !ok2 || norm2 != compact {
		t.Fatalf("Normalize(%s) = %s, %v; want unchanged", compact, norm2, ok2)
	}

	if Validate(orig) != nil {
		t.Fatalf("Validate(orig) should be nil")
	}
	if Validate(compact) != nil {
		t.Fatalf("Validate(compact) should be nil")
	}
	if Validate("invalid-id-xyz") == nil {
		t.Fatalf("Validate(invalid) should error")
	}
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

func BenchmarkEncode(b *testing.B) {
	u := uuid.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Encode(u)
	}
}

func BenchmarkDecode(b *testing.B) {
	s := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decode(s)
	}
}
