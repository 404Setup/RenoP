/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package nativepkg

import (
	"bytes"
	"strings"

	"renop/internal/core"
	"renop/internal/service/conan"
	"renop/pkg/hex"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/goccy/go-json"
)

// VerifyConanPublication implements Conan's signing-extension manifest contract.
// Missing files are an incomplete upload, not permission to publish unsigned.
func VerifyConanPublication(publicKey string, artifacts []*core.NativeArtifact,
	read func(string, int64) ([]byte, error), digest func(string) (string, error),
) (bool, error) {
	files := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		parsed, ok := conan.Parse(artifact.Path)
		if !ok || parsed.Operation != "file" {
			return false, core.ErrNativeInvalid
		}
		if _, ok := files[parsed.File]; ok {
			return false, core.ErrNativeInvalid
		}
		files[parsed.File] = artifact.Path
	}
	for _, name := range []string{"metadata/sign/pkgsign-manifest.json", "metadata/sign/pkgsign-signatures.json", "metadata/sign/pkgsign-manifest.json.asc"} {
		if files[name] == "" {
			return false, nil
		}
	}
	manifest, err := read(files["metadata/sign/pkgsign-manifest.json"], 1<<20)
	if err != nil {
		return false, err
	}
	metadata, err := read(files["metadata/sign/pkgsign-signatures.json"], 64<<10)
	if err != nil {
		return false, err
	}
	var signatures struct {
		Signatures []struct {
			Method    string            `json:"method"`
			Artifacts map[string]string `json:"sign_artifacts"`
		} `json:"signatures"`
	}
	if json.Unmarshal(metadata, &signatures) != nil || len(signatures.Signatures) == 0 || len(signatures.Signatures) > 8 {
		return false, core.ErrNativeSignature
	}
	matched := false
	for _, signature := range signatures.Signatures {
		if signature.Method == "gpg" && signature.Artifacts["manifest"] == "pkgsign-manifest.json" && signature.Artifacts["signature"] == "pkgsign-manifest.json.asc" {
			matched = true
			break
		}
	}
	if !matched {
		return false, core.ErrNativeSignature
	}
	signature, err := read(files["metadata/sign/pkgsign-manifest.json.asc"], 64<<10)
	if err != nil {
		return false, err
	}
	ring, err := openpgp.ReadArmoredKeyRing(strings.NewReader(publicKey))
	if err != nil {
		return false, core.ErrNativeSignature
	}
	if _, err := openpgp.CheckArmoredDetachedSignature(ring, bytes.NewReader(manifest), bytes.NewReader(signature), nil); err != nil {
		return false, core.ErrNativeSignature
	}
	var document struct {
		Files []struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if json.Unmarshal(manifest, &document) != nil || len(document.Files) == 0 || len(document.Files) > 253 {
		return false, core.ErrNativeSignature
	}
	seen := make(map[string]bool, len(document.Files))
	for _, file := range document.Files {
		if file.File == "" || strings.HasPrefix(file.File, "metadata/") || seen[file.File] || len(file.SHA256) != 64 {
			return false, core.ErrNativeSignature
		}
		if _, err := hex.DecodeString(file.SHA256); err != nil {
			return false, core.ErrNativeSignature
		}
		seen[file.File] = true
		if files[file.File] == "" {
			return false, nil
		}
		actual, err := digest(files[file.File])
		if err != nil {
			return false, err
		}
		if !strings.EqualFold(actual, file.SHA256) {
			return false, core.ErrNativeSignature
		}
	}
	for file := range files {
		if !strings.HasPrefix(file, "metadata/sign/") && !seen[file] {
			return false, core.ErrNativeSignature
		}
	}
	return true, nil
}
