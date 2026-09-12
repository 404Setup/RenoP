/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package demo

import (
	"fmt"
	"strings"
	"time"

	"github.com/emmansun/base64"
	"github.com/goccy/go-json"

	"renop/internal/core"
	"renop/internal/service/cargo"
)

func (s *seedContext) packages() error {
	for _, step := range []func() error{s.mavenPackages, s.cargoPackages, s.npmPackages, s.dockerPackages} {
		if err := step(); err != nil {
			return err
		}
	}
	for _, file := range []struct {
		repo, path string
		size       int64
	}{
		{"downloads", "releases/demo-tool-1.2.0.zip", 18 << 20},
		{"downloads", "releases/demo-tool-1.2.0.tar.gz", 14 << 20},
		{"downloads", "guides/getting-started.pdf", 384 << 10},
		{"downloads", "checksums/SHA256SUMS", 256},
		{"private", "internal/architecture-notes.pdf", 192 << 10},
	} {
		if err := s.file(file.repo, file.path, file.size); err != nil {
			return err
		}
	}
	return nil
}

func (s *seedContext) mavenPackages() error {
	at := s.now - 45*24*time.Hour.Milliseconds()
	domain := &core.MavenDomain{Domain: "com.example", VerificationType: core.MavenVerificationDNS,
		VerificationHost: "example.com", VerificationCode: "demo-domain-proof", SuperTeamPrefix: "platform",
		CreatedAt: at, Verified: true, VerifiedAt: at, LastCheckAt: s.now - 3600000,
		Health: &core.MavenDomainHealth{Status: "healthy", CheckedAt: s.now - 3600000, ExpiresAt: s.now + 365*24*time.Hour.Milliseconds()}}
	if err := s.db.CreateMavenDomain(domain, "admin"); err != nil {
		return err
	}
	if err := s.db.ForceAddMavenMembers(domain.Domain, "admin", []string{"alice", "bobby"}, core.MavenPermissionPublish); err != nil {
		return err
	}
	if err := s.db.CreateMavenDomain(&core.MavenDomain{Domain: "org.preview", VerificationType: core.MavenVerificationDNS,
		VerificationHost: "preview.org", VerificationCode: "demo-pending-proof", CreatedAt: s.now - 3600000}, "bobby"); err != nil {
		return err
	}
	for i, name := range []string{"demo-client", "demo-utils", "demo-plugin"} {
		artifact := &core.MavenArtifact{Repository: "releases", Domain: domain.Domain, GroupID: "com.example.demo", ArtifactID: name,
			Description: "Example Java library with stable releases and source attachments.", Publisher: "admin", CreatedAt: at, UpdatedAt: at}
		for j, version := range []string{"1.0.0", "1.1.0", "1.2.0"} {
			size := int64(128+i*96+j*24) << 10
			if err := s.db.RecordMavenPublication(artifact, &core.MavenVersion{Repository: "releases", GroupID: artifact.GroupID,
				ArtifactID: name, Version: version, Publisher: "admin", Size: size, CreatedAt: s.now - int64(9-j*3)*24*time.Hour.Milliseconds()}); err != nil {
				return err
			}
			base := "com/example/demo/" + name + "/" + version + "/" + name + "-" + version
			for _, entry := range []struct {
				suffix string
				bytes  int64
			}{{".jar", size}, {".pom", 2048}, {"-sources.jar", 48 << 10}, {".jar.sha256", 64}} {
				if err := s.file("releases", base+entry.suffix, entry.bytes); err != nil {
					return err
				}
			}
		}
		if err := s.db.UpdateMavenArtifactReadme("releases", artifact.GroupID, name, "# "+name+"\n\nA preset package for exploring RenoP.\n\n## Getting started\n\nAdd this library to your Maven or Gradle dependencies. Package files are not included in demonstration mode."); err != nil {
			return err
		}
	}
	if err := s.db.EnsureMirroredMavenDomain("org.example", at); err != nil {
		return err
	}
	if err := s.db.RecordMavenMirrorPublication(&core.MavenArtifact{Repository: "mirror", Domain: "org.example", GroupID: "org.example",
		ArtifactID: "upstream-library", Description: "A mirrored dependency example", Mirrored: true, CreatedAt: at, UpdatedAt: at},
		&core.MavenVersion{Repository: "mirror", GroupID: "org.example", ArtifactID: "upstream-library", Version: "3.0.0", Mirrored: true, Size: 512 << 10, CreatedAt: at}); err != nil {
		return err
	}
	if err := s.file("mirror", "org/example/upstream-library/3.0.0/upstream-library-3.0.0.jar", 512<<10); err != nil {
		return err
	}
	return s.file("snapshots", "com/example/demo/demo-client/1.3.0-SNAPSHOT/demo-client-1.3.0-20260914.120000-1.jar", 256<<10)
}

