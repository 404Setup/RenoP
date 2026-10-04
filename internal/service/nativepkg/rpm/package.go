/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package rpm

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
	"path"
	"strconv"
	"strings"

	"renop/pkg/hex"
)

type Dependency struct {
	Name    string `xml:"name,attr"`
	Flags   string `xml:"flags,attr,omitempty"`
	Epoch   string `xml:"epoch,attr,omitempty"`
	Version string `xml:"ver,attr,omitempty"`
	Release string `xml:"rel,attr,omitempty"`
	Pre     string `xml:"pre,attr,omitempty"`
}

type File struct {
	Path string `xml:",chardata"`
	Type string `xml:"type,attr,omitempty"`
}

type Package struct {
	Filename, Name, Version, Release, Architecture, Summary, Description, License, URL, Group, Packager, Vendor, SourceRPM, SHA256 string
	Epoch, BuildTime, InstalledSize                                                                                                uint64
	Size, HeaderStart, HeaderEnd                                                                                                   int64
	Provides, Requires, Conflicts, Obsoletes                                                                                       []Dependency
	Files                                                                                                                          []File
}

func ReadPackage(reader io.ReaderAt, size int64, filename string) (*Package, error) {
	if size <= 112 || size > 8<<30 || !strings.HasSuffix(filename, ".rpm") || strings.ContainsAny(filename, "\x00\r\n") {
		return nil, ErrInvalidPackage
	}
	var lead [96]byte
	if _, err := reader.ReadAt(lead[:], 0); err != nil {
		return nil, err
	}
	if binary.BigEndian.Uint32(lead[:4]) != 0xedabeedb || (lead[4] != 3 && lead[4] != 4) {
		return nil, ErrInvalidPackage
	}
	signature, err := readHeader(reader, size, 96)
	if err != nil {
		return nil, err
	}
	start := (signature.end + 7) &^ 7
	metadata, err := readHeader(reader, size, start)
	if err != nil {
		return nil, err
	}
	if metadata.end >= size {
		return nil, ErrInvalidPackage
	}
	pkg := &Package{Filename: filename, Size: size, HeaderStart: start, HeaderEnd: metadata.end}
	for _, field := range []struct {
		tag    uint32
		target *string
	}{
		{1000, &pkg.Name}, {1001, &pkg.Version}, {1002, &pkg.Release}, {1022, &pkg.Architecture}, {1004, &pkg.Summary}, {1005, &pkg.Description},
		{1014, &pkg.License}, {1020, &pkg.URL}, {1016, &pkg.Group}, {1015, &pkg.Packager}, {1011, &pkg.Vendor}, {1044, &pkg.SourceRPM},
	} {
		values, err := metadata.strings(field.tag)
		if err != nil {
			return nil, err
		}
		if len(values) > 0 {
			*field.target = values[0]
		}
	}
	for _, field := range []struct {
		tag    uint32
		target *uint64
	}{{1003, &pkg.Epoch}, {1006, &pkg.BuildTime}, {1009, &pkg.InstalledSize}, {5009, &pkg.InstalledSize}} {
		values, err := metadata.numbers(field.tag)
		if err != nil {
			return nil, err
		}
		if len(values) > 0 {
			*field.target = values[0]
		}
	}
	if binary.BigEndian.Uint16(lead[6:8]) == 1 {
		pkg.Architecture = "src"
	}
	for _, value := range []string{pkg.Name, pkg.Version, pkg.Release, pkg.Architecture} {
		if value == "" || len(value) > 1024 || strings.ContainsAny(value, " \t\r\n/\\") {
			return nil, ErrInvalidPackage
		}
	}
	for _, field := range []struct {
		name, flags, version uint32
		target               *[]Dependency
	}{
		{1047, 1112, 1113, &pkg.Provides}, {1049, 1048, 1050, &pkg.Requires}, {1054, 1053, 1055, &pkg.Conflicts}, {1090, 1114, 1115, &pkg.Obsoletes},
	} {
		*field.target, err = readDependencies(metadata, field.name, field.flags, field.version)
		if err != nil {
			return nil, err
		}
	}
	pkg.Files, err = readFiles(metadata)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.NewSectionReader(reader, 0, size))
	if err != nil || n != size {
		return nil, ErrInvalidPackage
	}
	pkg.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return pkg, nil
}

func readDependencies(metadata *header, nameTag, flagsTag, versionTag uint32) ([]Dependency, error) {
	names, err := metadata.strings(nameTag)
	if err != nil {
		return nil, err
	}
	versions, err := metadata.strings(versionTag)
	if err != nil {
		return nil, err
	}
	flags, err := metadata.numbers(flagsTag)
	if err != nil {
		return nil, err
	}
	if len(names) > 16384 || (len(versions) != 0 && len(versions) != len(names)) || (len(flags) != 0 && len(flags) != len(names)) {
		return nil, ErrInvalidPackage
	}
	dependencies := make([]Dependency, 0, len(names))
	for i, name := range names {
		if name == "" || strings.ContainsAny(name, "\r\n\x00") {
			return nil, ErrInvalidPackage
		}
		entry := Dependency{Name: name}
		if len(flags) > 0 {
			switch flags[i] & 14 {
			case 2:
				entry.Flags = "LT"
			case 4:
				entry.Flags = "GT"
			case 8:
				entry.Flags = "EQ"
			case 10:
				entry.Flags = "LE"
			case 12:
				entry.Flags = "GE"
			}
			if flags[i]&(1<<6|1<<8|1<<9|1<<10|1<<11|1<<12) != 0 {
				entry.Pre = "1"
			}
		}
		if len(versions) > 0 && versions[i] != "" {
			version := versions[i]
			entry.Epoch = "0"
			if before, after, ok := strings.Cut(version, ":"); ok {
				if _, err := strconv.ParseUint(before, 10, 64); err != nil {
					return nil, ErrInvalidPackage
				}
				entry.Epoch, version = before, after
			}
			if at := strings.LastIndexByte(version, '-'); at >= 0 {
				entry.Release, version = version[at+1:], version[:at]
			}
			entry.Version = version
		}
		dependencies = append(dependencies, entry)
	}
	return dependencies, nil
}

func readFiles(metadata *header) ([]File, error) {
	base, err := metadata.strings(1117)
	if err != nil {
		return nil, err
	}
	dirs, err := metadata.strings(1118)
	if err != nil {
		return nil, err
	}
	indices, err := metadata.numbers(1116)
	if err != nil {
		return nil, err
	}
	modes, err := metadata.numbers(1030)
	if err != nil {
		return nil, err
	}
	flags, err := metadata.numbers(1037)
	if err != nil {
		return nil, err
	}
	if len(base) != len(indices) || (len(modes) != 0 && len(modes) != len(base)) || (len(flags) != 0 && len(flags) != len(base)) {
		return nil, ErrInvalidPackage
	}
	files := make([]File, 0, len(base))
	for i, name := range base {
		if indices[i] >= uint64(len(dirs)) || path.Base(name) != name || strings.ContainsAny(name, "\r\n\x00") {
			return nil, ErrInvalidPackage
		}
		file := File{Path: path.Join(dirs[indices[i]], name)}
		if len(modes) > 0 && modes[i]&0170000 == 0040000 {
			file.Type = "dir"
		}
		if len(flags) > 0 && flags[i]&(1<<6) != 0 {
			file.Type = "ghost"
		}
		files = append(files, file)
	}
	return files, nil
}
