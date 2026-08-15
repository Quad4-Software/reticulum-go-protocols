// SPDX-License-Identifier: Apache-2.0
package media_test

import (
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/media"
)

func TestOracleReorderThenPlay(t *testing.T) {
	jb := media.NewJitterBuffer(20, 8)
	now := time.Now()
	jb.Push(2, 40, []byte("b"))
	jb.Push(1, 20, []byte("a"))
	f1, ok := jb.PopReady(now.Add(5 * time.Millisecond))
	if !ok || f1.Sequence != 1 || string(f1.Payload) != "a" {
		t.Fatalf("expected seq 1 payload a, got %+v ok=%v", f1, ok)
	}
	f2, ok := jb.PopReady(now.Add(10 * time.Millisecond))
	if !ok || f2.Sequence != 2 || string(f2.Payload) != "b" {
		t.Fatalf("expected seq 2 payload b, got %+v ok=%v", f2, ok)
	}
}

func TestOracleMissingFrameSkipCountsLoss(t *testing.T) {
	jb := media.NewJitterBuffer(40, 8)
	now := time.Now()
	jb.Push(1, 1, []byte{1})
	jb.Push(3, 3, []byte{3})
	f, ok := jb.PopReady(now)
	if !ok || f.Sequence != 1 {
		t.Fatalf("first %+v", f)
	}
	_, ok = jb.PopReady(now.Add(5 * time.Millisecond))
	if ok {
		t.Fatal("seq 2 should still be waiting")
	}
	_, ok = jb.PopReady(now.Add(50 * time.Millisecond))
	if ok {
		t.Fatal("skip should not return a fabricated frame")
	}
	f, ok = jb.PopReady(now.Add(55 * time.Millisecond))
	if !ok || f.Sequence != 3 {
		t.Fatalf("expected seq 3 after skip, got %+v ok=%v", f, ok)
	}
	if jb.LossRate() == 0 {
		t.Fatal("skip must count loss")
	}
}