func (s *seedContext) cargoPackages() error {
	for i, name := range []string{"renop_demo", "fast_hash"} {
		normalized, valid := cargo.NormalizeCrateName(name)
		if !valid {
			return fmt.Errorf("invalid preset crate name %q", name)
		}
		pkg := &core.CargoPackage{Repository: "cargo", Name: name, NormalizedName: normalized, SuperTeamPrefix: "platform",
			Description: "A demonstration Rust crate with optional features.", Readme: "# " + name + "\n\nExplore versions, features, collaborators and download statistics.",
			Homepage: "https://example.com", RepositoryURL: "https://example.com/source", CreatedAt: s.now - 20*24*time.Hour.Milliseconds()}
		for j, version := range []string{"0.9.0", "1.0.0"} {
			at, size := s.now-int64(8-j*5)*24*time.Hour.Milliseconds(), int64(40+i*24+j*8)<<10
			if err := s.db.RecordCargoPublication(pkg, &core.CargoVersion{Repository: "cargo", Package: name, Version: version,
				Publisher: "admin", Checksum: digest(name + version), RustVersion: "1.85", License: "MIT OR Apache-2.0",
				Features: map[string][]string{"default": {"std"}, "std": {}, "serde": {}}, Size: size, CreatedAt: at}, "admin"); err != nil {
				return err
			}
			if err := s.file("cargo", "crates/"+name+"/"+name+"-"+version+".crate", size); err != nil {
				return err
			}
		}
		if err := s.db.ForceAddCargoMembers("cargo", normalized, name, "admin", []string{"alice"}, core.CargoPermissionManage); err != nil {
			return err
		}
		if err := s.db.SetCargoVersionYanked("cargo", normalized, "0.9.0", true, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *seedContext) npmPackages() error {
	for _, name := range []string{"@platform/ui", "demo-cli"} {
		team := ""
		if strings.HasPrefix(name, "@") {
			team = "platform"
		}
		pkg, err := s.db.CreateNPMPackageForTeam("npm", name, "admin", team, false, s.now-20*24*time.Hour.Milliseconds())
		if err != nil {
			return err
		}
		pkg.Description = "A preset JavaScript package for interface and CLI examples."
		for i, version := range []string{"1.0.0", "1.1.0"} {
			manifest, err := json.Marshal(map[string]any{"name": name, "version": version, "description": pkg.Description,
				"license": "MIT", "homepage": "https://example.com", "repository": map[string]string{"type": "git", "url": "https://example.com/source"},
				"readme": "# " + name + "\n\nA demonstration package.\n\n```sh\nnpm install " + name + "\n```", "engines": map[string]string{"node": ">=22"}})
			if err != nil {
				return err
			}
			base := name[strings.LastIndex(name, "/")+1:]
			path := name + "/-/" + base + "-" + version + ".tgz"
			size := int64(96+i*32) << 10
			item := &core.NPMVersion{Repository: "npm", Package: name, Version: version, Publisher: "admin", ManifestJSON: string(manifest),
				TarballPath: path, Shasum: digest(name + version)[:40], Integrity: "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64)),
				Size: size, CreatedAt: s.now - int64(7-i*5)*24*time.Hour.Milliseconds()}
			if i == 0 {
				item.Deprecated = "Prefer version 1.1.0 for new projects."
			}
			if err := s.db.RecordNPMPublication(pkg, item, map[string]string{"latest": version}, "admin"); err != nil {
				return err
			}
			if err := s.file("npm", path, size); err != nil {
				return err
			}
		}
		if err := s.db.ForceAddNPMMembers("npm", name, "admin", []string{"alice", "bobby"}, core.NPMPermissionPublish); err != nil {
			return err
		}
	}
	return nil
}

func (s *seedContext) dockerPackages() error {
	for _, name := range []string{"platform/api", "labs/worker"} {
		team, _, _ := strings.Cut(name, "/")
		if _, err := s.db.CreateDockerImageForTeam("docker", name, "admin", team, team == "labs", s.now-14*24*time.Hour.Milliseconds()); err != nil {
			return err
		}
		for i, tag := range []string{"1.0.0", "1.1.0"} {
			configuration, layer := "sha256:"+digest(name+tag+"config"), "sha256:"+digest(name+tag+"layer")
			size := int64(8+i*2) << 20
			for key, bytes := range map[string]int64{configuration: 512, layer: size} {
				if err := s.db.RecordDockerBlob("docker", key, bytes); err != nil {
					return err
				}
				if err := s.db.RecordDockerImageBlob("docker", name, key); err != nil {
					return err
				}
				if err := s.file("docker", "blobs/sha256/"+strings.TrimPrefix(key, "sha256:"), bytes); err != nil {
					return err
				}
			}
			body, err := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json",
				"config": map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": configuration, "size": 512},
				"layers": []any{map[string]any{"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip", "digest": layer, "size": size}}})
			if err != nil {
				return err
			}
			manifest := &core.DockerManifest{Repository: "docker", ImageName: name, Digest: "sha256:" + digest(string(body)),
				MediaType: "application/vnd.oci.image.manifest.v1+json", ConfigDigest: configuration, BlobDigests: []string{configuration, layer},
				RawJSON: body, Size: int64(len(body)), Publisher: "admin", CreatedAt: s.now - int64(5-i*4)*24*time.Hour.Milliseconds()}
			if err := s.db.PutDockerManifest(manifest, tag, "admin"); err != nil {
				return err
			}
			if i == 1 {
				if err := s.db.PutDockerManifest(manifest, "latest", "admin"); err != nil {
					return err
				}
			}
			if err := s.file("docker", fmt.Sprintf("manifests/%s/%s", name, strings.TrimPrefix(manifest.Digest, "sha256:")), int64(len(body))); err != nil {
				return err
			}
		}
		if err := s.db.ForceAddDockerMembers("docker", name, "admin", []string{"alice"}, core.DockerPermissionPublish); err != nil {
			return err
		}
	}
	return nil
}
