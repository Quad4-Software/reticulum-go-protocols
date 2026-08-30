// SPDX-License-Identifier: 0BSD
package proto_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

func FuzzUnpackEnvelope(f *testing.F) {
	env := proto.NewTyped(proto.TypeHello, proto.DefaultCaps().ToBody())
	raw, _ := env.Pack()
	f.Add(raw)
	f.Add([]byte{0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = proto.UnpackEnvelope(data)
	})
}

func FuzzSplitFrame(f *testing.F) {
	v, _ := proto.PackVideo(proto.CodecJPEG, 1, 1, []byte{1})
	f.Add(v)
	f.Add([]byte{0xf1})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = proto.SplitFrame(data)
	})
}
