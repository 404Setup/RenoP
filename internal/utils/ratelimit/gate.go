/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

// Package ratelimit provides bounded admission gates for expensive request classes.
package ratelimit

import (
	"math"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type entry struct {
	limiter *rate.Limiter
	seen    time.Time
}

// Gate retains a bounded set of token buckets and shares an overflow budget.
type Gate struct {
	mu       sync.Mutex
	entries  map[string]*entry
	overflow *rate.Limiter
	limit    rate.Limit
	burst    int
	capacity int
	idle     time.Duration
	pruneAt  time.Time
}

// New creates a gate with positive refill, burst, and retained-key limits.
func New(limit rate.Limit, burst, capacity int) *Gate {
	if limit <= 0 || burst < 1 || capacity < 1 {
		panic("invalid request rate limit")
	}
	return &Gate{
		entries: make(map[string]*entry), overflow: rate.NewLimiter(limit, burst),
		limit: limit, burst: burst, capacity: capacity,
		idle: max(5*time.Minute, time.Duration(math.Ceil(float64(burst)/float64(limit)*float64(time.Second)))),
	}
}

// Allow returns zero when admitted, otherwise the minimum delay before retrying.
// Denied requests do not reserve future tokens or extend the penalty.
func (g *Gate) Allow(key string) time.Duration { return g.allowAt(key, time.Now()) }

func (g *Gate) allowAt(key string, now time.Time) time.Duration {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !now.Before(g.pruneAt) {
		for key, entry := range g.entries {
			if now.Sub(entry.seen) >= g.idle {
				delete(g.entries, key)
			}
		}
		g.pruneAt = now.Add(time.Minute)
	}
	limiter := g.overflow
	if current := g.entries[key]; current != nil {
		current.seen = now
		limiter = current.limiter
	} else if len(g.entries) < g.capacity {
		limiter = rate.NewLimiter(g.limit, g.burst)
		g.entries[key] = &entry{limiter: limiter, seen: now}
	}
	if limiter.AllowN(now, 1) {
		return 0
	}
	return max(time.Millisecond, time.Duration(math.Ceil((1-limiter.TokensAt(now))/float64(g.limit)*float64(time.Second))))
}

// NetworkKey normalizes mapped IPv4 and groups IPv6 privacy addresses by /64.
func NetworkKey(ip string) string {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return "unknown"
	}
	address = address.Unmap().WithZone("")
	if address.Is6() {
		return netip.PrefixFrom(address, 64).Masked().String()
	}
	return address.String()
}
