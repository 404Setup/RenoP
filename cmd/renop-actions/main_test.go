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
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"renop/pkg/hex"
	"renop/pkg/pb"
	"strings"
	"testing"

	"github.com/goccy/go-json"
	"google.golang.org/protobuf/proto"
)

func TestParseStep(t *testing.T) {
	cases := []struct {
		args     []string
		wantStep string
		wantArgs int
	}{
		{[]string{"assemble", "-dist-dir", "dist"}, "assemble", 2},
		{[]string{"-step", "changelog", "-commit", "HEAD"}, "changelog", 2},
		{[]string{"--step", "validate-payload", "-dist-dir", "dist"}, "validate-payload", 2},
		{[]string{"-step=release-index", "-url", "https://example.com"}, "release-index", 2},
		{[]string{"-dist-dir", "dist", "-step", "assemble"}, "assemble", 2},
	}

	for _, tc := range cases {
		step, remaining := parseStep(tc.args)
		if step != tc.wantStep {
			t.Errorf("parseStep(%v) step = %q, want %q", tc.args, step, tc.wantStep)
		}
		if len(remaining) != tc.wantArgs {
			t.Errorf("parseStep(%v) remaining count = %d, want %d", tc.args, len(remaining), tc.wantArgs)
		}
	}
}

func TestAssembleAndValidatePayload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "renop-actions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	packageDir := filepath.Join(tempDir, "packages")
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		t.Fatal(err)
	}
	distDir := filepath.Join(tempDir, "dist")

	payload := []byte("renop executable test fixture")
	h := sha256.Sum256(payload)
	payloadHash := hex.EncodeToString(h[:])

	targets := defaultTargets
	version := "testver"

	for _, target := range targets {
		filename := "renop-" + version + "-" + target.GOOS + "-" + target.GOARCH + ".br"
		pkgFile := filepath.Join(packageDir, filename)
		if err := os.WriteFile(pkgFile, payload, 0644); err != nil {
			t.Fatal(err)
		}
		exe := "renop"
		if target.GOOS == "windows" {
			exe = "renop.exe"
		}
		desc := TargetDescriptor{
			OS:               target.GOOS,
			Arch:             target.GOARCH,
			File:             filename,
			SHA256:           payloadHash,
			Size:             int64(len(payload)),
			UncompressedSize: int64(len(payload) * 2),
			Format:           "brotli",
			Executable:       exe,
		}
		descData, err := json.Marshal(desc)
		if err != nil {
			t.Fatal(err)
		}
		jsonFile := filepath.Join(packageDir, target.GOOS+"-"+target.GOARCH+".json")
		if err := os.WriteFile(jsonFile, descData, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Run assemble
	err = runAssemble([]string{
		"-package-dir", packageDir,
		"-dist-dir", distDir,
		"-version", version,
		"-commit", strings.Repeat("a", 40),
		"-development", "true",
	})
	if err != nil {
		t.Fatalf("runAssemble failed: %v", err)
	}

	// Verify manifest.json
	manifestData, err := os.ReadFile(filepath.Join(distDir, "manifest.json"))
	if err != nil {
		t.Fatalf("failed to read manifest.json: %v", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("failed to unmarshal manifest.json: %v", err)
	}
	if len(manifest.Targets) != len(targets) {
		t.Errorf("manifest targets count = %d, want %d", len(manifest.Targets), len(targets))
	}
	if manifest.Version != version {
		t.Errorf("manifest version = %q, want %q", manifest.Version, version)
	}

	// Run validate-payload
	err = runValidatePayload([]string{"-dist-dir", distDir})
	if err != nil {
		t.Fatalf("runValidatePayload failed: %v", err)
	}

	// Test validate-payload failure with unexpected file
	unexpectedFile := filepath.Join(distDir, "unexpected.txt")
	if err := os.WriteFile(unexpectedFile, []byte("bad"), 0644); err != nil {
		t.Fatal(err)
	}
	err = runValidatePayload([]string{"-dist-dir", distDir})
	if err == nil || !strings.Contains(err.Error(), "must not be sent to the update API") {
		t.Errorf("expected error for unexpected file, got %v", err)
	}
	os.Remove(unexpectedFile)
}

func TestChangelogGeneration(t *testing.T) {
	tempFile, err := os.CreateTemp("", "changelog-*.md")
	if err != nil {
		t.Fatal(err)
	}
	outPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(outPath)

	err = runChangelog([]string{
		"-commit", "HEAD",
		"-out", outPath,
		"-footer", "Downloads: https://example.com",
	})
	if err != nil {
		t.Fatalf("runChangelog failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read changelog: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Downloads: https://example.com") {
		t.Errorf("changelog missing footer, got:\n%s", content)
	}
}

func TestReadDirectories(t *testing.T) {
	for _, test := range []struct {
		name, contentType string
		status            int
		listing           *pb.FileDetails
		want              []string
		wantError         bool
	}{
		{name: "actual directory names", status: 200, contentType: "application/x-protobuf", listing: &pb.FileDetails{
			Type: "DIRECTORY", Files: []*pb.FileDetails{
				{Type: "DIRECTORY", Name: "abcdef12"}, {Type: "FILE", Name: "info.json"}, {Type: "DIRECTORY", Name: "12345678"},
			},
		}, want: []string{"12345678", "abcdef12"}},
		{name: "first publication", status: 404, want: []string{}},
		{name: "denied", status: 403, wantError: true},
		{name: "HTML fallback", status: 200, contentType: "text/html", wantError: true},
		{name: "wrong resource", status: 200, contentType: "application/x-protobuf", listing: &pb.FileDetails{Type: "FILE"}, wantError: true},
		{name: "path escape", status: 200, contentType: "application/x-protobuf", listing: &pb.FileDetails{
			Type: "DIRECTORY", Files: []*pb.FileDetails{{Type: "DIRECTORY", Name: "../stable"}},
		}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture" || r.Header.Get("Accept") != "application/x-protobuf" {
					t.Error("missing authenticated protobuf request headers")
				}
				w.Header().Set("Content-Type", test.contentType)
				w.WriteHeader(test.status)
				if test.listing != nil {
					body, err := proto.Marshal(test.listing)
					if err != nil {
						t.Error(err)
					}
					_, _ = w.Write(body)
				}
			}))
			defer server.Close()
			got, err := readDirectories(context.Background(), server.Client(), server.URL, "fixture")
			if (err != nil) != test.wantError || !test.wantError && !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %v, %v; want %v, error=%v", got, err, test.want, test.wantError)
			}
		})
	}
}
