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
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
)

func compareDecode(t testing.TB, src []byte) {
	t.Helper()
	size := DecodedLen(len(src))
	got, want := bytes.Repeat([]byte{0xa5}, size+32), bytes.Repeat([]byte{0xa5}, size+32)
	n, err := Decode(got[16:16+size:16+size], src)
	wn, we := stdhex.Decode(want[16:16+size:16+size], src)
	if n != wn || err != we || !bytes.Equal(got, want) {
		t.Fatalf("Decode(%x): (%d, %v, %x), want (%d, %v, %x)", src, n, err, got, wn, we, want)
	}
	inplace := bytes.Clone(src)
	in, ie := Decode(inplace, inplace)
	if in != wn || ie != we || !bytes.Equal(inplace[:in], want[16:16+wn]) {
		t.Fatalf("in-place Decode(%x): (%d, %v)", src, in, ie)
	}
	gd, ge := DecodeString(string(src))
	wd, we := stdhex.DecodeString(string(src))
	if ge != we || !bytes.Equal(gd, wd) {
		t.Fatalf("DecodeString(%x): (%x, %v), want (%x, %v)", src, gd, ge, wd, we)
	}
	gd, ge = AppendDecode(make([]byte, 7, 7+size), src)
	wd, we = stdhex.AppendDecode(make([]byte, 7, 7+size), src)
	if ge != we || !bytes.Equal(gd, wd) {
		t.Fatalf("AppendDecode(%x): (%x, %v), want (%x, %v)", src, gd, ge, wd, we)
	}
}

func TestCompatibility(t *testing.T) {
	rng := rand.New(rand.NewPCG(16, 256))
	sizes := []int{16383, 16384, 16385, 32767, 32768, 32769, 65537}
	for size := 0; size <= 2049; size++ {
		sizes = append(sizes, size)
	}
	for _, size := range sizes {
		backing := make([]byte, size+31)
		src := backing[size%32:][:size]
		for i := range src {
			src[i] = byte(rng.Uint32())
		}
		want := stdhex.EncodeToString(src)
		got := bytes.Repeat([]byte{0xa5}, len(want)+32)
		n := Encode(got[16:16+len(want):16+len(want)], src)
		if n != len(want) || string(got[16:16+n]) != want ||
			!bytes.Equal(got[:16], bytes.Repeat([]byte{0xa5}, 16)) ||
			!bytes.Equal(got[16+n:], bytes.Repeat([]byte{0xa5}, 16)) {
			t.Fatalf("Encode length %d", size)
		}
		if EncodeToString(src) != want || string(AppendEncode([]byte("prefix"), src)) != "prefix"+want {
			t.Fatalf("string/append encode length %d", size)
		}
		compareDecode(t, []byte(want))
		compareDecode(t, []byte(strings.ToUpper(want)))
	}
}

func TestEveryByteAtVectorBoundaries(t *testing.T) {
	// Exercise every possible byte in every lane and across multiple vectors.
	for _, size := range []int{96, 97} {
		src := bytes.Repeat([]byte("a0F9"), 25)[:size]
		for i := range src {
			original := src[i]
			for b := range 256 {
				src[i] = byte(b)
				compareDecode(t, src)
			}
			src[i] = original
		}
	}
}

func didPanic(f func()) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	f()
	return
}

func TestShortDestinationCompatibility(t *testing.T) {
	for _, src := range [][]byte{bytes.Repeat([]byte("a0F9"), 24), []byte(strings.Repeat("a0", 40) + "g0"), []byte("g")} {
		for size := 0; size < len(src)*2; size++ {
			got, want := bytes.Repeat([]byte{0xa5}, size+16), bytes.Repeat([]byte{0xa5}, size+16)
			gp := didPanic(func() { Encode(got[:size:size], src) })
			wp := didPanic(func() { stdhex.Encode(want[:size:size], src) })
			if gp != wp || !bytes.Equal(got, want) {
				t.Fatalf("short Encode size %d", size)
			}
			gn, wn := -1, -1
			var ge, we error
			gp = didPanic(func() { gn, ge = Decode(got[:size:size], src) })
			wp = didPanic(func() { wn, we = stdhex.Decode(want[:size:size], src) })
			if gp != wp || gn != wn || ge != we || !bytes.Equal(got, want) {
				t.Fatalf("short Decode size %d: (%v, %d, %v), want (%v, %d, %v)", size, gp, gn, ge, wp, wn, we)
			}
		}
	}
}

