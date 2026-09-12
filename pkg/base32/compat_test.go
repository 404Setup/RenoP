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
	"fmt"
	"io"
	"math/rand/v2"
	"testing"
)

var testAlphabets = []string{
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ234567",
	"0123456789ABCDEFGHIJKLMNOPQRSTUV",
	"abcdefghijklmnopqrstuvwxyz234567",
	"\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f",
}

func checkDecode(t testing.TB, enc *Encoding, ref *stdbase32.Encoding, src []byte) {
	t.Helper()
	size := enc.DecodedLen(len(src))
	got, want := bytes.Repeat([]byte{0xa5}, size+32), bytes.Repeat([]byte{0xa5}, size+32)
	n, err := enc.Decode(got[16:16+size:16+size], src)
	wn, we := ref.Decode(want[16:16+size:16+size], src)
	wantErr := we
	if n != wn || err != we || !bytes.Equal(got, want) {
		t.Fatalf("Decode(%x): (%d, %v, %x), want (%d, %v, %x)", src, n, err, got, wn, we, want)
	}
	gd, ge := enc.DecodeString(string(src))
	wd, we := ref.DecodeString(string(src))
	if ge != we || !bytes.Equal(gd, wd) {
		t.Fatalf("DecodeString(%x): (%x, %v), want (%x, %v)", src, gd, ge, wd, we)
	}
	gd, ge = enc.AppendDecode([]byte("prefix"), src)
	wd, we = ref.AppendDecode([]byte("prefix"), src)
	if ge != we || !bytes.Equal(gd, wd) {
		t.Fatalf("AppendDecode(%x): (%x, %v), want (%x, %v)", src, gd, ge, wd, we)
	}
	// The standard DecodeString also relies on decoding in place.
	inplace := bytes.Clone(src)
	n, err = enc.Decode(inplace, inplace)
	if n != wn || err != wantErr || !bytes.Equal(inplace[:n], want[16:16+wn]) {
		t.Fatalf("in-place Decode(%x): (%x, %v)", src, inplace[:n], err)
	}
}

func TestStandardLibraryCompatibility(t *testing.T) {
	rng := rand.New(rand.NewPCG(32, 4648))
	for _, alphabet := range testAlphabets {
		for _, padding := range []rune{StdPadding, NoPadding, '@', 0xff} {
			enc, ref := NewEncoding(alphabet).WithPadding(padding), stdbase32.NewEncoding(alphabet).WithPadding(padding)
			for _, size := range []int{0, 1, 2, 3, 4, 5, 9, 10, 15, 16, 19, 20, 21, 25, 31, 32, 33, 40, 63, 64, 65, 127, 128, 129, 1023, 1024, 16383, 16384, 16385, 32769} {
				for offset := range 16 {
					src := make([]byte, size+offset)[offset:]
					for i := range src {
						src[i] = byte(rng.Uint32())
					}
					want := ref.EncodeToString(src)
					dst := bytes.Repeat([]byte{0xa5}, len(want)+offset+16)
					enc.Encode(dst[offset:offset+len(want):offset+len(want)], src)
					if string(dst[offset:offset+len(want)]) != want || !bytes.Equal(dst[:offset], bytes.Repeat([]byte{0xa5}, offset)) || !bytes.Equal(dst[offset+len(want):], bytes.Repeat([]byte{0xa5}, 16)) {
						t.Fatalf("Encode: alphabet %q, padding %d, size %d, offset %d", alphabet, padding, size, offset)
					}
					if enc.EncodeToString(src) != want || string(enc.AppendEncode([]byte("prefix"), src)) != "prefix"+want {
						t.Fatalf("string/append encoding: size %d", size)
					}
					checkDecode(t, enc, ref, []byte(want))
				}
			}
		}
	}
}

