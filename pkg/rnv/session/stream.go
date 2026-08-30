// SPDX-License-Identifier: 0BSD
package session

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"quad4/reticulum-go-protocols/pkg/rnv"
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

// Stream is an accepted live media session on a Conn.
type Stream struct {
	conn     *Conn
	offer    proto.StreamOffer
	mu       sync.Mutex
	closed   bool
	videoSeq uint32
	audioSeq uint32
	acceptCh chan proto.StreamOffer
	rejectCh chan proto.RejectBody
}

// OpenStream negotiates a live stream. Video requires profile Medium or High.
func (c *Conn) OpenStream(ctx context.Context, offer proto.StreamOffer) (*Stream, error) {
	if err := c.requireHandshake(); err != nil {
		return nil, err
	}
	if offer.Profile == 0 {
		offer.Profile = proto.ProfileMedium
	}
	if offer.Tracks == 0 {
		offer.Tracks = proto.TrackVideo
	}
	if offer.Tracks&proto.TrackVideo != 0 && offer.Profile < proto.ProfileMedium {
		return nil, rnv.ErrVideoTrackDenied
	}
	if err := c.ensureActiveLXSTGuard(offer.Tracks); err != nil {
		return nil, err
	}
	if err := GuardStreamOffer(c.LocalCaps(), c.PeerCaps(), offer); err != nil {
		return nil, err
	}

	c.mu.Lock()
	if c.stream != nil && !c.stream.closed {
		c.mu.Unlock()
		return nil, rnv.ErrStreamAlreadyOpen
	}
	sc := &Stream{
		conn:     c,
		offer:    offer,
		acceptCh: make(chan proto.StreamOffer, 1),
		rejectCh: make(chan proto.RejectBody, 1),
	}
	c.stream = sc
	c.mu.Unlock()

	if err := c.sendEnvelope(proto.NewTyped(proto.TypeStreamOffer, offer.ToBody())); err != nil {
		c.mu.Lock()
		c.stream = nil
		c.mu.Unlock()
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(c.ep.cfg.HelloTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		c.clearStream(sc)
		return nil, ctx.Err()
	case <-timer.C:
		c.clearStream(sc)
		return nil, rnv.ErrCapacity
	case rej := <-sc.rejectCh:
		c.clearStream(sc)
		if rej.Code == proto.RejectCapacity {
			return nil, rnv.ErrCapacity
		}
		return nil, rnv.ErrRejected
	case accepted := <-sc.acceptCh:
		sc.offer = accepted
		return sc, nil
	}
}

func (c *Conn) clearStream(sc *Stream) {
	c.mu.Lock()
	if c.stream == sc {
		c.stream = nil
	}
	c.mu.Unlock()
}

func (c *Conn) handleStreamOffer(offer proto.StreamOffer) {
	if err := GuardStreamOffer(c.LocalCaps(), c.PeerCaps(), offer); err != nil {
		code := proto.RejectCapacity
		if err == rnv.ErrInvalidOffer || err == rnv.ErrVideoTrackDenied || err == rnv.ErrAudioTrackDenied {
			code = proto.RejectCapacity
		}
		_ = c.sendReject(code, err.Error())
		return
	}
	if err := c.ensureActiveLXSTGuard(offer.Tracks); err != nil {
		_ = c.sendReject(proto.RejectPolicy, err.Error())
		return
	}
	sc := &Stream{conn: c, offer: offer}
	c.mu.Lock()
	c.stream = sc
	c.mu.Unlock()
	_ = c.sendEnvelope(proto.NewTyped(proto.TypeStreamAccept, offer.ToBody()))
	if h := c.ep.cfg.Handlers.OnStream; h != nil {
		h(c, offer)
	}
}

func (c *Conn) handleStreamAccept(offer proto.StreamOffer) {
	c.mu.Lock()
	sc := c.stream
	c.mu.Unlock()
	if sc == nil {
		return
	}
	select {
	case sc.acceptCh <- offer:
	default:
	}
}

// Offer returns the negotiated stream offer.
func (s *Stream) Offer() proto.StreamOffer {
	if s == nil {
		return proto.StreamOffer{}
	}
	return s.offer
}

// SendVideo sends one MJPEG (or registered) video frame.
func (s *Stream) SendVideo(payload []byte) error {
	if s == nil || s.conn == nil {
		return rnv.ErrStreamNotOpen
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return rnv.ErrStreamNotOpen
	}
	if s.offer.Tracks&proto.TrackVideo == 0 {
		s.mu.Unlock()
		return rnv.ErrVideoTrackDenied
	}
	seq := uint16(atomic.AddUint32(&s.videoSeq, 1)) // #nosec G115
	codec := s.offer.Video
	if codec == 0 {
		codec = proto.CodecJPEG
	}
	s.mu.Unlock()
	raw, err := proto.PackVideo(codec, proto.FlagKeyframe, seq, payload)
	if err != nil {
		return rnv.ErrFrameTooLarge
	}
	return s.conn.sendRaw(raw)
}

// SendAudio sends one Opus/Codec2 (or registered) audio frame.
func (s *Stream) SendAudio(payload []byte) error {
	if s == nil || s.conn == nil {
		return rnv.ErrStreamNotOpen
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return rnv.ErrStreamNotOpen
	}
	if s.offer.Tracks&proto.TrackAudio == 0 {
		s.mu.Unlock()
		return rnv.ErrAudioTrackDenied
	}
	seq := uint16(atomic.AddUint32(&s.audioSeq, 1)) // #nosec G115
	codec := s.offer.Audio
	if codec == 0 {
		codec = proto.CodecOpus
	}
	s.mu.Unlock()
	raw, err := proto.PackAudio(codec, 0, seq, payload)
	if err != nil {
		return rnv.ErrFrameTooLarge
	}
	return s.conn.sendRaw(raw)
}

// SendCtrl sends a STREAM_CTRL message.
func (s *Stream) SendCtrl(ctrl proto.CtrlBody) error {
	if s == nil || s.conn == nil {
		return rnv.ErrStreamNotOpen
	}
	return s.conn.sendEnvelope(proto.NewTyped(proto.TypeStreamCtrl, ctrl.ToBody()))
}

// Close marks the stream ended and sends EOS video flag if possible.
func (s *Stream) Close() error {
	if s == nil {
		return nil
	}
	s.markClosed()
	s.conn.clearStream(s)
	return nil
}

func (s *Stream) markClosed() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

// noteReject delivers a reject to a waiting OpenStream.
func (c *Conn) noteReject(body proto.RejectBody) {
	c.mu.Lock()
	sc := c.stream
	c.mu.Unlock()
	if sc == nil {
		return
	}
	select {
	case sc.rejectCh <- body:
	default:
	}
}
