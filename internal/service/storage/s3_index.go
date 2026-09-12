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
	"context"
	"crypto/sha256"
	"strings"
	"sync/atomic"
	"time"

	"renop/internal/artifactstore"
	"renop/internal/config"
	"renop/internal/service/index"
	"renop/internal/utils"
	"renop/pkg/hex"

	"github.com/minio/minio-go/v7"
)

var s3FileIndex atomic.Pointer[index.FileIndex]

func BindS3Index(files *index.FileIndex) { s3FileIndex.Store(files) }

type s3MetadataIndex struct {
	files *index.FileIndex
	scope string
}

// Index rebuilds discover interrupted uploads from content-addressed names. This
// requires LIST pages only, without reading payloads or probing every live blob.
func discoverS3Garbage(ctx context.Context, store artifactstore.S3) error {
	metadata, ok := store.Index.(*s3MetadataIndex)
	if !ok {
		return nil
	}
	prefix := strings.TrimSuffix(store.Prefix, "/") + "/"
	ctx, cancel := context.WithTimeout(ctx, s3TransferTimeout)
	defer cancel()
	for entry := range store.Client.ListObjects(ctx, store.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: false}) {
		if entry.Err != nil {
			return entry.Err
		}
		digest := strings.TrimSuffix(strings.TrimPrefix(entry.Key, prefix), "/")
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != sha256.Size {
			continue
		}
		metadata.files.EnsureContentGarbage(metadata.key(digest), time.Now().Unix())
	}
	return ctx.Err()
}

func pruneS3Metadata(cfg *config.S3Config, root string, scanned *index.FileIndex, started int64) {
	store, err := s3ContentStore(cfg)
	if err != nil {
		return
	}
	metadata, ok := store.Index.(*s3MetadataIndex)
	if !ok {
		return
	}
	prefix, err := s3ObjectKey(cfg, utils.GetS3Key(root))
	if err != nil {
		return
	}
	prefix = strings.TrimSuffix(prefix, "/") + "/"
	metadata.files.RangeContent(metadata.scope, func(key string, value index.ContentInfo) bool {
		local, ok := localPathFromS3Object(root, prefix, key)
		if ok && value.ModTime <= started && !scanned.HasFile(local) {
			metadata.files.RemoveContentIfMatch(metadata.key(key), value)
		}
		return true
	})
}

func newS3MetadataIndex(store artifactstore.S3) *s3MetadataIndex {
	files := s3FileIndex.Load()
	if files == nil {
		return nil
	}
	sum := sha256.Sum256([]byte(store.Client.EndpointURL().String() + "\x00" + store.Bucket + "\x00" + store.Prefix))
	return &s3MetadataIndex{files, hex.EncodeToString(sum[:])}
}

func (i *s3MetadataIndex) key(key string) string { return i.scope + "\x00" + key }
func (i *s3MetadataIndex) Get(key string) (artifactstore.S3Record, bool) {
	v, ok := i.files.GetContent(i.key(key))
	return artifactstore.S3Record(v), ok
}
func (i *s3MetadataIndex) Put(key string, v artifactstore.S3Record) {
	i.files.PutContent(i.key(key), index.ContentInfo(v))
}
func (i *s3MetadataIndex) Remove(key string)      { i.files.RemoveContent(i.key(key)) }
func (i *s3MetadataIndex) Has(digest string) bool { return i.files.HasContent(i.key(digest)) }
func (i *s3MetadataIndex) MarkGarbage(digest string, since int64) {
	i.files.MarkContentGarbage(i.key(digest), since)
}
func (i *s3MetadataIndex) ForgetGarbage(digest string, since int64) {
	i.files.ForgetContentGarbage(i.key(digest), since)
}
func (i *s3MetadataIndex) Garbage(limit int, before int64) []artifactstore.S3Garbage {
	values := i.files.ContentGarbage(i.scope, limit, before)
	result := make([]artifactstore.S3Garbage, 0, len(values))
	for _, value := range values {
		result = append(result, artifactstore.S3Garbage{Digest: value.Digest, Since: value.Since})
	}
	return result
}
