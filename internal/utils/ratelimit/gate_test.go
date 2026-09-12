/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package ratelimit

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestGateConcurrentAdmissionAndRetry(t *testing.T) {
	gate, now := New(rate.Every(time.Second), 3, 8), time.Now()
	var accepted atomic.Int32
	var workers sync.WaitGroup
	for range 30 {
		workers.Go(func() {
			if gate.allowAt("account", now) == 0 {
				accepted.Add(1)
			}
		})
	}
	workers.Wait()
	if accepted.Load() != 3 {
		t.Fatalf("admitted %d requests, want 3", accepted.Load())
	}
	for range 10 {
		if delay := gate.allowAt("account", now); delay != time.Second {
			t.Fatalf("retry delay = %s, want 1s", delay)
		}
	}
	if delay := gate.allowAt("account", now.Add(time.Second)); delay != 0 {
		t.Fatalf("denials consumed future allowance: %s", delay)
	}
}

func TestGateCapacityAndIdleRecovery(t *testing.T) {
	gate, now := New(rate.Every(time.Hour), 1, 1), time.Now()
	if gate.allowAt("first", now) != 0 || gate.allowAt("overflow", now) != 0 {
		t.Fatal("initial allowances denied")
	}
	if gate.allowAt("another", now) == 0 || len(gate.entries) != 1 {
		t.Fatal("rotating keys bypassed the bounded overflow budget")
	}
	if gate.allowAt("first", now.Add(10*time.Minute)) == 0 {
		t.Fatal("idle eviction reset an unrefilled budget")
	}
	if gate.allowAt("fresh", now.Add(2*time.Hour)) != 0 || len(gate.entries) != 1 {
		t.Fatal("idle keys did not make room for a new client")
	}
}

func TestNetworkKey(t *testing.T) {
	for ip, want := range map[string]string{
		"192.0.2.1": "192.0.2.1", "::ffff:192.0.2.1": "192.0.2.1",
		"2001:db8:1::1": "2001:db8:1::/64", "2001:db8:1::ffff": "2001:db8:1::/64", "bad": "unknown",
	} {
		if got := NetworkKey(ip); got != want {
			t.Errorf("NetworkKey(%q) = %q, want %q", ip, got, want)
		}
	}
}
