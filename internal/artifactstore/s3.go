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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"renop/pkg/hex"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const S3DedupMinimumSize int64 = 64 << 10
const referenceContentType = "application/vnd.renop.content-reference.v1"
const s3ReferenceBodySize = 97
const s3CollectionGrace = 24 * time.Hour

var s3TargetLocks [128]sync.Mutex
var s3ContentLocks [128]sync.Mutex

// S3 keeps logical objects and their timestamps separate from immutable payloads.
// Durable reference markers make interrupted uploads and deletes recoverable.
// Prefix is a private namespace outside every repository's visible object root.
type S3 struct {
	Client         *minio.Client
	Bucket, Prefix string
	Index          S3Index
}

func (s S3) lock(key string, content bool) func() {
	sum := sha256.Sum256([]byte(s.Client.EndpointURL().String() + "\x00" + s.Bucket + "\x00" + s.Prefix + "\x00" + key))
	locks := &s3TargetLocks
	if content {
		locks = &s3ContentLocks
	}
	mutex := &locks[int(sum[0])%len(locks)]
	mutex.Lock()
	return mutex.Unlock
}

func (s S3) blob(digest string) string {
	return strings.TrimSuffix(s.Prefix, "/") + "/" + digest + "/data"
}
func (s S3) marker(digest, key string) string {
	sum := sha256.Sum256([]byte(key))
	return strings.TrimSuffix(s.Prefix, "/") + "/" + digest + "/refs/" + hex.EncodeToString(sum[:])
}

func metadata(info minio.ObjectInfo, name string) string {
	for key, value := range info.UserMetadata {
		if strings.TrimPrefix(strings.ToLower(key), "x-amz-meta-") == name {
			return value
		}
	}
	return info.Metadata.Get("X-Amz-Meta-" + name)
}

// Reference recognizes only server-created metadata, never user-controlled bytes.
func Reference(info minio.ObjectInfo) (digest string, size int64, reference bool, err error) {
	if metadata(info, "renop-reference") != "1" {
		return "", info.Size, false, nil
	}
	digest = metadata(info, "renop-sha256")
	raw, decodeErr := hex.DecodeString(digest)
	size, err = strconv.ParseInt(metadata(info, "renop-size"), 10, 64)
	if decodeErr != nil || len(raw) != sha256.Size || size < S3DedupMinimumSize || err != nil || (info.Size != 0 && info.Size != s3ReferenceBodySize) {
		return "", 0, true, errors.New("invalid S3 content reference")
	}
	return strings.ToLower(digest), size, true, nil
}

func (s S3) Resolve(ctx context.Context, key string) (string, minio.ObjectInfo, error) {
	info, err := s.stat(ctx, key)
	if err != nil {
		return "", info, err
	}
	return s.ResolveInfo(key, info)
}

func (s S3) ResolveInfo(key string, info minio.ObjectInfo) (string, minio.ObjectInfo, error) {
	digest, size, reference, err := Reference(info)
	if err != nil {
		return "", info, err
	}
	if reference {
		info.Size = size
		if modified := metadata(info, "renop-modtime"); modified != "" {
			stamp, err := strconv.ParseInt(modified, 10, 64)
			if err != nil || stamp <= 0 {
				return "", info, errors.New("invalid S3 reference timestamp")
			}
			info.LastModified = time.Unix(0, stamp).UTC()
		}
		return s.blob(digest), info, nil
	}
	return key, info, nil
}

