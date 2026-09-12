/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package storage

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"renop/internal/config"
)

func TestCondaNativeClient(t *testing.T) {
	client := os.Getenv("RENOP_MICROMAMBA")
	if client == "" {
		t.Skip("isolated micromamba client not configured")
	}
	app := nativeClientTestServer(t, "conda", config.RepositoryFormatCondaNative)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	t.Cleanup(func() { _ = app.Shutdown(); <-done })
	url := "http://" + listener.Addr().String() + "/conda"
	request, err := http.NewRequest("PUT", url+"/noarch/example-1.0-0.conda", bytes.NewReader(condaStoragePackage(t)))
	if err != nil {
		t.Fatal(err)
	}
	request.SetBasicAuth("writer", "conan-test-password")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 201 {
		t.Fatalf("upload: %d %s %v", response.StatusCode, body, err)
	}
	root := storageTestTempDir(t)
	environment := filepath.Join(root, "environment")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "--no-rc", "--root-prefix", filepath.Join(root, "root"), "create", "--prefix", environment, "--override-channels", "--channel", url, "example", "--yes", "--json")
	cmd.Env = append(os.Environ(), "NO_PROXY=127.0.0.1,localhost,::1", "no_proxy=127.0.0.1,localhost,::1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("micromamba install: %v\n%s", err, output)
	}
	installed, err := os.ReadFile(filepath.Join(environment, "share", "example.txt"))
	if err != nil || string(installed) != "example\n" {
		t.Fatalf("installed payload: %q %v\n%s", installed, err, output)
	}
}
