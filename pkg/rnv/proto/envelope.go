// SPDX-License-Identifier: 0BSD
package proto

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
)

var (
	envelopeEnc cbor.UserBufferEncMode
	envelopeDec cbor.DecMode
	bufPool     = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)

func init() {
	enc, err := cbor.EncOptions{}.UserBufferEncMode()
	if err != nil {
		panic("rnv proto: cbor encoder: " + err.Error())
	}
	envelopeEnc = enc
	dec, err := cbor.DecOptions{
		MaxNestedLevels:  8,
		MaxArrayElements: 256,
		MaxMapPairs:      128,
	}.DecMode()
	if err != nil {
		panic("rnv proto: cbor decoder: " + err.Error())
	}
	envelopeDec = dec
}

// Envelope is a top-level RNV CBOR signalling message.
type Envelope struct {
	Version    uint64
	Type       uint64
	Body       map[uint64]any
	Extensions map[uint64][]byte
}

// Pack marshals the envelope. Unknown body keys are caller-owned.
func (e *Envelope) Pack() ([]byte, error) {
	if e == nil {
		return nil, fmt.Errorf("rnv proto: nil envelope")
	}
	if e.Version == 0 {
		e.Version = ProtocolVersion
	}
	if e.Version != ProtocolVersion {
		return nil, fmt.Errorf("rnv proto: wrong version %d", e.Version)
	}
	m := make(map[uint64]any, 4)
	m[FieldVersion] = e.Version
	m[FieldType] = e.Type
	if e.Body != nil {
		m[FieldBody] = e.Body
	}
	if len(e.Extensions) > 0 {
		ext := make(map[uint64]any, len(e.Extensions))
		for k, v := range e.Extensions {
			ext[k] = v
		}
		m[FieldExtensions] = ext
	}
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	if err := envelopeEnc.MarshalToBuffer(m, buf); err != nil {
		bufPool.Put(buf)
		return nil, err
	}
	if buf.Len() > MaxEnvelopeBytes {
		bufPool.Put(buf)
		return nil, fmt.Errorf("rnv proto: envelope too large")
	}
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	bufPool.Put(buf)
	return out, nil
}

// UnpackEnvelope decodes CBOR and skips unknown top-level keys.
func UnpackEnvelope(raw []byte) (*Envelope, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("rnv proto: empty envelope")
	}
	if len(raw) > MaxEnvelopeBytes {
		return nil, fmt.Errorf("rnv proto: envelope too large")
	}
	var m map[uint64]any
	if err := envelopeDec.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	e := &Envelope{Body: map[uint64]any{}}
	if v, ok := m[FieldVersion]; ok {
		e.Version = asUint64(v)
	}
	if e.Version != ProtocolVersion {
		return nil, fmt.Errorf("rnv proto: wrong version %d", e.Version)
	}
	if v, ok := m[FieldType]; ok {
		e.Type = asUint64(v)
	}
	if v, ok := m[FieldBody]; ok {
		e.Body = asUint64Map(v)
	}
	if v, ok := m[FieldExtensions]; ok {
		e.Extensions = asExtMap(v)
	}
	return e, nil
}

func asUint64(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case uint32:
		return uint64(n)
	case uint16:
		return uint64(n)
	case uint8:
		return uint64(n)
	case int64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int32:
		if n < 0 {
			return 0
		}
		return uint64(n)
	default:
		return 0
	}
}

func asUint64Map(v any) map[uint64]any {
	switch m := v.(type) {
	case map[uint64]any:
		return m
	case map[any]any:
		out := make(map[uint64]any, len(m))
		for k, val := range m {
			out[asUint64(k)] = val
		}
		return out
	default:
		return map[uint64]any{}
	}
}

func asExtMap(v any) map[uint64][]byte {
	m := asUint64Map(v)
	out := make(map[uint64][]byte, len(m))
	for k, val := range m {
		switch b := val.(type) {
		case []byte:
			out[k] = append([]byte(nil), b...)
		case string:
			out[k] = []byte(b)
		}
	}
	return out
}

func asString(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return ""
	}
}

func asBytes(v any) []byte {
	switch b := v.(type) {
	case []byte:
		return append([]byte(nil), b...)
	case string:
		return []byte(b)
	default:
		return nil
	}
}

// Caps describes peer capabilities exchanged in HELLO.
type Caps struct {
	MaxStill   uint64
	MaxClip    uint64
	Profiles   []int
	Codecs     []byte
	Tracks     byte
	Preferred  int
	StrictExt  bool
	Extensions map[uint64][]byte
}

