// SPDX-License-Identifier: Apache-2.0
package media_test

import (
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/media"
)

func BenchmarkJitterPushPop(b *testing.B) {
	jb := media.NewJitterBuffer(40, 32)
	payload := []byte{1, 2, 3, 4}
	now := time.Now()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		jb.Push(uint16(i), uint32(i), payload)
		_, _ = jb.PopReady(now.Add(time.Duration(i) * time.Millisecond))
	}
}

func TestJitterPushAllocs(t *testing.T) {
	jb := media.NewJitterBuffer(40, 32)
	payload := []byte{1, 2, 3, 4}
	n := testing.AllocsPerRun(200, func() {
		jb.Push(1, 1, payload)
	})
	if n > 3 {
		t.Fatalf("push allocs %v", n)
	}
}
