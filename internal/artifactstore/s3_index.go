/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package artifactstore

import (
	"context"
	"crypto/sha256"
	"hash"
	"io"
	"strconv"
	"time"

	"renop/pkg/hex"

	"github.com/minio/minio-go/v7"
)

type S3Record struct {
	Digest        string `json:"sha256,omitempty"`
	ETag          string `json:"etag,omitempty"`
	Size          int64  `json:"size"`
	ObjectSize    int64  `json:"object_size"`
	ModTime       int64  `json:"mod_time"`
	Reference     bool   `json:"reference,omitempty"`
	PreservedTime bool   `json:"preserved_time,omitempty"`
	Verified      bool   `json:"-"`
}

// S3Index supplies persisted transfer metadata and a local reclamation queue.
// Implementations must not perform remote I/O in these operations.
type S3Index interface {
	Get(string) (S3Record, bool)
	Put(string, S3Record)
	Remove(string)
	Has(string) bool
	MarkGarbage(string, int64)
	ForgetGarbage(string, int64)
	Garbage(int, int64) []S3Garbage
}

type S3Garbage struct {
	Digest string
	Since  int64
}

func (record S3Record) objectInfo() minio.ObjectInfo {
	info := minio.ObjectInfo{ETag: record.ETag, Size: record.ObjectSize, LastModified: time.Unix(0, record.ModTime).UTC()}
	if record.Reference {
		info.ContentType = referenceContentType
		info.UserMetadata = minio.StringMap{"renop-reference": "1", "renop-sha256": record.Digest, "renop-size": strconv.FormatInt(record.Size, 10)}
		if record.PreservedTime {
			info.UserMetadata["renop-modtime"] = strconv.FormatInt(record.ModTime, 10)
		}
	}
	return info
}

func (s S3) remember(key string, info minio.ObjectInfo) error {
	if s.Index == nil {
		return nil
	}
	digest, size, reference, err := Reference(info)
	if err != nil {
		return err
	}
	_, logical, err := s.ResolveInfo(key, info)
	if err != nil {
		return err
	}
	record := S3Record{Digest: digest, Size: size, ObjectSize: info.Size, ETag: info.ETag, ModTime: logical.LastModified.UnixNano(), Reference: reference, PreservedTime: metadata(info, "renop-modtime") != "", Verified: true}
	if old, ok := s.Index.Get(key); ok && old.ETag == record.ETag && old.ObjectSize == record.ObjectSize && !reference {
		record.Digest = old.Digest
	}
	s.Index.Put(key, record)
	return nil
}

func (s S3) stat(ctx context.Context, key string) (minio.ObjectInfo, error) {
	if s.Index != nil {
		if record, ok := s.Index.Get(key); ok && record.Verified {
			info := record.objectInfo()
			info.Key = key
			return info, nil
		}
	}
	info, err := s.Client.StatObject(ctx, s.Bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return info, err
	}
	return info, s.remember(key, info)
}

// ResolveListed uses one LIST result to validate cached metadata. A missing
// reference record needs one HEAD; ordinary files never do.
func (s S3) ResolveListed(ctx context.Context, info minio.ObjectInfo) (minio.ObjectInfo, error) {
	if s.Index != nil {
		if record, ok := s.Index.Get(info.Key); ok && record.ETag == info.ETag && record.ObjectSize == info.Size {
			record.Verified = true
			if !record.PreservedTime {
				record.ModTime = info.LastModified.UnixNano()
			}
			s.Index.Put(info.Key, record)
			_, logical, err := s.ResolveInfo(info.Key, record.objectInfo())
			logical.Key = info.Key
			return logical, err
		}
	}
	if (info.Size == 0 || info.Size == s3ReferenceBodySize) && metadata(info, "renop-reference") == "" {
		remote, err := s.Client.StatObject(ctx, s.Bucket, info.Key, minio.StatObjectOptions{})
		if err != nil {
			return info, err
		}
		remote.Key = info.Key
		info = remote
	}
	if err := s.remember(info.Key, info); err != nil {
		return info, err
	}
	_, logical, err := s.ResolveInfo(info.Key, info)
	return logical, err
}

func (s S3) TrackRead(key string, reader io.ReadCloser, info minio.ObjectInfo) io.ReadCloser {
	if s.Index == nil || info.Size < S3DedupMinimumSize {
		return reader
	}
	record, ok := s.Index.Get(key)
	if !ok || record.Reference || record.Digest != "" {
		return reader
	}
	return &s3LearningReader{ReadCloser: reader, index: s.Index, key: key, record: record, hasher: sha256.New()}
}

type s3LearningReader struct {
	io.ReadCloser
	index    S3Index
	key      string
	record   S3Record
	hasher   hash.Hash
	read     int64
	disabled bool
}

func (r *s3LearningReader) Read(data []byte) (int, error) {
	n, err := r.ReadCloser.Read(data)
	if !r.disabled {
		r.hasher.Write(data[:n])
		r.read += int64(n)
		if r.read == r.record.Size {
			r.disabled = true
			if current, ok := r.index.Get(r.key); ok && current.ETag == r.record.ETag && current.Size == r.record.Size {
				current.Digest = hex.EncodeToString(r.hasher.Sum(nil))
				r.index.Put(r.key, current)
			}
		} else if err != nil && err != io.EOF {
			r.disabled = true
			r.index.Remove(r.key)
		}
	}
	return n, err
}

func (r *s3LearningReader) ReadAt(data []byte, offset int64) (int, error) {
	r.disabled = true
	reader, ok := r.ReadCloser.(io.ReaderAt)
	if !ok {
		return 0, io.ErrNoProgress
	}
	return reader.ReadAt(data, offset)
}

// DeduplicateKnown uses a digest learned during an ordinary transfer. The source
// is copied inside S3 only when the canonical object is not already indexed.
func (s S3) DeduplicateKnown(ctx context.Context, key string, record S3Record) error {
	if !record.Verified || record.Reference || record.Digest == "" || record.ETag == "" || record.Size < S3DedupMinimumSize {
		return nil
	}
	prior := record.objectInfo()
	return s.install(ctx, key, record.Digest, record.Size, &prior, func(blob string) error {
		src := minio.CopySrcOptions{Bucket: s.Bucket, Object: key, MatchETag: record.ETag}
		dst := minio.CopyDestOptions{Bucket: s.Bucket, Object: blob}
		if record.Size <= 5<<30 {
			_, err := s.Client.CopyObject(ctx, dst, src)
			return err
		}
		_, err := s.Client.ComposeObject(ctx, dst, src)
		return err
	})
}
