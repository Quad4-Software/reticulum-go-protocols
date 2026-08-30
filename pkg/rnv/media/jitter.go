// SPDX-License-Identifier: 0BSD
package media

import (
	"sync"
	"time"
)

const (
	DefaultTargetMs  = 80
	DefaultMaxFrames = 12
)

// Frame is a buffered media unit.
type Frame struct {
	Sequence uint16
	Payload  []byte
	Arrival  time.Time
}

// JitterBuffer reorders droppable stream frames per track.
type JitterBuffer struct {
	mu        sync.Mutex
	targetMs  int
	maxFrames int
	frames    map[uint16]Frame
	nextSeq   uint16
	started   bool
	recvCount uint64
	dropCount uint64
}

// NewJitterBuffer creates a per-track jitter buffer.
func NewJitterBuffer(targetMs, maxFrames int) *JitterBuffer {
	if targetMs <= 0 {
		targetMs = DefaultTargetMs
	}
	if maxFrames <= 0 {
		maxFrames = DefaultMaxFrames
	}
	return &JitterBuffer{
		targetMs:  targetMs,
		maxFrames: maxFrames,
		frames:    make(map[uint16]Frame),
	}
}

// PushOwned stores payload without copying.
func (j *JitterBuffer) PushOwned(seq uint16, payload []byte) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.recvCount++
	if j.started && seqLess(seq, j.nextSeq) {
		j.dropCount++
		return
	}
	j.frames[seq] = Frame{Sequence: seq, Payload: payload, Arrival: time.Now()}
	if len(j.frames) > j.maxFrames {
		j.dropOldestLocked()
	}
}

// PopReady returns the next in-order frame after the target delay.
func (j *JitterBuffer) PopReady(now time.Time) (Frame, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.frames) == 0 {
		return Frame{}, false
	}
	if !j.started {
		var earliest uint16
		var found bool
		for seq, fr := range j.frames {
			if !found || seqLess(seq, earliest) {
				earliest = seq
				found = true
				_ = fr
			}
		}
		fr := j.frames[earliest]
		if now.Sub(fr.Arrival) < time.Duration(j.targetMs)*time.Millisecond {
			return Frame{}, false
		}
		j.started = true
		j.nextSeq = earliest
	}
	fr, ok := j.frames[j.nextSeq]
	if !ok {
		return Frame{}, false
	}
	if now.Sub(fr.Arrival) < time.Duration(j.targetMs)*time.Millisecond {
		return Frame{}, false
	}
	delete(j.frames, j.nextSeq)
	out := fr
	j.nextSeq++
	return out, true
}

// Stats returns recv and drop counts.
func (j *JitterBuffer) Stats() (recv, drop uint64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.recvCount, j.dropCount
}

func (j *JitterBuffer) dropOldestLocked() {
	var oldest uint16
	var found bool
	for seq := range j.frames {
		if !found || seqLess(seq, oldest) {
			oldest = seq
			found = true
		}
	}
	if found {
		delete(j.frames, oldest)
		j.dropCount++
	}
}

func seqLess(a, b uint16) bool {
	return int16(a-b) < 0
}