type fragmentedReader struct {
	data  []byte
	chunk int
	final error
	reads int
}

func (r *fragmentedReader) Read(p []byte) (int, error) {
	r.reads++
	if r.reads%3 == 0 {
		return 0, nil
	}
	n := copy(p, r.data[:min(len(r.data), r.chunk)])
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.final
	}
	return n, nil
}

func TestFragmentedDecoderCompatibility(t *testing.T) {
	failure := errors.New("reader failure")
	for _, suffix := range []string{"", "a", "g", "ag", "ga"} {
		data := []byte(strings.Repeat("01Ab9F", 350) + suffix)
		for _, chunk := range []int{1, 7, 31, 1023, 1024} {
			for _, end := range []error{io.EOF, failure} {
				got := NewDecoder(&fragmentedReader{data: data, chunk: chunk, final: end})
				want := stdhex.NewDecoder(&fragmentedReader{data: data, chunk: chunk, final: end})
				for step := 0; ; step++ {
					if step > len(data)*4+10 {
						t.Fatal("decoder made no progress")
					}
					size := []int{0, 1, 8, 15, 16, 31, 512, 1025}[step%8]
					gb, wb := make([]byte, size), make([]byte, size)
					gn, ge := got.Read(gb)
					wn, we := want.Read(wb)
					if gn != wn || ge != we || !bytes.Equal(gb, wb) {
						t.Fatalf("stream suffix %q chunk %d step %d: (%d, %v), want (%d, %v)", suffix, chunk, step, gn, ge, wn, we)
					}
					if ge != nil {
						break
					}
				}
			}
		}
	}
}

type failingWriter struct {
	bytes.Buffer
	limit int
	err   error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	n, _ := w.Buffer.Write(p[:min(len(p), w.limit)])
	w.limit -= n
	if n < len(p) {
		return n, w.err
	}
	return n, nil
}

func TestEncoderFailureCompatibility(t *testing.T) {
	failure := errors.New("writer failure")
	data := bytes.Repeat([]byte{0, 255, 123, 45}, 1025)
	for _, limit := range []int{0, 1, 31, 32, 1023, 1024, 2049, len(data) * 2} {
		gw, ww := &failingWriter{limit: limit, err: failure}, &failingWriter{limit: limit, err: failure}
		got, want := NewEncoder(gw), stdhex.NewEncoder(ww)
		for i := range 3 {
			gn, ge := got.Write(data)
			wn, we := want.Write(data)
			if gn != wn || ge != we || !bytes.Equal(gw.Bytes(), ww.Bytes()) {
				t.Fatalf("writer limit %d call %d: (%d, %v), want (%d, %v)", limit, i, gn, ge, wn, we)
			}
		}
	}
}

func FuzzCompatibility(f *testing.F) {
	for _, seed := range []string{"", "a", "g", "01ABCDef", strings.Repeat("a0F9", 33), strings.Repeat("f", 63) + "!"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		compareDecode(t, data)
		if EncodeToString(data) != stdhex.EncodeToString(data) {
			t.Fatal("encoding differs from standard library")
		}
	})
}

func BenchmarkStandardComparison(b *testing.B) {
	for _, size := range []int{16, 32, 64, 1024, 65536} {
		src := bytes.Repeat([]byte{0xab}, size)
		encoded, decoded := make([]byte, size*2), make([]byte, size)
		stdhex.Encode(encoded, src)
		for _, impl := range []struct {
			name   string
			encode func([]byte, []byte) int
			decode func([]byte, []byte) (int, error)
		}{{"renop", Encode, Decode}, {"stdlib", stdhex.Encode, stdhex.Decode}} {
			b.Run(fmt.Sprintf("Encode/%s/%d", impl.name, size), func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				for b.Loop() {
					impl.encode(encoded, src)
				}
			})
			b.Run(fmt.Sprintf("Decode/%s/%d", impl.name, size), func(b *testing.B) {
				b.SetBytes(int64(size))
				b.ReportAllocs()
				for b.Loop() {
					impl.decode(decoded, encoded)
				}
			})
		}
	}
}
