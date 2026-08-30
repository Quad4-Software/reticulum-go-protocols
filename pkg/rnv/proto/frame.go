// SPDX-License-Identifier: 0BSD
package proto

import (
	"encoding/binary"
	"fmt"
)

// Frame is a live media frame (video or audio).
type Frame struct {
	Magic   byte
	Codec   byte
	Flags   byte
	Seq     uint16
	Payload []byte
}

// PackFrame builds a binary media frame.
func PackFrame(magic, codec, flags byte, seq uint16, payload []byte) ([]byte, error) {
	max := MaxStreamFrameBytes
	if magic == MagicAudio {
		max = MaxAudioFrameBytes
	}
	if len(payload) > max {
		return nil, fmt.Errorf("rnv proto: frame payload too large")
	}
	out := make([]byte, FrameHeaderLen+len(payload))
	out[0] = magic
	out[1] = codec
	out[2] = flags
	binary.BigEndian.PutUint16(out[3:5], seq)
	copy(out[FrameHeaderLen:], payload)
	return out, nil
}

// PackVideo packs a video frame (magic 0xF1).
func PackVideo(codec, flags byte, seq uint16, payload []byte) ([]byte, error) {
	return PackFrame(MagicVideo, codec, flags, seq, payload)
}

// PackAudio packs an audio frame (magic 0xF2).
func PackAudio(codec, flags byte, seq uint16, payload []byte) ([]byte, error) {
	return PackFrame(MagicAudio, codec, flags, seq, payload)
}

// SplitFrame parses a binary media frame.
func SplitFrame(raw []byte) (Frame, error) {
	if len(raw) < FrameHeaderLen {
		return Frame{}, fmt.Errorf("rnv proto: frame too short")
	}
	magic := raw[0]
	if magic != MagicVideo && magic != MagicAudio {
		return Frame{}, fmt.Errorf("rnv proto: bad frame magic 0x%02x", magic)
	}
	max := MaxStreamFrameBytes
	if magic == MagicAudio {
		max = MaxAudioFrameBytes
	}
	payload := raw[FrameHeaderLen:]
	if len(payload) > max {
		return Frame{}, fmt.Errorf("rnv proto: frame payload too large")
	}
	return Frame{
		Magic:   magic,
		Codec:   raw[1],
		Flags:   raw[2],
		Seq:     binary.BigEndian.Uint16(raw[3:5]),
		Payload: append([]byte(nil), payload...),
	}, nil
}

// IsMediaFrame reports whether raw looks like a live media frame.
func IsMediaFrame(raw []byte) bool {
	if len(raw) < 1 {
		return false
	}
	return raw[0] == MagicVideo || raw[0] == MagicAudio
}