func (c Caps) ToBody() map[uint64]any {
	profs := make([]any, len(c.Profiles))
	for i, p := range c.Profiles {
		profs[i] = uint64(p) // #nosec G115 -- profile constants fit uint64
	}
	codecs := make([]any, len(c.Codecs))
	for i, co := range c.Codecs {
		codecs[i] = uint64(co)
	}
	body := map[uint64]any{
		HelloKeyMaxStill:  c.MaxStill,
		HelloKeyMaxClip:   c.MaxClip,
		HelloKeyProfiles:  profs,
		HelloKeyCodecs:    codecs,
		HelloKeyTracks:    uint64(c.Tracks),
		HelloKeyPreferred: uint64(c.Preferred), // #nosec G115
	}
	if c.StrictExt {
		body[HelloKeyStrictExt] = true
	}
	if len(c.Extensions) > 0 {
		ext := make(map[uint64]any, len(c.Extensions))
		for k, v := range c.Extensions {
			ext[k] = v
		}
		body[HelloKeyExtensions] = ext
	}
	return body
}

func CapsFromBody(body map[uint64]any) Caps {
	c := Caps{
		MaxStill:  MaxStillBytes,
		MaxClip:   MaxClipBytes,
		Preferred: ProfileLow,
		Tracks:    TrackVideo | TrackAudio,
	}
	if body == nil {
		return c
	}
	if v, ok := body[HelloKeyMaxStill]; ok {
		c.MaxStill = asUint64(v)
	}
	if v, ok := body[HelloKeyMaxClip]; ok {
		c.MaxClip = asUint64(v)
	}
	if v, ok := body[HelloKeyPreferred]; ok {
		c.Preferred = int(asUint64(v))
	}
	if v, ok := body[HelloKeyTracks]; ok {
		c.Tracks = byte(asUint64(v))
	}
	if v, ok := body[HelloKeyStrictExt]; ok {
		if b, ok := v.(bool); ok {
			c.StrictExt = b
		}
	}
	if v, ok := body[HelloKeyProfiles]; ok {
		c.Profiles = asIntList(v)
	}
	if v, ok := body[HelloKeyCodecs]; ok {
		c.Codecs = asByteList(v)
	}
	if v, ok := body[HelloKeyExtensions]; ok {
		c.Extensions = asExtMap(v)
	}
	return c
}

func asIntList(v any) []int {
	switch a := v.(type) {
	case []any:
		out := make([]int, 0, len(a))
		for _, x := range a {
			out = append(out, int(asUint64(x)))
		}
		return out
	default:
		return nil
	}
}

func asByteList(v any) []byte {
	switch a := v.(type) {
	case []any:
		out := make([]byte, 0, len(a))
		for _, x := range a {
			out = append(out, byte(asUint64(x)))
		}
		return out
	case []byte:
		return append([]byte(nil), a...)
	default:
		return nil
	}
}

// StillMeta describes an inbound or outbound still.
type StillMeta struct {
	Width    int
	Height   int
	Codec    byte
	Size     uint64
	Transfer int
	ID       []byte
	Data     []byte
}

func (s StillMeta) ToBody() map[uint64]any {
	body := map[uint64]any{
		StillKeyWidth:    uint64(s.Width),  // #nosec G115
		StillKeyHeight:   uint64(s.Height), // #nosec G115
		StillKeyCodec:    uint64(s.Codec),
		StillKeySize:     s.Size,
		StillKeyTransfer: uint64(s.Transfer), // #nosec G115
	}
	if len(s.ID) > 0 {
		body[StillKeyID] = s.ID
	}
	if len(s.Data) > 0 {
		body[StillKeyData] = s.Data
	}
	return body
}

func StillMetaFromBody(body map[uint64]any) StillMeta {
	s := StillMeta{Codec: CodecJPEG, Transfer: TransferPacket}
	if body == nil {
		return s
	}
	s.Width = int(asUint64(body[StillKeyWidth]))
	s.Height = int(asUint64(body[StillKeyHeight]))
	s.Codec = byte(asUint64(body[StillKeyCodec]))
	s.Size = asUint64(body[StillKeySize])
	s.Transfer = int(asUint64(body[StillKeyTransfer]))
	s.ID = asBytes(body[StillKeyID])
	s.Data = asBytes(body[StillKeyData])
	return s
}

