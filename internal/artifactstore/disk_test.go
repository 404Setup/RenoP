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
	"os"
	"path/filepath"
	"sync"
	"testing"

	"renop/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestDiskExistingFilesAndCancelledScan(t *testing.T) {
	d := &Disk{}
	root := testutil.TempDir(t)
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	require.NoError(t, os.WriteFile(a, []byte("existing identical bytes"), 0600))
	require.NoError(t, os.WriteFile(b, []byte("existing identical bytes"), 0600))
	changed, err := d.ReuseExisting(context.Background(), a)
	require.NoError(t, err)
	require.False(t, changed)
	changed, err = d.ReuseExisting(context.Background(), b)
	require.NoError(t, err)
	require.True(t, changed)
	ai, err := os.Stat(a)
	require.NoError(t, err)
	bi, err := os.Stat(b)
	require.NoError(t, err)
	require.True(t, os.SameFile(ai, bi))
	changed, err = d.ReuseExisting(context.Background(), b)
	require.NoError(t, err)
	require.False(t, changed)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = d.ReuseExisting(ctx, a)
	require.ErrorIs(t, err, context.Canceled)
	data, err := os.ReadFile(a)
	require.NoError(t, err)
	require.Equal(t, "existing identical bytes", string(data))
}

func TestDiskDeduplicationReplacementAndDeletion(t *testing.T) {
	d := &Disk{}
	root := testutil.TempDir(t)
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	require.NoError(t, d.WriteFile(a, []byte("shared payload"), 0600))
	require.NoError(t, d.WriteFile(b, []byte("shared payload"), 0600))
	ai, err := os.Stat(a)
	require.NoError(t, err)
	bi, err := os.Stat(b)
	require.NoError(t, err)
	require.True(t, os.SameFile(ai, bi), "identical contents should share physical storage")
	require.NoError(t, d.WriteFile(a, []byte("replacement"), 0600))
	data, err := os.ReadFile(b)
	require.NoError(t, err)
	require.Equal(t, "shared payload", string(data))
	require.NoError(t, os.Remove(a))
	data, err = os.ReadFile(b)
	require.NoError(t, err)
	require.Equal(t, "shared payload", string(data))
}

func TestDiskStaleCandidateAndFailedCommit(t *testing.T) {
	d := &Disk{}
	root := testutil.TempDir(t)
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	require.NoError(t, d.WriteFile(a, []byte("original"), 0600))
	// Simulate an operator changing a candidate outside the artifact installer.
	require.NoError(t, os.WriteFile(a, []byte("changed!"), 0600))
	require.NoError(t, d.WriteFile(b, []byte("original"), 0600))
	data, err := os.ReadFile(b)
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
	staged := filepath.Join(root, "staged")
	require.NoError(t, os.WriteFile(staged, []byte("original"), 0600))
	blocked := filepath.Join(root, "directory")
	require.NoError(t, os.Mkdir(blocked, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(blocked, "child"), []byte("retained"), 0600))
	require.Error(t, d.Commit(staged, blocked))
	data, err = os.ReadFile(staged)
	require.NoError(t, err)
	require.Equal(t, "original", string(data))
	data, err = os.ReadFile(filepath.Join(blocked, "child"))
	require.NoError(t, err)
	require.Equal(t, "retained", string(data))
}

func TestDiskConcurrentIdenticalCommits(t *testing.T) {
	d := &Disk{}
	root := testutil.TempDir(t)
	var wg sync.WaitGroup
	paths := make([]string, 16)
	errs := make([]error, len(paths))
	for i := range paths {
		paths[i] = filepath.Join(root, string(rune('a'+i)))
		wg.Go(func() { errs[i] = d.WriteFile(paths[i], []byte("concurrent contents"), 0600) })
	}
	wg.Wait()
	first, err := os.Stat(paths[0])
	require.NoError(t, err)
	for i, path := range paths {
		require.NoError(t, errs[i])
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.True(t, os.SameFile(first, info))
	}
}
