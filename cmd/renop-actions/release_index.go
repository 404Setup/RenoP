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
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-json"
	"google.golang.org/protobuf/proto"

	"renop/pkg/pb"
)

func runReleaseIndex(args []string) error {
	fs := flag.NewFlagSet("release-index", flag.ContinueOnError)
	url := fs.String("url", "", "Repository directory API URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	token := os.Getenv("RENOP_PUBLISH_TOKEN")
	if token == "" {
		token = os.Getenv("MVNC_TOKEN")
	}

	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("release inventory redirects are not permitted")
	}}

	directories, err := readDirectories(context.Background(), client, *url, token)
	if err == nil {
		err = json.NewEncoder(os.Stdout).Encode(directories)
	}
	return err
}

func readDirectories(ctx context.Context, client *http.Client, url, token string) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/x-protobuf")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return []string{}, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release inventory returned HTTP %d", response.StatusCode)
	}
	if strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]) != "application/x-protobuf" {
		return nil, errors.New("release inventory did not return protobuf")
	}
	const maxBytes = 8 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBytes {
		return nil, errors.New("release inventory exceeds 8 MiB")
	}
	var listing pb.FileDetails
	if err := proto.Unmarshal(body, &listing); err != nil {
		return nil, fmt.Errorf("decode release inventory: %w", err)
	}
	if listing.GetType() != "DIRECTORY" {
		return nil, errors.New("release inventory is not a directory")
	}
	directories := make([]string, 0)
	for _, file := range listing.GetFiles() {
		if file.GetType() != "DIRECTORY" {
			continue
		}
		name := file.GetName()
		if name == "" || len(name) > 128 || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
			return nil, errors.New("release inventory contains an unsafe directory name")
		}
		directories = append(directories, name)
		if len(directories) > 4096 {
			return nil, errors.New("release inventory exceeds 4096 entries")
		}
	}
	sort.Strings(directories)
	return directories, nil
}