// ClipMeta describes a clip offer or accept.
type ClipMeta struct {
	ID     []byte
	Size   uint64
	Codec  byte
	Mime   string
	SHA256 []byte
}

func (c ClipMeta) ToBody() map[uint64]any {
	body := map[uint64]any{
		ClipKeySize:  c.Size,
		ClipKeyCodec: uint64(c.Codec),
	}
	if len(c.ID) > 0 {
		body[ClipKeyID] = c.ID
	}
	if c.Mime != "" {
		body[ClipKeyMime] = c.Mime
	}
	if len(c.SHA256) > 0 {
		body[ClipKeySHA256] = c.SHA256
	}
	return body
}

func ClipMetaFromBody(body map[uint64]any) ClipMeta {
	c := ClipMeta{Codec: CodecOpaque}
	if body == nil {
		return c
	}
	c.ID = asBytes(body[ClipKeyID])
	c.Size = asUint64(body[ClipKeySize])
	c.Codec = byte(asUint64(body[ClipKeyCodec]))
	c.Mime = asString(body[ClipKeyMime])
	c.SHA256 = asBytes(body[ClipKeySHA256])
	return c
}

// StreamOffer is negotiated before live media.
type StreamOffer struct {
	Profile int
	Tracks  byte
	Video   byte
	Audio   byte
	MaxFPS  int
}

func (s StreamOffer) ToBody() map[uint64]any {
	body := map[uint64]any{
		StreamKeyProfile: uint64(s.Profile), // #nosec G115
		StreamKeyTracks:  uint64(s.Tracks),
	}
	if s.Video != 0 {
		body[StreamKeyVideo] = uint64(s.Video)
	}
	if s.Audio != 0 {
		body[StreamKeyAudio] = uint64(s.Audio)
	}
	if s.MaxFPS > 0 {
		body[StreamKeyMaxFPS] = uint64(s.MaxFPS) // #nosec G115
	}
	return body
}

func StreamOfferFromBody(body map[uint64]any) StreamOffer {
	s := StreamOffer{Profile: ProfileMedium, Video: CodecJPEG, Audio: CodecOpus}
	if body == nil {
		return s
	}
	s.Profile = int(asUint64(body[StreamKeyProfile]))
	s.Tracks = byte(asUint64(body[StreamKeyTracks]))
	if v, ok := body[StreamKeyVideo]; ok {
		s.Video = byte(asUint64(v))
	}
	if v, ok := body[StreamKeyAudio]; ok {
		s.Audio = byte(asUint64(v))
	}
	if v, ok := body[StreamKeyMaxFPS]; ok {
		s.MaxFPS = int(asUint64(v))
	}
	return s
}

// RejectBody is a typed rejection.
type RejectBody struct {
	Code   int
	Reason string
}

func (r RejectBody) ToBody() map[uint64]any {
	body := map[uint64]any{RejectKeyCode: uint64(r.Code)} // #nosec G115
	if r.Reason != "" {
		body[RejectKeyReason] = r.Reason
	}
	return body
}

func RejectFromBody(body map[uint64]any) RejectBody {
	r := RejectBody{}
	if body == nil {
		return r
	}
	r.Code = int(asUint64(body[RejectKeyCode]))
	r.Reason = asString(body[RejectKeyReason])
	return r
}

// CtrlBody is STREAM_CTRL.
type CtrlBody struct {
	Bitrate  int
	Keyframe bool
	Pause    bool
}

func (c CtrlBody) ToBody() map[uint64]any {
	body := map[uint64]any{}
	if c.Bitrate > 0 {
		body[CtrlKeyBitrate] = uint64(c.Bitrate) // #nosec G115
	}
	if c.Keyframe {
		body[CtrlKeyKeyframe] = true
	}
	if c.Pause {
		body[CtrlKeyPause] = true
	}
	return body
}

func CtrlFromBody(body map[uint64]any) CtrlBody {
	c := CtrlBody{}
	if body == nil {
		return c
	}
	c.Bitrate = int(asUint64(body[CtrlKeyBitrate]))
	if v, ok := body[CtrlKeyKeyframe].(bool); ok {
		c.Keyframe = v
	}
	if v, ok := body[CtrlKeyPause].(bool); ok {
		c.Pause = v
	}
	return c
}

// NewTyped builds a versioned envelope of the given type.
func NewTyped(typ uint64, body map[uint64]any) *Envelope {
	return &Envelope{Version: ProtocolVersion, Type: typ, Body: body}
}
