/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package nativepkg owns managed identities and upload validation for native
// repository protocols. Storage and browser APIs share these rules.
package nativepkg

import (
	"archive/tar"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"

	"renop/internal/config"
	"renop/internal/core"
	"renop/internal/service/apk"
	"renop/internal/service/apt"
	"renop/internal/service/conan"
	"renop/internal/service/conda"
	"renop/internal/service/rpm"
	"renop/internal/utils"
)

const MaxArtifactBytes int64 = 8 << 30

// ValidUploadPath excludes publisher-supplied repository indexes and arbitrary
// files. Indexes are derived from admitted, published artifacts by the server.
func ValidUploadPath(format, relative string) bool {
	clean, valid := utils.SanitizePath(relative)
	if !valid || clean != relative || len(relative) > 1024 || relative == "" || strings.HasPrefix(relative, "~/") {
		return false
	}
	switch format {
	case config.RepositoryFormatConda:
		parts := strings.Split(relative, "/")
		return len(parts) == 2 && (strings.HasSuffix(relative, ".conda") || strings.HasSuffix(relative, ".tar.bz2"))
	case config.RepositoryFormatAPK:
		return strings.HasSuffix(relative, ".apk")
	case config.RepositoryFormatAPT:
		return strings.HasSuffix(relative, ".deb") && !strings.HasPrefix(relative, "dists/")
	case config.RepositoryFormatRPM:
		return strings.HasSuffix(relative, ".rpm") && !strings.HasPrefix(relative, "repodata/")
	case config.RepositoryFormatConan:
		p, ok := conan.Parse(relative)
		if !ok || p.Operation != "file" {
			return false
		}
		switch p.File {
		case "conanfile.py", "conandata.yml", "conan_export.tgz", "conan_sources.tgz":
			return p.Package == ""
		case "conaninfo.txt", "conan_package.tgz":
			return p.Package != ""
		case "conanmanifest.txt":
			return true
		case "metadata/sign/pkgsign-manifest.json", "metadata/sign/pkgsign-signatures.json", "metadata/sign/pkgsign-manifest.json.asc":
			return true
		default:
			return false
		}
	}
	return false
}

func ConanIdentity(relative string) (name, version string, ok bool) {
	p, valid := conan.Parse(relative)
	if !valid {
		return "", "", false
	}
	parts := strings.Split(relative, "/")
	name = parts[2]
	if parts[4] != "_" || parts[5] != "_" {
		name += "@" + parts[4] + "/" + parts[5]
	}
	version = parts[3] + "#" + p.RecipeRevision
	if p.Package != "" {
		version += ":" + p.Package + "#" + p.PackageRevision
	}
	return name, version, core.ValidNativeResourceName(name) && len(version) <= 255
}

