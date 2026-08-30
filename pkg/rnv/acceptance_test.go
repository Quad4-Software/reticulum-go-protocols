// SPDX-License-Identifier: 0BSD
package rnv_test

import (
	"errors"
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv"
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
	"quad4/reticulum-go-protocols/pkg/rnv/session"
)

func TestAcceptanceConstants(t *testing.T) {
	if rnv.AppName != "rnv" || proto.MediaAspect != "media" {
		t.Fatal("destination name")
	}
	if proto.ProtocolVersion != 1 {
		t.Fatal("version")
	}
	if rnv.MaxStillBytes != 256<<10 || rnv.MaxClipBytes != 8<<20 {
		t.Fatal("caps")
	}
	if proto.MagicVideo != 0xF1 || proto.MagicAudio != 0xF2 {
		t.Fatal("magics")
	}
}

func TestRecommendStack(t *testing.T) {
	if rnv.RecommendStack(rnv.UseCaseVoiceOnly) != rnv.StackLXST {
		t.Fatal("voice")
	}
	if rnv.RecommendStack(rnv.UseCaseAV) != rnv.StackRNV {
		t.Fatal("av")
	}
	if rnv.RecommendStack(rnv.UseCaseStills) != rnv.StackRNV {
		t.Fatal("stills")
	}
}

func TestSafeConfigDefaults(t *testing.T) {
	cfg := session.SafeConfig()
	if cfg.Caps.Preferred != proto.ProfileLow {
		t.Fatal("preferred should be low")
	}
	if cfg.AllowParallelLXST {
		t.Fatal("parallel lxst should be off")
	}
}

func TestValidateStillClip(t *testing.T) {
	if err := rnv.ValidateStillMeta(proto.StillMeta{Width: 10000, Height: 10, Size: 10}, 100); !errors.Is(err, rnv.ErrBadDimensions) {
		t.Fatalf("%v", err)
	}
	if err := rnv.ValidateStillMeta(proto.StillMeta{Width: 10, Height: 10, Size: rnv.MaxStillBytes + 1}, 0); !errors.Is(err, rnv.ErrStillTooLarge) {
		t.Fatalf("%v", err)
	}
	if err := rnv.ValidateClipMeta(proto.ClipMeta{Size: rnv.MaxClipBytes + 1}, 0); !errors.Is(err, rnv.ErrClipTooLarge) {
		t.Fatalf("%v", err)
	}
}

func TestValidateOfferWrapper(t *testing.T) {
	local := proto.DefaultCaps()
	remote := proto.DefaultCaps()
	offer := proto.StreamOffer{Profile: proto.ProfileMedium, Tracks: proto.TrackVideo, Video: proto.CodecJPEG}
	if err := rnv.ValidateOffer(local, remote, offer); err != nil {
		t.Fatal(err)
	}
	bad := proto.StreamOffer{Profile: proto.ProfileUltraLow, Tracks: proto.TrackVideo}
	if err := rnv.ValidateOffer(local, remote, bad); !errors.Is(err, rnv.ErrInvalidOffer) {
		t.Fatalf("%v", err)
	}
}
