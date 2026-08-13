// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"quad4/msgpack/v5/pkg/msgpack"
	"quad4/reticulum-go/pkg/cryptography"
)

// Workblock expansion rounds (aligned with LXStamper).
const (
	WorkblockExpandRounds        = 3000
	WorkblockExpandRoundsPN      = 1000
	WorkblockExpandRoundsPeering = 25

	StampSize = 32
)

// ErrStampNotFound means GenerateStamp ended before finding a stamp (e.g. cancelled context).
var ErrStampNotFound = errors.New("lxmf: stamp generation cancelled")

// StampWorkblock returns the HKDF-expanded workblock (256 * expandRounds bytes).
func StampWorkblock(material []byte, expandRounds int) ([]byte, error) {
	if expandRounds <= 0 {
		return nil, errors.New("lxmf: expandRounds must be positive")
	}
	if len(material) == 0 {
		return nil, errors.New("lxmf: workblock material required")
	}

	out := make([]byte, 0, 256*expandRounds)
	for n := range expandRounds {
		nBytes, err := msgpack.Marshal(n)
		if err != nil {
			return nil, fmt.Errorf("lxmf: workblock msgpack: %w", err)
		}
		saltSrc := make([]byte, 0, len(material)+len(nBytes))
		saltSrc = append(saltSrc, material...)
		saltSrc = append(saltSrc, nBytes...)
		saltSum := sha256.Sum256(saltSrc)
		block, err := cryptography.DeriveKey(material, saltSum[:], nil, 256)
		if err != nil {
			return nil, fmt.Errorf("lxmf: workblock hkdf: %w", err)
		}
		out = append(out, block...)
	}
	return out, nil
}

// StampValue returns the leading-zero-bit score of SHA256(workblock||stamp).
func StampValue(workblock, stamp []byte) int {
	if len(stamp) == 0 {
		return 0
	}
	buf := make([]byte, 0, len(workblock)+len(stamp))
	buf = append(buf, workblock...)
	buf = append(buf, stamp...)
	sum := sha256.Sum256(buf)

	value := 0
	for _, b := range sum {
		if b == 0 {
			value += 8
			continue
		}
		for bit := 7; bit >= 0; bit-- {
			if b&(1<<bit) != 0 {
				return value
			}
			value++
		}
		return value
	}
	return value
}

// StampValid reports whether the stamp satisfies targetCost against workblock.
func StampValid(stamp []byte, targetCost int, workblock []byte) bool {
	if targetCost <= 0 {
		return true
	}
	if len(stamp) != StampSize {
		return false
	}
	if targetCost > 256 {
		return false
	}
	target := stampTargetBytes(targetCost)
	buf := make([]byte, 0, len(workblock)+len(stamp))
	buf = append(buf, workblock...)
	buf = append(buf, stamp...)
	sum := sha256.Sum256(buf)
	return bytes.Compare(sum[:], target) <= 0
}

// ValidatePNStamp checks PN transient data (LXMF bytes + 32-byte stamp) and returns ids and stamp on success.
func ValidatePNStamp(transientData []byte, targetCost int) (transientID, lxmData []byte, value int, stamp []byte) {
	if len(transientData) <= Overhead+StampSize {
		return nil, nil, 0, nil
	}
	cut := len(transientData) - StampSize
	lxm := transientData[:cut]
	st := transientData[cut:]
	tidSum := sha256.Sum256(lxm)
	wb, err := StampWorkblock(tidSum[:], WorkblockExpandRoundsPN)
	if err != nil {
		return nil, nil, 0, nil
	}
	if !StampValid(st, targetCost, wb) {
		return nil, nil, 0, nil
	}
	return tidSum[:], append([]byte(nil), lxm...), StampValue(wb, st), append([]byte(nil), st...)
}

// ValidatePeeringKey checks peeringKey against targetCost using the peering workblock.
func ValidatePeeringKey(peeringID, peeringKey []byte, targetCost int) bool {
	wb, err := StampWorkblock(peeringID, WorkblockExpandRoundsPeering)
	if err != nil {
		return false
	}
	return StampValid(peeringKey, targetCost, wb)
}

// GenerateStamp searches for a stamp meeting stampCost; parallel workers respect ctx. ErrStampNotFound if cancelled.
func GenerateStamp(ctx context.Context, messageID []byte, stampCost, expandRounds int) ([]byte, int, error) {
	if stampCost <= 0 {
		return nil, 0, errors.New("lxmf: stampCost must be positive")
	}
	if expandRounds <= 0 {
		expandRounds = WorkblockExpandRounds
	}
	wb, err := StampWorkblock(messageID, expandRounds)
	if err != nil {
		return nil, 0, err
	}
	target := stampTargetBytes(stampCost)

	workers := max(runtime.NumCPU(), 1)

	type result struct {
		stamp []byte
	}

	resCh := make(chan result, 1)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Go(func() {
			candidate := make([]byte, StampSize)
			buf := make([]byte, 0, len(wb)+StampSize)
			for {
				select {
				case <-subCtx.Done():
					return
				default:
				}
				if _, err := rand.Read(candidate); err != nil {
					return
				}
				buf = append(buf[:0], wb...)
				buf = append(buf, candidate...)
				sum := sha256.Sum256(buf)
				if bytes.Compare(sum[:], target) <= 0 {
					stamp := append([]byte(nil), candidate...)
					select {
					case resCh <- result{stamp: stamp}:
						cancel()
					case <-subCtx.Done():
					}
					return
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(resCh)
	}()

	res, ok := <-resCh
	if !ok {
		return nil, 0, ErrStampNotFound
	}
	return res.stamp, StampValue(wb, res.stamp), nil
}

// GenerateStampWithDeadline wraps GenerateStamp with a deadline.
func GenerateStampWithDeadline(parent context.Context, messageID []byte, stampCost, expandRounds int, deadline time.Time) ([]byte, int, error) {
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	return GenerateStamp(ctx, messageID, stampCost, expandRounds)
}

func stampTargetBytes(cost int) []byte {
	target := make([]byte, 32)
	if cost >= 256 {
		return target
	}
	pos := 256 - cost
	byteIdx := 31 - (pos / 8)
	bitIdx := pos % 8
	if byteIdx < 0 {
		return target
	}
	target[byteIdx] = 1 << bitIdx
	return target
}
