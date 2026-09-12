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
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"renop/pkg/pb"

	"google.golang.org/protobuf/proto"
)

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
