// SPDX-License-Identifier: 0BSD
package session_test

import (
	"errors"
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv"
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
	"quad4/reticulum-go-protocols/pkg/rnv/session"
)

func TestGuardLowVideo(t *testing.T) {
	local := proto.DefaultCaps()
	remote := proto.DefaultCaps()
	err := session.GuardStreamOffer(local, remote, proto.StreamOffer{
		Profile: proto.ProfileLow,
		Tracks:  proto.TrackVideo,
		Video:   proto.CodecJPEG,
	})
	if !errors.Is(err, rnv.ErrVideoTrackDenied) {
		t.Fatalf("%v", err)
	}
}

func TestGuardMediumOK(t *testing.T) {
	local := proto.DefaultCaps()
	remote := proto.DefaultCaps()
	err := session.GuardStreamOffer(local, remote, proto.StreamOffer{
		Profile: proto.ProfileMedium,
		Tracks:  proto.TrackVideo | proto.TrackAudio,
		Video:   proto.CodecJPEG,
		Audio:   proto.CodecOpus,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGuardUltraLowAudio(t *testing.T) {
	local := proto.DefaultCaps()
	remote := proto.DefaultCaps()
	err := session.GuardStreamOffer(local, remote, proto.StreamOffer{
		Profile: proto.ProfileUltraLow,
		Tracks:  proto.TrackAudio,
		Audio:   proto.CodecCodec2,
	})
	if !errors.Is(err, rnv.ErrAudioTrackDenied) && !errors.Is(err, rnv.ErrInvalidOffer) {
		t.Fatalf("%v", err)
	}
}

func TestMutationAbsoluteStillCap(t *testing.T) {
	if err := rnv.ValidateStillMeta(proto.StillMeta{Width: 1, Height: 1, Size: rnv.MaxStillBytes + 1}, rnv.MaxStillBytes*2); !errors.Is(err, rnv.ErrStillTooLarge) {
		t.Fatalf("absolute max must still apply: %v", err)
	}
}

func TestParallelLXSTConfigDefault(t *testing.T) {
	cfg := session.SafeConfig()
	if cfg.AllowParallelLXST {
		t.Fatal("expected AllowParallelLXST false")
	}
	cfg.LXSTActive = func([]byte) bool { return true }
	if cfg.LXSTActive(nil) != true {
		t.Fatal("hook")
	}
}
