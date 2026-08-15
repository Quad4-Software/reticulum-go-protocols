// SPDX-License-Identifier: Apache-2.0
package media

import (
	"sync"
	"time"
)

const (
	DefaultTargetMs  = 60
	DefaultMaxFrames = 16
)

type Frame struct {
	Sequence  uint16
	Timestamp uint32
	Payload   []byte
	Arrival   time.Time
}

// JitterBuffer reorders and holds incoming media frames.
type JitterBuffer struct {
	mutex     sync.Mutex
	targetMs  int
	maxFrames int
	frames    map[uint16]Frame
	nextSeq   uint16
	started   bool
	lastPop   time.Time
	lossCount uint64
	recvCount uint64
}

func NewJitterBuffer(targetMs int, maxFrames int) *JitterBuffer {
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

func (j *JitterBuffer) SetTargetMs(ms int) {
	j.mutex.Lock()
	j.targetMs = ms
	j.mutex.Unlock()
}

func (j *JitterBuffer) TargetMs() int {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return j.targetMs
}

func (j *JitterBuffer) Push(seq uint16, timestamp uint32, payload []byte) {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	j.recvCount++
	if j.started && seqLess(seq, j.nextSeq) {
		return
	}
	j.frames[seq] = Frame{
		Sequence:  seq,
		Timestamp: timestamp,
		Payload:   append([]byte(nil), payload...),
		Arrival:   time.Now(),
	}
	if len(j.frames) > j.maxFrames {
		j.dropOldestLocked()
	}
}

func (j *JitterBuffer) PopReady(now time.Time) (Frame, bool) {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	if len(j.frames) == 0 {
		return Frame{}, false
	}
	if !j.started {
		j.started = true
		j.nextSeq = j.lowestSeqLocked()
	}
	frame, ok := j.frames[j.nextSeq]
	if !ok {
		if j.shouldSkipMissingLocked(now) {
			j.lossCount++
			delete(j.frames, j.nextSeq)
			j.nextSeq++
			return Frame{}, false
		}
		return Frame{}, false
	}
	delete(j.frames, j.nextSeq)
	j.nextSeq++
	j.lastPop = now
	return frame, true
}

func (j *JitterBuffer) LossRate() float64 {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	if j.recvCount == 0 {
		return 0
	}
	return float64(j.lossCount) / float64(j.recvCount)
}

func (j *JitterBuffer) Depth() int {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return len(j.frames)
}

func (j *JitterBuffer) shouldSkipMissingLocked(now time.Time) bool {
	wait := time.Duration(j.targetMs) * time.Millisecond / 2
	if j.lastPop.IsZero() {
		return len(j.frames) > 0
	}
	return now.Sub(j.lastPop) > wait
}

func (j *JitterBuffer) dropOldestLocked() {
	if len(j.frames) == 0 {
		return
	}
	oldest := j.lowestSeqLocked()
	delete(j.frames, oldest)
}

func (j *JitterBuffer) lowestSeqLocked() uint16 {
	var lowest uint16
	first := true
	for seq := range j.frames {
		if first || seqLess(seq, lowest) {
			lowest = seq
			first = false
		}
	}
	return lowest
}

func seqLess(a, b uint16) bool {
	return int16(a-b) < 0 // #nosec G115 -- RFC 1982 serial comparison
}