func (s S3) PutFile(ctx context.Context, key, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < S3DedupMinimumSize {
		return s.PutRaw(ctx, key, file, info.Size(), "")
	}
	hasher := sha256.New()
	size, err := io.Copy(hasher, contextReader{ctx, file})
	if err != nil {
		return err
	}
	if size != info.Size() {
		return errors.New("staged artifact changed during hashing")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	return s.install(ctx, key, digest, size, nil, func(blob string) error {
		_, err := s.Client.PutObject(ctx, s.Bucket, blob, file, size, minio.PutObjectOptions{ContentType: "application/octet-stream"})
		return err
	})
}

func (s S3) install(ctx context.Context, key, digest string, size int64, prior *minio.ObjectInfo, upload func(string) error) error {
	if !validS3Digest(digest) {
		return errors.New("invalid S3 content digest")
	}
	unlock := s.lock(key, false)
	defer unlock()
	old, err := s.oldDigest(ctx, key)
	if err != nil {
		return err
	}
	if prior != nil {
		current, err := s.stat(ctx, key)
		if err != nil {
			return err
		}
		if current.ETag != prior.ETag {
			return errors.New("S3 artifact changed during deduplication")
		}
	}
	err = func() error {
		unlock := s.lock(digest, true)
		defer unlock()
		blob := s.blob(digest)
		if s.Index != nil {
			s.Index.MarkGarbage(digest, time.Now().Unix())
		}
		var err error
		if s.Index == nil || !s.Index.Has(digest) {
			stored, statErr := s.Client.StatObject(ctx, s.Bucket, blob, minio.StatObjectOptions{})
			if missingS3(statErr) {
				err = upload(blob)
			} else if statErr != nil {
				err = statErr
			} else if stored.Size != size {
				err = errors.New("S3 content size mismatch")
			}
		}
		if err != nil {
			return err
		}
		// Marker before pointer: an unknown PUT outcome must retain the payload.
		if _, err := s.Client.PutObject(ctx, s.Bucket, s.marker(digest, key), strings.NewReader(key), int64(len(key)), minio.PutObjectOptions{ContentType: "text/plain"}); err != nil {
			return err
		}
		options := minio.PutObjectOptions{ContentType: referenceContentType, UserMetadata: map[string]string{"renop-reference": "1", "renop-sha256": digest, "renop-size": strconv.FormatInt(size, 10)}}
		if prior != nil {
			options.SetMatchETag(prior.ETag)
			options.UserMetadata["renop-modtime"] = strconv.FormatInt(prior.LastModified.UnixNano(), 10)
		}
		var body [s3ReferenceBodySize]byte
		copy(body[:], "renop-content-v1")
		raw, _ := hex.DecodeString(digest)
		copy(body[20:52], raw)
		binary.BigEndian.PutUint64(body[52:60], uint64(size))
		if s.Index != nil {
			s.Index.Remove(key)
		}
		result, err := s.Client.PutObject(ctx, s.Bucket, key, bytes.NewReader(body[:]), int64(len(body)), options)
		if err == nil {
			if s.Index != nil {
				modified := result.LastModified
				if modified.IsZero() {
					modified = time.Now().UTC()
				}
				if prior != nil {
					modified = prior.LastModified
				}
				s.Index.Put(key, S3Record{Digest: digest, ETag: result.ETag, Size: size, ObjectSize: s3ReferenceBodySize, ModTime: modified.UnixNano(), Reference: true, PreservedTime: prior != nil, Verified: true})
			}
			_ = s.Client.RemoveObject(ctx, s.Bucket, strings.TrimSuffix(blob, "data")+"gc", minio.RemoveObjectOptions{})
		}
		return err
	}()
	if err == nil && old != "" && old != digest {
		s.removeMarker(ctx, old, key)
	}
	return err
}

// PutStream hashes during upload to remote staging, then promotes the payload
// with server-side copy. No archive is buffered in memory or downloaded locally.
func (s S3) PutStream(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if size >= 0 && size < S3DedupMinimumSize {
		return s.PutRaw(ctx, key, reader, size, contentType)
	}
	temporary := strings.TrimSuffix(s.Prefix, "/") + "/staging/" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + uuid.NewString()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = s.Client.RemoveObject(cleanup, s.Bucket, temporary, minio.RemoveObjectOptions{})
	}()
	hasher := sha256.New()
	counter := &s3CountingReader{Reader: io.TeeReader(reader, hasher)}
	if _, err := s.Client.PutObject(ctx, s.Bucket, temporary, counter, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return err
	}
	if size >= 0 && counter.size != size {
		return errors.New("stream size changed during upload")
	}
	size = counter.size
	if size < S3DedupMinimumSize {
		object, err := s.Client.GetObject(ctx, s.Bucket, temporary, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		defer object.Close()
		return s.PutRaw(ctx, key, object, size, contentType)
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	return s.install(ctx, key, digest, size, nil, func(blob string) error {
		destination := minio.CopyDestOptions{Bucket: s.Bucket, Object: blob}
		source := minio.CopySrcOptions{Bucket: s.Bucket, Object: temporary}
		if size <= 5<<30 {
			_, err := s.Client.CopyObject(ctx, destination, source)
			return err
		}
		_, err := s.Client.ComposeObject(ctx, destination, source)
		return err
	})
}

type s3CountingReader struct {
	io.Reader
	size int64
}

func (r *s3CountingReader) Read(data []byte) (int, error) {
	n, err := r.Reader.Read(data)
	r.size += int64(n)
	return n, err
}

// PutRaw retains streaming for small control files.
// Replacing a reference releases its old marker only after a durable replacement.
func (s S3) PutRaw(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	unlock := s.lock(key, false)
	defer unlock()
	old, err := s.oldDigest(ctx, key)
	if err != nil {
		return err
	}
	if s.Index != nil {
		s.Index.Remove(key)
	}
	result, err := s.Client.PutObject(ctx, s.Bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err == nil && s.Index != nil {
		modified := result.LastModified
		if modified.IsZero() {
			modified = time.Now().UTC()
		}
		s.Index.Put(key, S3Record{ETag: result.ETag, Size: result.Size, ObjectSize: result.Size, ModTime: modified.UnixNano(), Verified: true})
	}
	if err == nil && old != "" {
		s.removeMarker(ctx, old, key)
	}
	return err
}

func (s S3) oldDigest(ctx context.Context, key string) (string, error) {
	info, err := s.stat(ctx, key)
	if missingS3(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	digest, _, _, err := Reference(info)
	return digest, err
}

func (s S3) removeMarker(ctx context.Context, digest, key string) {
	unlock := s.lock(digest, true)
	defer unlock()
	_ = s.Client.RemoveObject(ctx, s.Bucket, s.marker(digest, key), minio.RemoveObjectOptions{})
}

func (s S3) Delete(ctx context.Context, key string) error {
	unlock := s.lock(key, false)
	defer unlock()
	old, err := s.oldDigest(ctx, key)
	if err != nil {
		return err
	}
	if s.Index != nil {
		s.Index.Remove(key)
	}
	if err := s.Client.RemoveObject(ctx, s.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return err
	}
	if old != "" {
		s.removeMarker(ctx, old, key)
	}
	return nil
}

func missingS3(err error) bool {
	if err == nil {
		return false
	}
	response := minio.ToErrorResponse(err)
	return response.StatusCode == 404 || response.Code == "NoSuchKey" || response.Code == "NoSuchObject"
}

// Collect repairs stale markers and retires unreferenced payloads after a full
// grace period. Callbacks cap work per pass; payload readers have 30-minute leases.
func (s S3) Collect(ctx context.Context, after string, limit int) (string, error) {
	if s.Index != nil {
		for _, candidate := range s.Index.Garbage(limit, time.Now().Add(-s3CollectionGrace).Unix()) {
			if s.Index.Has(candidate.Digest) {
				continue
			}
			if err := s.collectBlobWithGrace(ctx, candidate.Digest, time.Now(), false); err != nil {
				return "", err
			}
			s.Index.ForgetGarbage(candidate.Digest, candidate.Since)
		}
		return "", nil
	}
	return s.collect(ctx, after, limit, false)
}

// CollectUnreferenced is used after the last repository using this namespace is
// deleted. Live references still protect data; deleted files need no read grace.
func (s S3) CollectUnreferenced(ctx context.Context, after string, limit int) (string, error) {
	return s.collect(ctx, after, limit, true)
}

func (s S3) collect(ctx context.Context, after string, limit int, immediate bool) (string, error) {
	prefix := strings.TrimSuffix(s.Prefix, "/") + "/"
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	visited := 0
	for object := range s.Client.ListObjects(ctx, s.Bucket, minio.ListObjectsOptions{Prefix: prefix, StartAfter: after, Recursive: false}) {
		if object.Err != nil {
			return after, object.Err
		}
		if visited >= limit {
			return after, nil
		}
		visited++
		after = object.Key
		if object.Key == prefix+"staging/" {
			if err := s.collectStaging(ctx, object.Key); err != nil {
				return after, err
			}
			continue
		}
		digest := strings.TrimSuffix(strings.TrimPrefix(object.Key, prefix), "/")
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != sha256.Size {
			continue
		}
		if err := s.collectBlobWithGrace(ctx, digest, time.Now(), !immediate); err != nil {
			return after, err
		}
	}
	return "", ctx.Err()
}

func (s S3) collectStaging(ctx context.Context, prefix string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	visited := 0
	for object := range s.Client.ListObjects(ctx, s.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return object.Err
		}
		if visited >= 16 {
			return nil
		}
		visited++
		if time.Since(object.LastModified) < s3CollectionGrace {
			return nil
		}
		if err := s.Client.RemoveObject(ctx, s.Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (s S3) CleanupStaging(ctx context.Context) error {
	return s.collectStaging(ctx, strings.TrimSuffix(s.Prefix, "/")+"/staging/")
}

func (s S3) collectBlob(ctx context.Context, digest string, now time.Time) error {
	return s.collectBlobWithGrace(ctx, digest, now, true)
}

func (s S3) collectBlobWithGrace(ctx context.Context, digest string, now time.Time, grace bool) error {
	if !validS3Digest(digest) {
		return errors.New("invalid S3 content digest")
	}
	unlock := s.lock(digest, true)
	defer unlock()
	root := strings.TrimSuffix(s.blob(digest), "data")
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	visited := 0
	for marker := range s.Client.ListObjects(listCtx, s.Bucket, minio.ListObjectsOptions{Prefix: root + "refs/", Recursive: true}) {
		if marker.Err != nil {
			return marker.Err
		}
		visited++
		if visited > 10000 {
			return nil
		}
		if marker.Size < 1 || marker.Size > 4096 {
			return errors.New("invalid S3 reference marker")
		}
		reader, err := s.Client.GetObject(ctx, s.Bucket, marker.Key, minio.GetObjectOptions{})
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(reader, 4097))
		reader.Close()
		if err != nil {
			return err
		}
		if len(data) > 4096 || s.marker(digest, string(data)) != marker.Key {
			return errors.New("invalid S3 reference marker")
		}
		info, err := s.Client.StatObject(ctx, s.Bucket, string(data), minio.StatObjectOptions{})
		current := ""
		if missingS3(err) {
			if s.Index != nil {
				s.Index.Remove(string(data))
			}
		} else if err != nil {
			return err
		} else {
			current, _, _, err = Reference(info)
			if err != nil {
				return err
			}
			if err := s.remember(string(data), info); err != nil {
				return err
			}
		}
		if current == digest {
			_ = s.Client.RemoveObject(ctx, s.Bucket, root+"gc", minio.RemoveObjectOptions{})
			return nil
		}
		if err := s.Client.RemoveObject(ctx, s.Bucket, marker.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	if grace {
		mark, err := s.Client.StatObject(ctx, s.Bucket, root+"gc", minio.StatObjectOptions{})
		if missingS3(err) {
			_, err = s.Client.PutObject(ctx, s.Bucket, root+"gc", bytes.NewReader(nil), 0, minio.PutObjectOptions{})
			return err
		}
		if err != nil {
			return err
		}
		if now.Sub(mark.LastModified) < s3CollectionGrace {
			return nil
		}
	}
	if err := s.Client.RemoveObject(ctx, s.Bucket, root+"data", minio.RemoveObjectOptions{}); err != nil {
		return err
	}
	return s.Client.RemoveObject(ctx, s.Bucket, root+"gc", minio.RemoveObjectOptions{})
}

func validS3Digest(digest string) bool {
	raw, err := hex.DecodeString(digest)
	return err == nil && len(raw) == sha256.Size && digest == strings.ToLower(digest)
}