func TestInvalidBlockCompatibility(t *testing.T) {
	for _, alphabet := range testAlphabets[:2] {
		for _, padding := range []rune{StdPadding, NoPadding, '@', 0xff} {
			enc, ref := NewEncoding(alphabet).WithPadding(padding), stdbase32.NewEncoding(alphabet).WithPadding(padding)
			src := []byte(ref.EncodeToString(bytes.Repeat([]byte("base32"), 10)))
			for i := range src {
				original := src[i]
				for value := range 256 {
					src[i] = byte(value)
					checkDecode(t, enc, ref, src)
				}
				src[i] = original
			}
			for i := 0; i <= len(src); i++ {
				checkDecode(t, enc, ref, src[:i])
				withNewlines := append([]byte{}, src[:i]...)
				withNewlines = append(withNewlines, '\r', '\n')
				withNewlines = append(withNewlines, src[i:]...)
				checkDecode(t, enc, ref, withNewlines)
			}
		}
	}
}

func TestLargeStreamingCompatibility(t *testing.T) {
	data := bytes.Repeat([]byte("a large fragmented stream\x00\xff"), 800)
	for _, alphabet := range testAlphabets {
		for _, padding := range []rune{StdPadding, NoPadding, 0xff} {
			enc, ref := NewEncoding(alphabet).WithPadding(padding), stdbase32.NewEncoding(alphabet).WithPadding(padding)
			for _, chunk := range []int{1, 7, 16, 31, 1024} {
				var got bytes.Buffer
				w := NewEncoder(enc, &got)
				for start := 0; start < len(data); start += chunk {
					p := data[start:min(start+chunk, len(data))]
					if n, err := w.Write(p); err != nil || n != len(p) {
						t.Fatalf("Write: %d, %v", n, err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				if got.String() != ref.EncodeToString(data) {
					t.Fatal("stream encoding differs")
				}
				for _, suffix := range []string{"", "!", "A", "\r\n"} {
					encoded := append(bytes.Clone(got.Bytes()), suffix...)
					gd, ge := io.ReadAll(NewDecoder(enc, &chunkReader{data: encoded, chunk: chunk}))
					wd, we := io.ReadAll(stdbase32.NewDecoder(ref, &chunkReader{data: encoded, chunk: chunk}))
					if ge != we || !bytes.Equal(gd, wd) {
						t.Fatalf("stream decoding differs, chunk %d, padding %d: %v, want %v", chunk, padding, ge, we)
					}
				}
			}
		}
	}
}

type chunkReader struct {
	data  []byte
	chunk int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p[:min(len(p), r.chunk)], r.data)
	r.data = r.data[n:]
	return n, nil
}

func FuzzCompatibility(f *testing.F) {
	for _, seed := range [][]byte{nil, []byte("MZXW6YTBOI======"), bytes.Repeat([]byte("A"), 128), {0, 255, '\r', '\n'}} {
		f.Add(seed, byte(0))
	}
	f.Fuzz(func(t *testing.T, data []byte, mode byte) {
		alphabet := testAlphabets[int(mode)%len(testAlphabets)]
		padding := []rune{StdPadding, NoPadding, '@', 0xff}[int(mode/4)%4]
		enc, ref := NewEncoding(alphabet).WithPadding(padding), stdbase32.NewEncoding(alphabet).WithPadding(padding)
		checkDecode(t, enc, ref, data)
		encoded := enc.EncodeToString(data)
		if encoded != ref.EncodeToString(data) {
			t.Fatal("encoding differs")
		}
		checkDecode(t, enc, ref, []byte(encoded))
	})
}

func BenchmarkCompare(b *testing.B) {
	for _, size := range []int{20, 64, 1024, 64 * 1024} {
		data := bytes.Repeat([]byte{0x9b}, size)
		encoded := []byte(stdbase32.StdEncoding.EncodeToString(data))
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			for _, codec := range []struct {
				name   string
				encode func([]byte, []byte)
				decode func([]byte, []byte) (int, error)
			}{{"module", StdEncoding.Encode, StdEncoding.Decode}, {"stdlib", stdbase32.StdEncoding.Encode, stdbase32.StdEncoding.Decode}} {
				b.Run(codec.name+"/encode", func(b *testing.B) {
					dst := make([]byte, len(encoded))
					b.SetBytes(int64(size))
					b.ReportAllocs()
					for b.Loop() {
						codec.encode(dst, data)
					}
				})
				b.Run(codec.name+"/decode", func(b *testing.B) {
					dst := make([]byte, StdEncoding.DecodedLen(len(encoded)))
					b.SetBytes(int64(size))
					b.ReportAllocs()
					for b.Loop() {
						if _, err := codec.decode(dst, encoded); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
