/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package protohttp

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
	"google.golang.org/protobuf/proto"

	"renop/pkg/pb"
)

func TestReadDecodesBoundedStreamingBody(t *testing.T) {
	body, err := proto.Marshal(&pb.LoginRequest{Name: "admin", Secret: "secret"})
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New(fiber.Config{StreamRequestBody: true})
	app.Post("/", func(c fiber.Ctx) error {
		var req pb.LoginRequest
		if err := Read(c, &req); err != nil {
			return err
		}
		if req.GetName() != "admin" || req.GetSecret() != "secret" {
			t.Fatalf("unexpected request: %v", &req)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusNoContent)
	}
}

func TestReadRejectsOversizedBody(t *testing.T) {
	body := bytes.Repeat([]byte{'x'}, MaxRequestBodySize+1)
	fastCtx := &fasthttp.RequestCtx{}
	fastCtx.Request.SetBodyStream(bytes.NewReader(body), -1)
	app := fiber.New()
	ctx := app.AcquireCtx(fastCtx)
	defer app.ReleaseCtx(ctx)

	var req pb.LoginRequest
	if err := Read(ctx, &req); err != fiber.ErrRequestEntityTooLarge {
		t.Fatalf("Read error = %v, want %v", err, fiber.ErrRequestEntityTooLarge)
	}
}

func TestProtobufReadWrite(t *testing.T) {
	want := &pb.SessionDto{PublicId: "public-session", CreatedAt: 1 << 54}
	binary, err := proto.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	app := fiber.New(fiber.Config{StreamRequestBody: true})
	app.Post("/", func(c fiber.Ctx) error {
		var request pb.SessionDto
		if err := Read(c, &request); err != nil {
			return err
		}
		c.Set(fiber.HeaderVary, "Origin")
		return WriteStatus(c, fiber.StatusCreated, &request)
	})

	cases := []struct {
		name, contentType, accept string
		wantStatus                int
	}{
		{"default content type", "", "", fiber.StatusCreated},
		{"binary content type", ContentType, "", fiber.StatusCreated},
		{"protobuf alias", "application/protobuf", "", fiber.StatusCreated},
		{"octet stream", fiber.MIMEOctetStream, "", fiber.StatusCreated},
		{"json request rejected", fiber.MIMEApplicationJSON, "", fiber.StatusUnsupportedMediaType},
		{"json in accept header still returns protobuf", ContentType, "application/json", fiber.StatusCreated},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(binary))
			if tc.contentType != "" {
				request.Header.Set(fiber.HeaderContentType, tc.contentType)
			}
			if tc.accept != "" {
				request.Header.Set(fiber.HeaderAccept, tc.accept)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.wantStatus)
			}
			if tc.wantStatus == fiber.StatusCreated {
				if response.Header.Get(fiber.HeaderContentType) != ContentType {
					t.Fatalf("content-type = %s, want %s", response.Header.Get(fiber.HeaderContentType), ContentType)
				}
				body, err := io.ReadAll(response.Body)
				if err != nil {
					t.Fatal(err)
				}
				var got pb.SessionDto
				if err := proto.Unmarshal(body, &got); err != nil {
					t.Fatalf("failed to unmarshal protobuf: %v", err)
				}
				if !proto.Equal(want, &got) {
					t.Fatalf("decoded response differs: %v, %v", &got, want)
				}
			}
		})
	}
}
