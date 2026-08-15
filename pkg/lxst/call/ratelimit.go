// SPDX-License-Identifier: Apache-2.0
package call

import (
	"sync"
	"time"
)

const (
	rateLimitMaxBuckets = 4096
)

// RateLimiter is a per-identity token bucket for incoming calls.
type RateLimiter struct {
	mutex    sync.Mutex
	interval time.Duration
	burst    int
	buckets  map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(interval time.Duration, burst int) *RateLimiter {
	if interval <= 0 {
		interval = DefaultRateInterval
	}
	if burst <= 0 {
		burst = DefaultRateBurst
	}
	return &RateLimiter{interval: interval, burst: burst, buckets: map[string]*bucket{}}
}

func (r *RateLimiter) Allow(hash []byte) bool {
	if r == nil {
		return true
	}
	key := string(hash)
	now := time.Now()
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if len(r.buckets) >= rateLimitMaxBuckets {
		r.pruneLocked(now)
	}
	b := r.buckets[key]
	if b == nil {
		if len(r.buckets) >= rateLimitMaxBuckets {
			r.pruneLocked(now)
		}
		if len(r.buckets) >= rateLimitMaxBuckets {
			return false
		}
		b = &bucket{tokens: float64(r.burst), last: now}
		r.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds() / r.interval.Seconds()
	b.tokens += elapsed
	if b.tokens > float64(r.burst) {
		b.tokens = float64(r.burst)
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (r *RateLimiter) pruneLocked(now time.Time) {
	stale := max(r.interval*2, time.Minute)
	for k, b := range r.buckets {
		if now.Sub(b.last) > stale {
			delete(r.buckets, k)
		}
	}
	if len(r.buckets) < rateLimitMaxBuckets {
		return
	}
	n := 0
	limit := rateLimitMaxBuckets / 2
	for k := range r.buckets {
		delete(r.buckets, k)
		n++
		if n >= limit {
			return
		}
	}
}
