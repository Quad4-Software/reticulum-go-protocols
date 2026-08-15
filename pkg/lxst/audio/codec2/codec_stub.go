//go:build !cgo

// SPDX-License-Identifier: Apache-2.0
package codec2

import (
	"fmt"

	"quad4/reticulum-go-protocols/pkg/lxst/audio/opus"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

type stubCodec struct {
	header byte
	frame  int
	play   int
	closed bool
}

func NewEncoder(cfg Config) (opus.Encoder, error) {
	return newStub(cfg), nil
}

func NewDecoder(cfg Config) (opus.Decoder, error) {
	return newStub(cfg), nil
}

func newStub(cfg Config) *stubCodec {
	if cfg.FrameSamples <= 0 {
		cfg.FrameSamples = SampleRate * 200 / 1000
	}
	if cfg.PlaySamples <= 0 {
		cfg.PlaySamples = proto.PlaybackSampleRate * 200 / 1000
	}
	if cfg.Header == 0 && cfg.Bitrate != 700 {
		cfg.Header = headerForBitrate(cfg.Bitrate)
	}
	return &stubCodec{header: cfg.Header, frame: cfg.FrameSamples, play: cfg.PlaySamples}
}

func (s *stubCodec) Encode(pcm []int16) ([]byte, error) {
	if s.closed {
		return nil, ErrCodecClosed
	}
	if len(pcm) == 0 {
		return nil, fmt.Errorf("empty pcm")
	}
	n := len(pcm)
	out := make([]byte, 5+n*2)
	out[0] = s.header
	out[1] = 'S'
	out[2] = 'T'
	out[3] = byte(n >> 8)
	out[4] = byte(n)
	for i := 0; i < n; i++ {
		out[5+i*2] = byte(pcm[i] >> 8)
		out[5+i*2+1] = byte(pcm[i])
	}
	return out, nil
}

func (s *stubCodec) Decode(packet []byte) ([]int16, error) {
	if s.closed {
		return nil, ErrCodecClosed
	}
	if len(packet) < 5 || packet[1] != 'S' || packet[2] != 'T' {
		return nil, fmt.Errorf("invalid stub codec2 packet")
	}
	n := int(packet[3])<<8 | int(packet[4])
	if len(packet) < 5+n*2 {
		return nil, fmt.Errorf("truncated stub codec2 packet")
	}
	pcm := make([]int16, n)
	for i := range pcm {
		pcm[i] = int16(packet[5+i*2])<<8 | int16(packet[5+i*2+1])
	}
	return opus.Upsample(pcm, SampleRate, proto.PlaybackSampleRate), nil
}

func (s *stubCodec) DecodePLC() ([]int16, error) {
	if s.closed {
		return nil, ErrCodecClosed
	}
	return make([]int16, s.play), nil
}

func (s *stubCodec) SetBitrate(int) error {
	if s.closed {
		return ErrCodecClosed
	}
	return nil
}

func (s *stubCodec) SetFEC(bool) error {
	return s.SetBitrate(0)
}

func (s *stubCodec) SetPacketLossPerc(int) error {
	return s.SetBitrate(0)
}

func (s *stubCodec) FrameSamples() int {
	return s.frame
}

func (s *stubCodec) Close() error {
	s.closed = true
	return nil
}
