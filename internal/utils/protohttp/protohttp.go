/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package protohttp reads and writes bounded protobuf HTTP payloads.
package protohttp

import (
	"mime"

	"github.com/gofiber/fiber/v3"
	"google.golang.org/protobuf/proto"

	"renop/internal/utils"
)

// ContentType is the MIME type used for protobuf request/response bodies.
const ContentType = "application/x-protobuf"

// MaxRequestBodySize bounds control-plane requests without limiting
// streamed artifact uploads handled by the storage routes.
const MaxRequestBodySize = 1 << 20

// Write encodes m using the protobuf binary representation.
func Write(c fiber.Ctx, m proto.Message) error {
	return WriteStatus(c, fiber.StatusOK, m)
}

// WriteStatus is Write with an explicit HTTP status code.
func WriteStatus(c fiber.Ctx, status int, m proto.Message) error {
	c.Vary(fiber.HeaderAccept)
	data, err := proto.Marshal(m)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to encode response")
	}
	c.Set(fiber.HeaderContentType, ContentType)
	return c.Status(status).Send(data)
}

// Read decodes a size-limited request according to Content-Type; only binary protobuf is accepted.
func Read(c fiber.Ctx, m proto.Message) error {
	return ReadLimit(c, m, MaxRequestBodySize)
}

// ReadLimit applies a caller-owned size bound for schema-backed payloads such as legal documents.
func ReadLimit(c fiber.Ctx, m proto.Message, maxBytes int64) error {
	body, err := utils.ReadRequestBodyLimited(c, maxBytes)
	if err != nil {
		return err
	}
	contentType, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType, ContentType))
	if err != nil {
		return fiber.ErrBadRequest
	}
	switch contentType {
	case ContentType, "application/protobuf", fiber.MIMEOctetStream:
		return proto.Unmarshal(body, m)
	default:
		return fiber.ErrUnsupportedMediaType
	}
}
