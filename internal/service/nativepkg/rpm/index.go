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
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"strconv"

	"github.com/klauspost/compress/gzip"

	"renop/pkg/hex"
)

type versionXML struct {
	Epoch   string `xml:"epoch,attr"`
	Version string `xml:"ver,attr"`
	Release string `xml:"rel,attr"`
}
type checksumXML struct {
	Type  string `xml:"type,attr"`
	PkgID string `xml:"pkgid,attr,omitempty"`
	Value string `xml:",chardata"`
}
type locationXML struct {
	Href string `xml:"href,attr"`
}
type timeXML struct {
	File  uint64 `xml:"file,attr"`
	Build uint64 `xml:"build,attr"`
}
type sizeXML struct {
	Package   int64  `xml:"package,attr"`
	Installed uint64 `xml:"installed,attr"`
	Archive   uint64 `xml:"archive,attr"`
}
type headerRangeXML struct {
	Start int64 `xml:"start,attr"`
	End   int64 `xml:"end,attr"`
}
type formatXML struct {
	License   string         `xml:"rpm:license"`
	Vendor    string         `xml:"rpm:vendor"`
	Group     string         `xml:"rpm:group"`
	SourceRPM string         `xml:"rpm:sourcerpm"`
	Header    headerRangeXML `xml:"rpm:header-range"`
	Provides  []Dependency   `xml:"rpm:provides>rpm:entry"`
	Requires  []Dependency   `xml:"rpm:requires>rpm:entry"`
	Conflicts []Dependency   `xml:"rpm:conflicts>rpm:entry"`
	Obsoletes []Dependency   `xml:"rpm:obsoletes>rpm:entry"`
	Files     []File         `xml:"file"`
}
type primaryPackageXML struct {
	Type        string      `xml:"type,attr"`
	Name        string      `xml:"name"`
	Arch        string      `xml:"arch"`
	Version     versionXML  `xml:"version"`
	Checksum    checksumXML `xml:"checksum"`
	Summary     string      `xml:"summary"`
	Description string      `xml:"description"`
	Packager    string      `xml:"packager"`
	URL         string      `xml:"url"`
	Time        timeXML     `xml:"time"`
	Size        sizeXML     `xml:"size"`
	Location    locationXML `xml:"location"`
	Format      formatXML   `xml:"format"`
}
type filePackageXML struct {
	PkgID   string     `xml:"pkgid,attr"`
	Name    string     `xml:"name,attr"`
	Arch    string     `xml:"arch,attr"`
	Version versionXML `xml:"version"`
	Files   []File     `xml:"file"`
}
type primaryXML struct {
	XMLName  xml.Name            `xml:"metadata"`
	XMLNS    string              `xml:"xmlns,attr"`
	RPMNS    string              `xml:"xmlns:rpm,attr"`
	Count    int                 `xml:"packages,attr"`
	Packages []primaryPackageXML `xml:"package"`
}
type filelistsXML struct {
	XMLName  xml.Name         `xml:"filelists"`
	XMLNS    string           `xml:"xmlns,attr"`
	Count    int              `xml:"packages,attr"`
	Packages []filePackageXML `xml:"package"`
}
type otherXML struct {
	XMLName  xml.Name         `xml:"otherdata"`
	XMLNS    string           `xml:"xmlns,attr"`
	Count    int              `xml:"packages,attr"`
	Packages []filePackageXML `xml:"package"`
}
type repomdDataXML struct {
	Type         string      `xml:"type,attr"`
	Checksum     checksumXML `xml:"checksum"`
	OpenChecksum checksumXML `xml:"open-checksum"`
	Location     locationXML `xml:"location"`
	Timestamp    uint64      `xml:"timestamp"`
	Size         int         `xml:"size"`
	OpenSize     int         `xml:"open-size"`
}
type repomdXML struct {
	XMLName  xml.Name        `xml:"repomd"`
	XMLNS    string          `xml:"xmlns,attr"`
	Revision string          `xml:"revision"`
	Data     []repomdDataXML `xml:"data"`
}

// Index produces rpm-md documents with hashes over the exact served bytes.
// Content-hashed filenames prevent clients from mistaking a newer metadata body
// for the version advertised by an older repomd.xml response.
func Index(packages []*Package) (map[string][]byte, error) {
	if len(packages) > 10000 {
		return nil, ErrInvalidPackage
	}
	primary := primaryXML{XMLNS: "http://linux.duke.edu/metadata/common", RPMNS: "http://linux.duke.edu/metadata/rpm", Count: len(packages)}
	files := filelistsXML{XMLNS: "http://linux.duke.edu/metadata/filelists", Count: len(packages)}
	other := otherXML{XMLNS: "http://linux.duke.edu/metadata/other", Count: len(packages)}
	var timestamp uint64
	for _, pkg := range packages {
		if pkg == nil {
			return nil, ErrInvalidPackage
		}
		version := versionXML{strconv.FormatUint(pkg.Epoch, 10), pkg.Version, pkg.Release}
		primary.Packages = append(primary.Packages, primaryPackageXML{
			Type: "rpm", Name: pkg.Name, Arch: pkg.Architecture, Version: version, Checksum: checksumXML{"sha256", "YES", pkg.SHA256},
			Summary: pkg.Summary, Description: pkg.Description, Packager: pkg.Packager, URL: pkg.URL, Time: timeXML{pkg.BuildTime, pkg.BuildTime},
			Size: sizeXML{pkg.Size, pkg.InstalledSize, pkg.InstalledSize}, Location: locationXML{pkg.Filename},
			Format: formatXML{pkg.License, pkg.Vendor, pkg.Group, pkg.SourceRPM, headerRangeXML{pkg.HeaderStart, pkg.HeaderEnd}, pkg.Provides, pkg.Requires, pkg.Conflicts, pkg.Obsoletes, pkg.Files},
		})
		filePackage := filePackageXML{pkg.SHA256, pkg.Name, pkg.Architecture, version, pkg.Files}
		files.Packages = append(files.Packages, filePackage)
		filePackage.Files = nil
		other.Packages = append(other.Packages, filePackage)
		if pkg.BuildTime > timestamp {
			timestamp = pkg.BuildTime
		}
	}
	result := make(map[string][]byte)
	repomd := repomdXML{XMLNS: "http://linux.duke.edu/metadata/repo"}
	total := 0
	for _, document := range []struct {
		name  string
		value any
	}{{"primary", primary}, {"filelists", files}, {"other", other}} {
		plain, err := xml.Marshal(document.value)
		if err != nil {
			return nil, err
		}
		plain = append([]byte(xml.Header), plain...)
		total += len(plain)
		if total > 64<<20 {
			return nil, ErrInvalidPackage
		}
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err := writer.Write(plain); err != nil {
			return nil, err
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		body := compressed.Bytes()
		hash, openHash := sha256.Sum256(body), sha256.Sum256(plain)
		digest := hex.EncodeToString(hash[:])
		filename := "repodata/" + digest + "-" + document.name + ".xml.gz"
		result[filename] = body
		repomd.Data = append(repomd.Data, repomdDataXML{document.name, checksumXML{Type: "sha256", Value: digest}, checksumXML{Type: "sha256", Value: hex.EncodeToString(openHash[:])}, locationXML{filename}, timestamp, len(body), len(plain)})
	}
	// Revision changes with content even when all RPMs have the same build time.
	repomd.Revision = repomd.Data[0].Checksum.Value
	body, err := xml.Marshal(repomd)
	if err != nil {
		return nil, err
	}
	result["repodata/repomd.xml"] = append([]byte(xml.Header), body...)
	return result, nil
}
