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
	"sort"
	"strings"

	"github.com/goccy/go-json"
)

func runValidatePayload(args []string) error {
	fs := flag.NewFlagSet("validate-payload", flag.ContinueOnError)
	distDir := fs.String("dist-dir", "", "Build output directory")
	DistDir := fs.String("DistDir", "", "Alias for dist-dir")

	if err := fs.Parse(args); err != nil {
		return err
	}

	dDir := *distDir
	if dDir == "" {
		dDir = *DistDir
	}
	if dDir == "" {
		return errors.New("DistDir is required")
	}

	dDir, err := filepath.Abs(dDir)
	if err != nil {
		return err
	}

	info, err := os.Stat(dDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("Release payload directory not found: %s", dDir)
	}

	entries, err := os.ReadDir(dDir)
	if err != nil {
		return err
	}

	var packages []string
	var unexpected []string
	hasManifest := false

	for _, e := range entries {
		if e.IsDir() {
			unexpected = append(unexpected, e.Name())
			continue
		}
		if e.Name() == "manifest.json" {
			hasManifest = true
		} else if strings.HasSuffix(e.Name(), ".br") {
			packages = append(packages, e.Name())
		} else {
			unexpected = append(unexpected, e.Name())
		}
	}

	if len(packages) == 0 {
		return fmt.Errorf("Release payload contains no Brotli packages: %s", dDir)
	}
	manifestPath := filepath.Join(dDir, "manifest.json")
	if !hasManifest {
		return fmt.Errorf("Release payload manifest not found: %s", manifestPath)
	}
	if len(unexpected) > 0 {
		return fmt.Errorf("Release payload contains files that must not be sent to the update API: %s", strings.Join(unexpected, ", "))
	}

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	// Strip UTF-8 BOM if present
	if len(manifestData) >= 3 && manifestData[0] == 0xEF && manifestData[1] == 0xBB && manifestData[2] == 0xBF {
		manifestData = manifestData[3:]
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	manifestFilesSet := make(map[string]struct{})
	for _, t := range manifest.Targets {
		manifestFilesSet[t.File] = struct{}{}
	}
	var manifestFiles []string
	for f := range manifestFilesSet {
		manifestFiles = append(manifestFiles, f)
	}
	sort.Strings(manifestFiles)

	packageSet := make(map[string]struct{})
	for _, p := range packages {
		packageSet[p] = struct{}{}
	}
	var packageNames []string
	for p := range packageSet {
		packageNames = append(packageNames, p)
	}
	sort.Strings(packageNames)

	if len(manifestFiles) != len(packageNames) {
		return fmt.Errorf("Manifest target count %d does not match package count %d", len(manifestFiles), len(packageNames))
	}

	for i := range packageNames {
		if packageNames[i] != manifestFiles[i] {
			return fmt.Errorf("Manifest target '%s' does not match package '%s'", manifestFiles[i], packageNames[i])
		}
	}

	fmt.Printf("Validated update payload: %d Brotli package(s) and manifest.json\n", len(packageNames))
	return nil
}
