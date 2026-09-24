/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goccy/go-json"
)

func runAssemble(args []string) error {
	fs := flag.NewFlagSet("assemble", flag.ContinueOnError)
	packageDir := fs.String("package-dir", "", "Directory containing built package artifacts")
	PackageDir := fs.String("PackageDir", "", "Alias for package-dir")
	distDir := fs.String("dist-dir", "", "Output distribution directory")
	DistDir := fs.String("DistDir", "", "Alias for dist-dir")
	version := fs.String("version", "", "Version string or commit SHA")
	Version := fs.String("Version", "", "Alias for version")
	devStr := fs.String("development", "false", "Whether this is a development build")
	Development := fs.String("Development", "", "Alias for development")
	commit := fs.String("commit", "", "Commit SHA")
	Commit := fs.String("Commit", "", "Alias for commit")
	prevCommit := fs.String("previous-commit", "", "Preceding commit SHA")
	PreviousCommit := fs.String("PreviousCommit", "", "Alias for previous-commit")
	targetsFile := fs.String("targets-file", "", "Optional path to build-targets.psd1")

	if err := fs.Parse(args); err != nil {
		return err
	}

	pkgDir := *packageDir
	if pkgDir == "" {
		pkgDir = *PackageDir
	}
	dDir := *distDir
	if dDir == "" {
		dDir = *DistDir
	}
	ver := *version
	if ver == "" {
		ver = *Version
	}
	cmt := *commit
	if cmt == "" {
		cmt = *Commit
	}
	prev := *prevCommit
	if prev == "" {
		prev = *PreviousCommit
	}
	isDev := *devStr
	if *Development != "" {
		isDev = *Development
	}

	if pkgDir == "" || dDir == "" || ver == "" || cmt == "" {
		return errors.New("PackageDir, DistDir, Version, and Commit are required.")
	}

	targets, err := loadTargets(*targetsFile)
	if err != nil {
		return err
	}

	pkgDir, err = filepath.Abs(pkgDir)
	if err != nil {
		return err
	}

	displayVersion := ver
	if hexHashPattern.MatchString(ver) {
		displayVersion = ver[:7]
	}
	safeVersion := safeVersionPattern.ReplaceAllString(displayVersion, "_")

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() {
			return fmt.Errorf("Expected exactly %d packages and target descriptors.", len(targets))
		}
	}
	if len(entries) != len(targets)*2 {
		return fmt.Errorf("Expected exactly %d packages and target descriptors.", len(targets))
	}

	descriptors := make([]TargetDescriptor, 0, len(targets))
	for _, t := range targets {
		goos, goarch := t.GOOS, t.GOARCH
		jsonPath := filepath.Join(pkgDir, fmt.Sprintf("%s-%s.json", goos, goarch))
		jsonData, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("Expected exactly %d packages and target descriptors.", len(targets))
		}
		var desc TargetDescriptor
		if err := json.Unmarshal(jsonData, &desc); err != nil {
			return fmt.Errorf("Invalid package descriptor for %s/%s", goos, goarch)
		}

		filename := fmt.Sprintf("renop-%s-%s-%s.br", safeVersion, goos, goarch)
		executable := "renop"
		if goos == "windows" {
			executable = "renop.exe"
		}

		if desc.OS != goos || desc.Arch != goarch || desc.File != filename ||
			desc.Executable != executable || desc.Format != "brotli" ||
			!sha256Pattern.MatchString(desc.SHA256) || desc.Size <= 0 || desc.UncompressedSize <= 0 {
			return fmt.Errorf("Invalid package descriptor for %s/%s", goos, goarch)
		}

		pkgPath := filepath.Join(pkgDir, filename)
		pkgInfo, err := os.Stat(pkgPath)
		if err != nil || pkgInfo.Size() != desc.Size {
			return fmt.Errorf("Package hash or size mismatch for %s/%s", goos, goarch)
		}

		hash, err := fileSHA256(pkgPath)
		if err != nil || !strings.EqualFold(hash, desc.SHA256) {
			return fmt.Errorf("Package hash or size mismatch for %s/%s", goos, goarch)
		}

		descriptors = append(descriptors, desc)
	}

	dDir, err = filepath.Abs(dDir)
	if err != nil {
		return err
	}

	if info, err := os.Stat(dDir); err == nil && info.IsDir() {
		distEntries, err := os.ReadDir(dDir)
		if err != nil {
			return err
		}
		if len(distEntries) > 0 {
			return errors.New("Release output directory must be empty.")
		}
	} else {
		if err := os.MkdirAll(dDir, 0755); err != nil {
			return err
		}
	}

	for _, d := range descriptors {
		src := filepath.Join(pkgDir, d.File)
		dst := filepath.Join(dDir, d.File)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", d.File, err)
		}
	}

	manifest := Manifest{
		Version:        displayVersion,
		Commit:         cmt,
		PreviousCommit: prev,
		Development:    strings.EqualFold(isDev, "true"),
		Targets:        descriptors,
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	manifestPath := filepath.Join(dDir, "manifest.json")
	if err := os.WriteFile(manifestPath, append(manifestBytes, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Assembled %d verified release targets.\n", len(descriptors))
	return nil
}

func loadTargets(targetsFile string) ([]Target, error) {
	if targetsFile != "" {
		if data, err := os.ReadFile(targetsFile); err == nil {
			if parsed := parsePsd1Targets(string(data)); len(parsed) > 0 {
				return parsed, nil
			}
		}
	}
	for _, cand := range []string{"scripts/build-targets.psd1", "../../scripts/build-targets.psd1"} {
		if data, err := os.ReadFile(cand); err == nil {
			if parsed := parsePsd1Targets(string(data)); len(parsed) > 0 {
				return parsed, nil
			}
		}
	}
	if dir, err := os.Getwd(); err == nil {
		for range 5 {
			cand := filepath.Join(dir, "scripts", "build-targets.psd1")
			if data, err := os.ReadFile(cand); err == nil {
				if parsed := parsePsd1Targets(string(data)); len(parsed) > 0 {
					return parsed, nil
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return defaultTargets, nil
}

func parsePsd1Targets(content string) []Target {
	re := regexp.MustCompile(`GOOS\s*=\s*'([^']+)';\s*GOARCH\s*=\s*'([^']+)'`)
	matches := re.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	targets := make([]Target, 0, len(matches))
	for _, m := range matches {
		targets = append(targets, Target{GOOS: m[1], GOARCH: m[2]})
	}
	return targets
}
