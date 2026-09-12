/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package apk

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto"
	"crypto/rsa"
	_ "crypto/sha1"
	"crypto/sha256"
	_ "crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"strings"

	"renop/pkg/hex"

	"github.com/klauspost/compress/gzip"
)

var ErrSignature = errors.New("APK package requires a valid trusted signature")

func ParsePublicKey(text string) (*rsa.PublicKey, error) {
	if len(text) > 16<<10 {
		return nil, ErrSignature
	}
	block, rest := pem.Decode([]byte(text))
	if block == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, ErrSignature
	}
	var key *rsa.PublicKey
	if block.Type == "RSA PUBLIC KEY" {
		key, _ = x509.ParsePKCS1PublicKey(block.Bytes)
	} else if block.Type == "PUBLIC KEY" {
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err == nil {
			key, _ = parsed.(*rsa.PublicKey)
		}
	}
	if key == nil || key.N.BitLen() < 2048 || key.N.BitLen() > 8192 {
		return nil, ErrSignature
	}
	return key, nil
}

// VerifySignature checks the signature over the compressed control member and
// its datahash over the compressed payload. Both are required for APK v2.
func VerifySignature(reader io.ReaderAt, size int64, publicKey string) error {
	key, err := ParsePublicKey(publicKey)
	if err != nil || size <= 0 || size > MaxPackageBytes {
		return ErrSignature
	}
	section := io.NewSectionReader(reader, 0, size)
	buffered := bufio.NewReader(section)
	type signature struct {
		data []byte
		hash crypto.Hash
	}
	var signatures []signature
	for range 8 {
		position, _ := section.Seek(0, io.SeekCurrent)
		start := position - int64(buffered.Buffered())
		compressed, err := gzip.NewReader(buffered)
		if err != nil {
			return ErrSignature
		}
		compressed.Multistream(false)
		bounded := &io.LimitedReader{R: compressed, N: maxControlBytes + 1}
		archive := tar.NewReader(bounded)
		var metadata []byte
		for range 4096 {
			header, err := archive.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				compressed.Close()
				return ErrSignature
			}
			name := strings.TrimPrefix(header.Name, "./")
			if name == ".PKGINFO" {
				if metadata != nil || header.Size <= 0 || header.Size > maxPackageInfoBytes {
					compressed.Close()
					return ErrSignature
				}
				metadata, err = io.ReadAll(io.LimitReader(archive, maxPackageInfoBytes+1))
				if err != nil {
					compressed.Close()
					return ErrSignature
				}
			} else if strings.HasPrefix(name, ".SIGN.") {
				if len(signatures) >= 16 || header.Size <= 0 || header.Size > 1024 {
					compressed.Close()
					return ErrSignature
				}
				algorithm := crypto.Hash(0)
				switch {
				case strings.HasPrefix(name, ".SIGN.RSA."):
					algorithm = crypto.SHA1
				case strings.HasPrefix(name, ".SIGN.RSA256."):
					algorithm = crypto.SHA256
				case strings.HasPrefix(name, ".SIGN.RSA512."):
					algorithm = crypto.SHA512
				}
				if algorithm != 0 {
					data, err := io.ReadAll(io.LimitReader(archive, 1025))
					if err != nil {
						compressed.Close()
						return ErrSignature
					}
					signatures = append(signatures, signature{data, algorithm})
				}
			}
		}
		_, err = io.Copy(io.Discard, bounded)
		compressed.Close()
		if err != nil || bounded.N <= 0 {
			return ErrSignature
		}
		position, _ = section.Seek(0, io.SeekCurrent)
		end := position - int64(buffered.Buffered())
		if metadata == nil {
			continue
		}
		if end >= size || len(signatures) == 0 {
			return ErrSignature
		}
		verified := false
		for _, signature := range signatures {
			hash := signature.hash.New()
			if _, err := io.Copy(hash, io.NewSectionReader(reader, start, end-start)); err != nil {
				return err
			}
			if rsa.VerifyPKCS1v15(key, signature.hash, hash.Sum(nil), signature.data) == nil {
				verified = true
				break
			}
		}
		if !verified {
			return ErrSignature
		}
		datahash := ""
		for line := range strings.SplitSeq(string(metadata), "\n") {
			if value, ok := strings.CutPrefix(strings.TrimSpace(line), "datahash = "); ok {
				if datahash != "" {
					return ErrSignature
				}
				datahash = value
			}
		}
		hash := sha256.New()
		if _, err := io.Copy(hash, io.NewSectionReader(reader, end, size-end)); err != nil {
			return err
		}
		if !strings.EqualFold(datahash, hex.EncodeToString(hash.Sum(nil))) {
			return ErrSignature
		}
		return nil
	}
	return ErrSignature
}
