// SPDX-License-Identifier: 0BSD
package proto

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
)

var announceEnc cbor.UserBufferEncMode

func init() {
	enc, err := cbor.EncOptions{}.UserBufferEncMode()
	if err != nil {
		panic("rnv proto: announce encoder: " + err.Error())
	}
	announceEnc = enc
}

var announceBufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// AnnounceAppData is public metadata placed on destination announces.
type AnnounceAppData struct {
	Version  uint64
	Caps     uint64
	Profile  int
	ExtBloom []byte
}

// EncodeAnnounceAppData packs announce metadata.
func EncodeAnnounceAppData(a AnnounceAppData) ([]byte, error) {
	if a.Version == 0 {
		a.Version = ProtocolVersion
	}
	m := map[uint64]any{
		AnnounceKeyVersion: a.Version,
		AnnounceKeyCaps:    a.Caps,
		AnnounceKeyProfile: uint64(a.Profile), // #nosec G115
	}
	if len(a.ExtBloom) > 0 {
		m[AnnounceKeyExtBloom] = a.ExtBloom
	}
	buf := announceBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if err := announceEnc.MarshalToBuffer(m, buf); err != nil {
		announceBufPool.Put(buf)
		return nil, err
	}
	if buf.Len() > 512 {
		announceBufPool.Put(buf)
		return nil, fmt.Errorf("rnv proto: announce app-data too large")
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	announceBufPool.Put(buf)
	return out, nil
}

// DecodeAnnounceAppData unpacks announce metadata and skips unknown keys.
func DecodeAnnounceAppData(raw []byte) (AnnounceAppData, error) {
	a := AnnounceAppData{Version: ProtocolVersion, Profile: ProfileLow}
	if len(raw) == 0 {
		return a, nil
	}
	var m map[uint64]any
	if err := envelopeDec.Unmarshal(raw, &m); err != nil {
		return a, err
	}
	if v, ok := m[AnnounceKeyVersion]; ok {
		a.Version = asUint64(v)
	}
	if v, ok := m[AnnounceKeyCaps]; ok {
		a.Caps = asUint64(v)
	}
	if v, ok := m[AnnounceKeyProfile]; ok {
		a.Profile = int(asUint64(v))
	}
	if v, ok := m[AnnounceKeyExtBloom]; ok {
		a.ExtBloom = asBytes(v)
	}
	return a, nil
}

// CapsBitmap builds announce capability bits from Caps.
func CapsBitmap(c Caps) uint64 {
	var b uint64
	b |= CapStill
	if c.MaxClip > 0 {
		b |= CapClip
	}
	for _, p := range c.Profiles {
		lim := LimitsFor(p)
		if lim.AllowVideo {
			b |= CapStream
		}
		if lim.AllowAudio {
			b |= CapAudio
		}
	}
	if c.Tracks&TrackAudio != 0 {
		b |= CapAudio
	}
	if c.Tracks&TrackVideo != 0 {
		b |= CapStream
	}
	return b
}
