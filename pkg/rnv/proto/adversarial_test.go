// SPDX-License-Identifier: 0BSD
package proto_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

func TestAdversarialUnknownMagicAndStrict(t *testing.T) {
	if _, err := proto.SplitFrame([]byte{0x00, 1, 2, 3, 4, 5}); err == nil {
		t.Fatal("bad magic")
	}
	huge := make([]byte, proto.MaxStreamFrameBytes+10)
	if _, err := proto.PackVideo(proto.CodecJPEG, 0, 1, huge); err == nil {
		t.Fatal("oversize video")
	}
	ahuge := make([]byte, proto.MaxAudioFrameBytes+1)
	if _, err := proto.PackAudio(proto.CodecOpus, 0, 1, ahuge); err == nil {
		t.Fatal("oversize audio")
	}
}

func TestAdversarialUnknownKeysSkipped(t *testing.T) {
	env := proto.NewTyped(proto.TypeHello, map[uint64]any{
		proto.HelloKeyPreferred: uint64(proto.ProfileLow),
		9999:                    "ignored",
	})
	raw, err := env.Pack()
	if err != nil {
		t.Fatal(err)
	}
	got, err := proto.UnpackEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != proto.TypeHello {
		t.Fatal(got.Type)
	}
}
