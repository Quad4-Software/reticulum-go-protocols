// SPDX-License-Identifier: 0BSD
package media_test

import (
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/rnv/media"
)

func TestJitterReorder(t *testing.T) {
	jb := media.NewJitterBuffer(10, 8)
	jb.PushOwned(2, []byte{2})
	jb.PushOwned(1, []byte{1})
	time.Sleep(15 * time.Millisecond)
	f1, ok := jb.PopReady(time.Now())
	if !ok || f1.Sequence != 1 {
		t.Fatalf("%v %+v", ok, f1)
	}
	f2, ok := jb.PopReady(time.Now())
	if !ok || f2.Sequence != 2 {
		t.Fatalf("%v %+v", ok, f2)
	}
}

func TestClockSkew(t *testing.T) {
	var c media.Clock
	c.NoteVideo(1)
	time.Sleep(5 * time.Millisecond)
	c.NoteAudio(1)
	if c.Skew() <= 0 {
		t.Fatalf("skew %v", c.Skew())
	}
}
