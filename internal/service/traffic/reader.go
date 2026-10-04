/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package traffic

import (
	"errors"
	"io"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ErrStreamTimeout is returned when a download connection is unresponsive for over 30 seconds.
var ErrStreamTimeout = errors.New("download stream timeout: inactive for over 30 seconds")

// DownloadSession maintains tracking information for one in-flight download stream.
type DownloadSession struct {
	Manager        *TrafficManager
	Subject        string
	FileKey        string
	FileSize       int64
	RequestedSize  int64
	DeductedAmount int64
	Multiplier     float64
	IsMultiThread  bool
	IsS3Redirect   bool

	bytesRead    atomic.Int64
	lastActivity atomic.Int64
	closeOnce    sync.Once
}

// WrapReadCloser wraps an io.ReadCloser with streaming accounting, inactivity timeouts, and refund logic.
func (s *DownloadSession) WrapReadCloser(rc io.ReadCloser) io.ReadCloser {
	if s == nil || s.IsS3Redirect || s.DeductedAmount == 0 {
		return rc
	}
	s.lastActivity.Store(time.Now().UnixMilli())
	return &trackingReadCloser{
		session: s,
		inner:   rc,
	}
}

type trackingReadCloser struct {
	session *DownloadSession
	inner   io.ReadCloser
}

func (t *trackingReadCloser) Read(p []byte) (int, error) {
	now := time.Now().UnixMilli()
	last := t.session.lastActivity.Load()
	if last > 0 && now-last > 30000 {
		return 0, ErrStreamTimeout
	}

	n, err := t.inner.Read(p)
	if n > 0 {
		t.session.bytesRead.Add(int64(n))
		t.session.lastActivity.Store(time.Now().UnixMilli())
	}
	return n, err
}

func (t *trackingReadCloser) Close() error {
	var err error
	t.session.closeOnce.Do(func() {
		if t.inner != nil {
			err = t.inner.Close()
		}
		t.session.onStreamFinish()
	})
	return err
}

func (s *DownloadSession) onStreamFinish() {
	if s.Manager == nil {
		return
	}
	// Decrement active downloads tracking
	s.Manager.decrementActiveDownload(s.Subject, s.FileKey)

	read := s.bytesRead.Load()
	// If completed full requested size, no refund
	if read >= s.RequestedSize {
		return
	}

	// Incomplete download: check if bytes read <= 70% of total file size
	threshold := int64(math.Floor(0.70 * float64(s.FileSize)))
	if read <= threshold {
		usedQuota := int64(math.Ceil(float64(read) * s.Multiplier))
		unusedQuota := s.DeductedAmount - usedQuota
		if unusedQuota > 0 {
			refundBytes := int64(math.Floor(0.80 * float64(unusedQuota)))
			if refundBytes > 0 {
				s.Manager.Refund(s.Subject, refundBytes)
			}
			// Mark breakpoint resumption as revoked: client must restart download from byte 0
			s.Manager.RevokeBreakpointResume(s.Subject, s.FileKey)
		}
	}
}
