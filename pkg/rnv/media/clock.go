// SPDX-License-Identifier: 0BSD
package media

import (
	"sync"
	"time"
)

// Clock is a best-effort A/V playout skew helper.
// It does not guarantee lip-sync on lossy Reticulum links.
type Clock struct {
	mu        sync.Mutex
	origin    time.Time
	videoSkew time.Duration
	audioSkew time.Duration
}

// NoteVideo records video frame arrival for skew estimates.
func (c *Clock) NoteVideo(seq uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.origin.IsZero() {
		c.origin = time.Now()
	}
	_ = seq
	c.videoSkew = time.Since(c.origin)
}

// NoteAudio records audio frame arrival for skew estimates.
func (c *Clock) NoteAudio(seq uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.origin.IsZero() {
		c.origin = time.Now()
	}
	_ = seq
	c.audioSkew = time.Since(c.origin)
}

// Skew returns audio minus video playout estimate.
func (c *Clock) Skew() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.audioSkew - c.videoSkew
}