func Inspect(format, relative string, reader io.ReaderAt, size int64) (*core.NativeArtifact, []byte, error) {
	if size < 0 || size == 0 && !(format == config.RepositoryFormatConan && path.Base(relative) == "conaninfo.txt") || size > MaxArtifactBytes || !ValidUploadPath(format, relative) {
		return nil, nil, core.ErrNativeInvalid
	}
	artifact := &core.NativeArtifact{Path: relative, Size: size}
	var record any
	var err error
	switch format {
	case config.RepositoryFormatConda:
		var pkg *conda.Package
		pkg, err = conda.ReadPackage(reader, size, path.Base(relative))
		if err == nil {
			parts := strings.Split(relative, "/")
			_, err = conda.Repodata(parts[0], []*conda.Package{pkg})
			var version, build string
			if json.Unmarshal(pkg.Record["name"], &artifact.Name) != nil || json.Unmarshal(pkg.Record["version"], &version) != nil || json.Unmarshal(pkg.Record["build"], &build) != nil {
				return nil, nil, core.ErrNativeInvalid
			}
			artifact.Version = version + "/" + parts[0] + "/" + build
			expected := artifact.Name + "-" + version + "-" + build
			if path.Base(relative) != expected+".conda" && path.Base(relative) != expected+".tar.bz2" {
				return nil, nil, core.ErrNativeInvalid
			}
			record = pkg
		}
	case config.RepositoryFormatAPK:
		var pkg *apk.Package
		pkg, err = apk.ReadPackage(reader, size, path.Base(relative))
		if err == nil {
			artifact.Name = pkg.Fields["P"]
			artifact.Version = pkg.Fields["V"] + "/" + pkg.Fields["A"]
			if path.Base(relative) != artifact.Name+"-"+pkg.Fields["V"]+".apk" {
				return nil, nil, core.ErrNativeInvalid
			}
			record = pkg
		}
	case config.RepositoryFormatAPT:
		var pkg *apt.Package
		pkg, err = apt.ReadPackage(reader, size, relative)
		if err == nil {
			artifact.Name = pkg.Fields["Package"]
			artifact.Version = pkg.Fields["Version"] + "/" + pkg.Fields["Architecture"]
			version := pkg.Fields["Version"]
			if _, withoutEpoch, ok := strings.Cut(version, ":"); ok {
				version = withoutEpoch
			}
			if path.Base(relative) != artifact.Name+"_"+version+"_"+pkg.Fields["Architecture"]+".deb" {
				return nil, nil, core.ErrNativeInvalid
			}
			record = pkg
		}
	case config.RepositoryFormatRPM:
		var pkg *rpm.Package
		pkg, err = rpm.ReadPackage(reader, size, relative)
		if err == nil {
			artifact.Name = pkg.Name
			artifact.Version = strconv.FormatUint(pkg.Epoch, 10) + ":" + pkg.Version + "-" + pkg.Release + "/" + pkg.Architecture
			if path.Base(relative) != pkg.Name+"-"+pkg.Version+"-"+pkg.Release+"."+pkg.Architecture+".rpm" {
				return nil, nil, core.ErrNativeInvalid
			}
			record = pkg
		}
	case config.RepositoryFormatConan:
		var valid bool
		artifact.Name, artifact.Version, valid = ConanIdentity(relative)
		if !valid {
			return nil, nil, core.ErrNativeInvalid
		}
		err = validateConanFile(relative, reader, size)
	}
	if err != nil || !core.ValidNativeResourceName(artifact.Name) || len(artifact.Version) == 0 || len(artifact.Version) > 255 {
		return nil, nil, core.ErrNativeInvalid
	}
	if record == nil {
		return artifact, nil, nil
	}
	data, err := json.Marshal(record)
	if err != nil || len(data) > 1<<20 {
		return nil, nil, core.ErrNativeInvalid
	}
	return artifact, data, nil
}

func validateConanFile(relative string, reader io.ReaderAt, size int64) error {
	if !strings.HasSuffix(relative, ".tgz") {
		if size > 2<<20 {
			return core.ErrNativeInvalid
		}
		data, err := io.ReadAll(io.NewSectionReader(reader, 0, size))
		if err != nil {
			return err
		}
		if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
			return core.ErrNativeInvalid
		}
		return nil
	}
	compressed, err := gzip.NewReader(io.NewSectionReader(reader, 0, size))
	if err != nil {
		return core.ErrNativeInvalid
	}
	defer compressed.Close()
	bounded := &io.LimitedReader{R: compressed, N: MaxArtifactBytes + 1}
	archive := tar.NewReader(bounded)
	for range 100000 {
		entry, err := archive.Next()
		if errors.Is(err, io.EOF) {
			_, err = io.Copy(io.Discard, bounded)
			if bounded.N <= 0 {
				return core.ErrNativeInvalid
			}
			return err
		}
		if err != nil || bounded.N <= 0 {
			return core.ErrNativeInvalid
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if name == "." && entry.Typeflag == tar.TypeDir {
			continue
		}
		if clean, ok := utils.SanitizePath(name); !ok || clean != name || entry.Size < 0 || entry.Size > MaxArtifactBytes {
			return core.ErrNativeInvalid
		}
		if entry.Typeflag == tar.TypeSymlink || entry.Typeflag == tar.TypeLink {
			target := path.Clean(path.Join(path.Dir(name), entry.Linkname))
			if strings.HasPrefix(entry.Linkname, "/") || strings.Contains(entry.Linkname, `\`) || target == ".." || strings.HasPrefix(target, "../") {
				return core.ErrNativeInvalid
			}
		}
	}
	return core.ErrNativeInvalid
}
