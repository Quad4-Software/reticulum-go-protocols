// SPDX-License-Identifier: 0BSD
package proto_test

import (
	"bytes"
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	caps := proto.DefaultCaps()
	env := proto.NewTyped(proto.TypeHello, caps.ToBody())
	raw, err := env.Pack()
	if err != nil {
		t.Fatal(err)
	}
	got, err := proto.UnpackEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != proto.TypeHello || got.Version != proto.ProtocolVersion {
		t.Fatalf("got %+v", got)
	}
	back := proto.CapsFromBody(got.Body)
	if back.Preferred != proto.ProfileLow || back.MaxStill != proto.MaxStillBytes {
		t.Fatalf("caps %+v", back)
	}
}

func TestFrameVideoAudioRoundTrip(t *testing.T) {
	v, err := proto.PackVideo(proto.CodecJPEG, proto.FlagKeyframe, 7, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	fv, err := proto.SplitFrame(v)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Magic != proto.MagicVideo || fv.Seq != 7 || !bytes.Equal(fv.Payload, []byte{1, 2, 3}) {
		t.Fatalf("%+v", fv)
	}
	a, err := proto.PackAudio(proto.CodecOpus, 0, 9, []byte{9, 8})
	if err != nil {
		t.Fatal(err)
	}
	fa, err := proto.SplitFrame(a)
	if err != nil {
		t.Fatal(err)
	}
	if fa.Magic != proto.MagicAudio || fa.Codec != proto.CodecOpus {
		t.Fatalf("%+v", fa)
	}
}

func TestFrameTooLarge(t *testing.T) {
	payload := make([]byte, proto.MaxStreamFrameBytes+1)
	if _, err := proto.PackVideo(proto.CodecJPEG, 0, 1, payload); err == nil {
		t.Fatal("expected error")
	}
}

func TestBadMagic(t *testing.T) {
	if _, err := proto.SplitFrame([]byte{0x00, 1, 2, 3, 4}); err == nil {
		t.Fatal("expected bad magic")
	}
}

func TestProfileMatrix(t *testing.T) {
	cases := []struct {
		profile int
		video   bool
		audio   bool
	}{
		{proto.ProfileUltraLow, false, false},
		{proto.ProfileLow, false, true},
		{proto.ProfileMedium, true, true},
		{proto.ProfileHigh, true, true},
	}
	for _, tc := range cases {
		lim := proto.LimitsFor(tc.profile)
		if lim.AllowVideo != tc.video || lim.AllowAudio != tc.audio {
			t.Fatalf("profile %x video=%v audio=%v", tc.profile, lim.AllowVideo, lim.AllowAudio)
		}
	}
}

func TestValidateStreamOfferMedium(t *testing.T) {
	local := proto.DefaultCaps()
	remote := proto.DefaultCaps()
	offer := proto.StreamOffer{Profile: proto.ProfileMedium, Tracks: proto.TrackVideo | proto.TrackAudio, Video: proto.CodecJPEG, Audio: proto.CodecOpus}
	if err := proto.ValidateStreamOffer(local, remote, offer); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStreamOfferLowVideoFails(t *testing.T) {
	local := proto.DefaultCaps()
	remote := proto.DefaultCaps()
	offer := proto.StreamOffer{Profile: proto.ProfileLow, Tracks: proto.TrackVideo, Video: proto.CodecJPEG}
	if err := proto.ValidateStreamOffer(local, remote, offer); err == nil {
		t.Fatal("expected failure")
	}
}

func TestAnnounceRoundTrip(t *testing.T) {
	raw, err := proto.EncodeAnnounceAppData(proto.AnnounceAppData{
		Caps:    proto.CapStill | proto.CapStream,
		Profile: proto.ProfileLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := proto.DecodeAnnounceAppData(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Profile != proto.ProfileLow || a.Caps&proto.CapStill == 0 {
		t.Fatalf("%+v", a)
	}
}

func TestRegistryPrivateCodec(t *testing.T) {
	r := proto.NewCodecRegistry()
	id := byte(0xE1)
	if err := r.Register(id, func(in []byte) ([]byte, error) {
		return append([]byte{0xAA}, in...), nil
	}, func(in []byte) ([]byte, error) {
		return in[1:], nil
	}); err != nil {
		t.Fatal(err)
	}
	enc, err := r.Encode(id, []byte{1})
	if err != nil || !bytes.Equal(enc, []byte{0xAA, 1}) {
		t.Fatalf("%x %v", enc, err)
	}
	if err := r.Register(proto.CodecJPEG, nil, nil); err == nil {
		t.Fatal("should not replace jpeg")
	}
}

func TestEnvelopeTooLarge(t *testing.T) {
	body := map[uint64]any{0: bytes.Repeat([]byte("x"), proto.MaxEnvelopeBytes)}
	env := proto.NewTyped(proto.TypeHello, body)
	if _, err := env.Pack(); err == nil {
		t.Fatal("expected too large")
	}
}

func TestStillClipMetaRoundTrip(t *testing.T) {
	s := proto.StillMeta{Width: 64, Height: 48, Codec: proto.CodecJPEG, Size: 100, Transfer: proto.TransferPacket, ID: []byte{1, 2}}
	env := proto.NewTyped(proto.TypeStill, s.ToBody())
	raw, _ := env.Pack()
	got, _ := proto.UnpackEnvelope(raw)
	sm := proto.StillMetaFromBody(got.Body)
	if sm.Width != 64 || sm.Height != 48 {
		t.Fatalf("%+v", sm)
	}
	c := proto.ClipMeta{Size: 50, Codec: proto.CodecOpaque, Mime: "video/mp4", ID: []byte{3}}
	cm := proto.ClipMetaFromBody(c.ToBody())
	if cm.Mime != "video/mp4" || cm.Size != 50 {
		t.Fatalf("%+v", cm)
	}
}

func TestMinProfile(t *testing.T) {
	if proto.MinProfile(proto.ProfileHigh, proto.ProfileLow) != proto.ProfileLow {
		t.Fatal("min")
	}
}
