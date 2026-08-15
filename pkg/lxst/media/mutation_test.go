// SPDX-License-Identifier: Apache-2.0
package media_test

import (
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/media"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func TestMutationReorderIsNotLoss(t *testing.T) {
	jb := media.NewJitterBuffer(20, 8)
	jb.Push(2, 40, []byte{2})
	jb.Push(1, 20, []byte{1})
	if jb.LossRate() != 0 {
		t.Fatalf("reorder counted as loss: %f", jb.LossRate())
	}
}

func TestMutationSkipMissingIsLoss(t *testing.T) {
	jb := media.NewJitterBuffer(40, 8)
	now := time.Now()
	jb.Push(1, 1, []byte{1})
	jb.Push(3, 3, []byte{3})
	_, _ = jb.PopReady(now)
	_, _ = jb.PopReady(now.Add(5 * time.Millisecond))
	_, _ = jb.PopReady(now.Add(50 * time.Millisecond))
	if jb.LossRate() == 0 {
		t.Fatal("skipping a missing seq must count loss")
	}
}

func TestMutationAdaptiveFloorIsMinBitrate(t *testing.T) {
	ac := media.NewAdaptiveController()
	for range 40 {
		ac.Update(media.LinkMetrics{RTT: 8, LossRate: 1, JitterMs: 400, Q: 0, SNR: 0})
	}
	if ac.Bitrate() < proto.MinBitrate {
		t.Fatalf("bitrate %d below min", ac.Bitrate())
	}
	if ac.Bitrate() > proto.MaxBitrate {
		t.Fatalf("bitrate %d above max", ac.Bitrate())
	}
}
