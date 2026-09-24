/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Command renop-actions provides unified workflow automation steps for RenoP CI/CD.
package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"renop/pkg/hex"
)

type Target struct {
	GOOS   string
	GOARCH string
}

type TargetDescriptor struct {
	OS               string `json:"os"`
	Arch             string `json:"arch"`
	File             string `json:"file"`
	SHA256           string `json:"sha256"`
	Size             int64  `json:"size"`
	UncompressedSize int64  `json:"uncompressed_size"`
	Format           string `json:"format"`
	Executable       string `json:"executable"`
}

type Manifest struct {
	Version        string             `json:"version"`
	Commit         string             `json:"commit"`
	PreviousCommit string             `json:"previous_commit"`
	Development    bool               `json:"development"`
	Targets        []TargetDescriptor `json:"targets"`
}

var defaultTargets = []Target{
	{GOOS: "darwin", GOARCH: "amd64"},
	{GOOS: "darwin", GOARCH: "amd64v2"},
	{GOOS: "darwin", GOARCH: "amd64v3"},
	{GOOS: "darwin", GOARCH: "amd64v4"},
	{GOOS: "darwin", GOARCH: "arm64"},
	{GOOS: "freebsd", GOARCH: "amd64"},
	{GOOS: "freebsd", GOARCH: "amd64v2"},
	{GOOS: "freebsd", GOARCH: "amd64v3"},
	{GOOS: "freebsd", GOARCH: "amd64v4"},
	{GOOS: "freebsd", GOARCH: "arm64"},
	{GOOS: "linux", GOARCH: "amd64"},
	{GOOS: "linux", GOARCH: "amd64v2"},
	{GOOS: "linux", GOARCH: "amd64v3"},
	{GOOS: "linux", GOARCH: "amd64v4"},
	{GOOS: "linux", GOARCH: "arm64"},
	{GOOS: "linux", GOARCH: "loong64"},
	{GOOS: "linux", GOARCH: "riscv64"},
	{GOOS: "netbsd", GOARCH: "amd64"},
	{GOOS: "netbsd", GOARCH: "amd64v2"},
	{GOOS: "netbsd", GOARCH: "amd64v3"},
	{GOOS: "netbsd", GOARCH: "amd64v4"},
	{GOOS: "openbsd", GOARCH: "amd64"},
	{GOOS: "openbsd", GOARCH: "amd64v2"},
	{GOOS: "openbsd", GOARCH: "amd64v3"},
	{GOOS: "openbsd", GOARCH: "amd64v4"},
	{GOOS: "openbsd", GOARCH: "arm64"},
	{GOOS: "windows", GOARCH: "amd64"},
	{GOOS: "windows", GOARCH: "amd64v2"},
	{GOOS: "windows", GOARCH: "amd64v3"},
	{GOOS: "windows", GOARCH: "amd64v4"},
	{GOOS: "windows", GOARCH: "arm64"},
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
var hexHashPattern = regexp.MustCompile(`^(?i:[0-9a-f]{40}|[0-9a-f]{64})$`)
var safeVersionPattern = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func main() {
	step, remainingArgs := parseStep(os.Args[1:])
	if step == "" {
		fmt.Fprintln(os.Stderr, "Usage: renop-actions -step <assemble|validate-payload|changelog|release-index> [options]")
		os.Exit(1)
	}

	var err error
	switch step {
	case "assemble":
		err = runAssemble(remainingArgs)
	case "validate-payload", "validate", "test-release-payload", "test-payload":
		err = runValidatePayload(remainingArgs)
	case "changelog":
		err = runChangelog(remainingArgs)
	case "release-index", "inventory":
		err = runReleaseIndex(remainingArgs)
	default:
		err = fmt.Errorf("unknown step: %s", step)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseStep(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	// Check positional step
	if !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	// Check -step or --step
	for i := range args {
		if (args[i] == "-step" || args[i] == "--step") && i+1 < len(args) {
			step := args[i+1]
			remaining := append(append([]string{}, args[:i]...), args[i+2:]...)
			return step, remaining
		}
		if strings.HasPrefix(args[i], "-step=") || strings.HasPrefix(args[i], "--step=") {
			parts := strings.SplitN(args[i], "=", 2)
			step := parts[1]
			remaining := append(append([]string{}, args[:i]...), args[i+1:]...)
			return step, remaining
		}
	}
	// Backward-compatibility: if -url is passed, treat as release-index
	for _, a := range args {
		if a == "-url" || strings.HasPrefix(a, "-url=") {
			return "release-index", args
		}
	}
	return "", args
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
