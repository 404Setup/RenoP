/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package repositorycapacity

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConcurrentReservationsNeverExceedCapacity(t *testing.T) {
	m := New()
	var loads, committed atomic.Int64
	load := func(context.Context) (int64, error) { loads.Add(1); return 0, nil }
	stat := func(string) (int64, error) { return 0, nil }
	var workers sync.WaitGroup
	for i := range 20 {
		workers.Go(func() {
			r, err := m.Reserve(context.Background(), "repo", 100, []Object{{Path: fmt.Sprint(i), Size: 10}}, load, stat)
			if errors.Is(err, ErrExceeded) {
				return
			}
			if err != nil {
				t.Error(err)
				return
			}
			defer r.Release()
			if committed.Add(10) > 100 {
				t.Error("overcommitted repository capacity")
			}
			r.Commit()
		})
	}
	workers.Wait()
	require.EqualValues(t, 100, committed.Load())
	require.EqualValues(t, 1, loads.Load())
	other, err := m.Reserve(context.Background(), "other", 10, []Object{{Path: "a", Size: 10}}, load, stat)
	require.NoError(t, err)
	other.Commit()
}

func TestReplacementFailureAndDeletionRemeasureStorage(t *testing.T) {
	m := New()
	used, old := int64(10), int64(10)
	load := func(context.Context) (int64, error) { return used, nil }
	stat := func(path string) (int64, error) {
		if path == "old" {
			return old, nil
		}
		return 0, nil
	}
	r, err := m.Reserve(context.Background(), "repo", 10, []Object{{Path: "old", Size: 6}}, load, stat)
	require.NoError(t, err)
	used, old = 6, 6
	r.Commit()
	r, err = m.Reserve(context.Background(), "repo", 10, []Object{{Path: "new", Size: 4}}, load, stat)
	require.NoError(t, err)
	// A failed operation can still have installed bytes before rollback failed.
	used = 10
	r.Release()
	_, err = m.Reserve(context.Background(), "repo", 10, []Object{{Path: "third", Size: 1}}, load, stat)
	require.ErrorIs(t, err, ErrExceeded)
	used, old = 4, 0
	m.Invalidate("repo")
	r, err = m.Reserve(context.Background(), "repo", 10, []Object{{Path: "third", Size: 6}}, load, stat)
	require.NoError(t, err)
	r.Commit()
	// Lowering a limit preserves installed files and still permits size reductions.
	r, err = m.Reserve(context.Background(), "repo", 1, []Object{{Path: "empty", Size: 0}}, load, stat)
	require.NoError(t, err)
	r.Commit()
}

func TestOverlappingWritesAreCancellableAndInvalidLoadsFailClosed(t *testing.T) {
	m := New()
	load := func(context.Context) (int64, error) { return 0, nil }
	stat := func(string) (int64, error) { return 0, nil }
	r, err := m.Reserve(context.Background(), "repo", 10, []Object{{Path: "same", Size: 5}}, load, stat)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = m.Reserve(ctx, "repo", 10, []Object{{Path: "same", Size: 5}}, load, stat)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	r.Release()
	r, err = m.Reserve(context.Background(), "repo", 10, []Object{{Path: "same", Size: 5}}, load, stat)
	require.NoError(t, err)
	r.Release()
	m.Forget("repo")
	_, err = m.Reserve(context.Background(), "repo", 10, []Object{{Path: "same", Size: 5}},
		func(context.Context) (int64, error) { return 0, errors.New("storage unavailable") }, stat)
	require.ErrorContains(t, err, "storage unavailable")
	_, err = m.Reserve(context.Background(), "other", 10, []Object{{Path: "same", Size: 5}, {Path: "same", Size: 1}}, load, stat)
	require.ErrorIs(t, err, ErrInvalid)
}
