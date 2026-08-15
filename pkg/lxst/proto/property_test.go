// SPDX-License-Identifier: Apache-2.0
package proto_test

import (
	"bytes"
	"math/rand"
	"testing"

	"quad4/pbt/pkg/pbt"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func TestPropertySignallingRoundTrip(t *testing.T) {
	gen := pbt.SliceOf(pbt.IntRange(0, 400), 1, 8)
	pbt.Check(t, pbt.ForAll("signalling round trip", gen, func(signals []int) bool {
		raw, err := proto.PackSignalling(signals)
		if err != nil {
			return false
		}
		pkt, err := proto.Unpack(raw)
		if err != nil {
			return false
		}
		if len(pkt.Signals) != len(signals) {
			return false
		}
		for i, s := range signals {
			if pkt.Signals[i] != s {
				return false
			}
		}
		return true
	}), pbt.WithRuns(200))
}

func TestPropertyFrameRoundTrip(t *testing.T) {
	gen := pbt.NewGenerator("payload", func(r *rand.Rand, _ int) []byte {
		n := r.Intn(48)
		b := make([]byte, n)
		_, _ = r.Read(b)
		return b
	})
	pbt.Check(t, pbt.ForAll("frame round trip", gen, func(payload []byte) bool {
		raw, err := proto.PackFrame(proto.CodecOpus, payload)
		if err != nil {
			return false
		}
		pkt, err := proto.Unpack(raw)
		if err != nil {
			return false
		}
		if len(pkt.Frames) != 1 {
			return false
		}
		codec, body, err := proto.SplitFrame(pkt.Frames[0])
		if err != nil {
			return false
		}
		return codec == proto.CodecOpus && bytes.Equal(body, payload)
	}), pbt.WithRuns(200))
}

func TestPropertyTelephonyHashLen(t *testing.T) {
	gen := pbt.NewGenerator("idhash", func(r *rand.Rand, _ int) []byte {
		b := make([]byte, 16)
		_, _ = r.Read(b)
		return b
	})
	pbt.Check(t, pbt.ForAll("telephony hash is 16 bytes", gen, func(id []byte) bool {
		h := proto.TelephonyHash(id)
		return len(h) == 16
	}), pbt.WithRuns(100))
}
