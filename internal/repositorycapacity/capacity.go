/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package repositorycapacity bounds committed repository bytes while allowing independent writes to proceed concurrently.
package repositorycapacity

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

var (
	ErrExceeded = errors.New("repository capacity exceeded")
	ErrInvalid  = errors.New("repository capacity request is invalid")
)

// Object describes the final size of one committed artifact, including generated companions.
type Object struct {
	Path string
	Size int64
}

type bucket struct {
	mu          sync.Mutex
	changed     chan struct{}
	paths       map[string]bool
	active      int
	used        int64
	reserved    int64
	epoch       uint64
	initialized bool
	loading     bool
	loadErr     error
	retryAt     time.Time
}

// Manager keeps bounded per-repository counters and only the paths of in-flight writes.
// The loader measures installed storage once, after restart or an explicit invalidation.
type Manager struct {
	mu     sync.Mutex
	scopes map[string]*bucket
}

// New creates an empty manager without performing I/O.
func New() *Manager { return &Manager{scopes: make(map[string]*bucket)} }

func (m *Manager) scope(name string) *bucket {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scopes == nil {
		m.scopes = make(map[string]*bucket)
	}
	b := m.scopes[name]
	if b == nil {
		b = &bucket{changed: make(chan struct{}), paths: make(map[string]bool)}
		m.scopes[name] = b
	}
	return b
}

func (b *bucket) notify() { close(b.changed); b.changed = make(chan struct{}) }

func wait(ctx context.Context, changed <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-changed:
		return nil
	}
}

func (b *bucket) ready(ctx context.Context, load func(context.Context) (int64, error)) error {
	for {
		b.mu.Lock()
		if b.initialized {
			b.mu.Unlock()
			return nil
		}
		if b.loading || b.active > 0 {
			changed := b.changed
			b.mu.Unlock()
			if err := wait(ctx, changed); err != nil {
				return err
			}
			continue
		}
		if b.loadErr != nil && time.Now().Before(b.retryAt) {
			err := b.loadErr
			b.mu.Unlock()
			return err
		}
		b.loading = true
		epoch := b.epoch
		b.mu.Unlock()
		used, err := load(ctx)
		if err == nil && used < 0 {
			err = ErrInvalid
		}
		b.mu.Lock()
		b.loading = false
		if err == nil && b.epoch == epoch {
			b.used, b.initialized, b.loadErr = used, true, nil
		} else if err != nil {
			b.loadErr, b.retryAt = err, time.Now().Add(time.Second)
		}
		b.notify()
		b.mu.Unlock()
		if err != nil {
			return err
		}
	}
}

// Reservation protects capacity and serializes only overlapping artifact paths.
type Reservation struct {
	once     sync.Once
	bucket   *bucket
	objects  []Object
	delta    int64
	reserved int64
}

// Reserve checks actual previous sizes and reserves every positive delta before storage changes.
// load and stat run outside locks. A non-positive limit uses the original unmetered path.
func (m *Manager) Reserve(ctx context.Context, scope string, limit int64, objects []Object,
	load func(context.Context) (int64, error), stat func(string) (int64, error)) (*Reservation, error) {
	if limit <= 0 {
		return nil, nil
	}
	if m == nil || scope == "" || len(objects) == 0 || len(objects) > 64 || load == nil || stat == nil {
		return nil, ErrInvalid
	}
	objects = append([]Object(nil), objects...)
	seen := make(map[string]bool, len(objects))
	for _, object := range objects {
		if object.Path == "" || object.Size < 0 || seen[object.Path] {
			return nil, ErrInvalid
		}
		seen[object.Path] = true
	}
	b := m.scope(scope)
	for {
		if err := b.ready(ctx, load); err != nil {
			return nil, err
		}
		b.mu.Lock()
		if !b.initialized {
			b.mu.Unlock()
			continue
		}
		busy := false
		for _, object := range objects {
			busy = busy || b.paths[object.Path]
		}
		if busy {
			changed := b.changed
			b.mu.Unlock()
			if err := wait(ctx, changed); err != nil {
				return nil, err
			}
			continue
		}
		epoch := b.epoch
		for _, object := range objects {
			b.paths[object.Path] = true
		}
		b.active++
		b.mu.Unlock()
		reservation := &Reservation{bucket: b, objects: objects}
		var statErr error
		var previousTotal int64
		for _, object := range objects {
			previous, err := stat(object.Path)
			if err != nil || previous < 0 {
				statErr = err
				if statErr == nil {
					statErr = ErrInvalid
				}
				break
			}
			if previousTotal > math.MaxInt64-previous {
				statErr = ErrInvalid
				break
			}
			previousTotal += previous
			delta := object.Size - previous
			if delta > 0 && reservation.reserved > math.MaxInt64-delta ||
				delta > 0 && reservation.delta > math.MaxInt64-delta ||
				delta < 0 && reservation.delta < math.MinInt64-delta {
				statErr = ErrInvalid
				break
			}
			reservation.delta += delta
			reservation.reserved += max(delta, 0)
		}
		if statErr == nil {
			statErr = ctx.Err()
		}
		b.mu.Lock()
		if statErr == nil && previousTotal > b.used {
			b.initialized = false
			b.epoch++
		}
		if statErr != nil || !b.initialized || b.epoch != epoch ||
			reservation.reserved > 0 && (b.used > limit || b.reserved > limit-b.used || reservation.reserved > limit-b.used-b.reserved) {
			for _, object := range objects {
				delete(b.paths, object.Path)
			}
			b.active--
			b.notify()
			retry := !b.initialized || b.epoch != epoch
			b.mu.Unlock()
			if statErr != nil {
				return nil, statErr
			}
			if retry {
				continue
			}
			return nil, ErrExceeded
		}
		b.reserved += reservation.reserved
		b.mu.Unlock()
		return reservation, nil
	}
}

// Commit records the successful final sizes and releases the reservation exactly once.
func (r *Reservation) Commit() { r.finish(true) }

// Release abandons a reservation and remeasures storage before another admission.
// Remeasuring also covers partial writes and cleanup failures; installed data is never underestimated.
func (r *Reservation) Release() { r.finish(false) }

func (r *Reservation) finish(success bool) {
	if r == nil {
		return
	}
	r.once.Do(func() {
		b := r.bucket
		b.mu.Lock()
		defer b.mu.Unlock()
		b.reserved -= r.reserved
		b.active--
		for _, object := range r.objects {
			delete(b.paths, object.Path)
		}
		if success {
			b.used += r.delta
		} else {
			b.initialized = false
			b.epoch++
		}
		b.notify()
	})
}

// Invalidate remeasures a repository after deletion, index rebuilding, or backend reconfiguration.
// An empty scope invalidates all repositories.
func (m *Manager) Invalidate(scope string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, b := range m.scopes {
		if scope != "" && name != scope {
			continue
		}
		b.mu.Lock()
		b.initialized, b.loadErr = false, nil
		b.epoch++
		b.notify()
		b.mu.Unlock()
	}
}

// Forget drops an unused repository counter after its configuration has been removed.
func (m *Manager) Forget(scope string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if b := m.scopes[scope]; b != nil {
		b.mu.Lock()
		if b.active == 0 && !b.loading {
			delete(m.scopes, scope)
		}
		b.mu.Unlock()
	}
}
