// SPDX-License-Identifier: Apache-2.0
package media_test

import (
	"testing"
	"time"

	"quad4/pbt/pkg/pbt"
	"quad4/reticulum-go-protocols/pkg/lxst/media"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func TestPropertyInOrderPop(t *testing.T) {
	gen := pbt.IntRange(1, 32)
	pbt.Check(t, pbt.ForAll("in-order jitter pops sequential seqs", gen, func(n int) bool {
		jb := media.NewJitterBuffer(20, n+4)
		now := time.Now()
		for i := range n {
			jb.Push(uint16(i), uint32(i), []byte{byte(i)})
		}
		for i := range n {
			f, ok := jb.PopReady(now.Add(time.Duration(i) * time.Millisecond))
			if !ok || f.Sequence != uint16(i) {
				return false
			}
		}
		return true
	}), pbt.WithRuns(50))
}

func TestPropertyAdaptiveClamp(t *testing.T) {
	pbt.Check(t, pbt.ForAll("bitrate stays in proto range", pbt.IntRange(0, 5), func(n int) bool {
		ac := media.NewAdaptiveController()
		for i := 0; i < n+1; i++ {
			ac.Update(media.LinkMetrics{RTT: 5, LossRate: 0.9, JitterMs: 400})
		}
		br := ac.Bitrate()
		return br >= proto.MinBitrate && br <= proto.MaxBitrate
	}), pbt.WithRuns(40))
}
