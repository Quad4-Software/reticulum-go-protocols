// SPDX-License-Identifier: 0BSD
package proto

import (
	"fmt"
	"sync"
)

// CodecFunc encodes or decodes opaque media for a registered codec id.
type CodecFunc func(in []byte) ([]byte, error)

// CodecRegistry maps private or alternate codec ids to handlers.
type CodecRegistry struct {
	mu     sync.RWMutex
	encode map[byte]CodecFunc
	decode map[byte]CodecFunc
}

// DefaultRegistry is the process-wide registry for private codecs.
var DefaultRegistry = NewCodecRegistry()

// NewCodecRegistry creates an empty registry.
func NewCodecRegistry() *CodecRegistry {
	return &CodecRegistry{
		encode: make(map[byte]CodecFunc),
		decode: make(map[byte]CodecFunc),
	}
}

// Register installs encode/decode handlers for a private codec id (0xE0-0xFE)
// or an alternate known id. Builtin JPEG/Opus/Codec2 remain protocol-defined.
func (r *CodecRegistry) Register(id byte, encode, decode CodecFunc) error {
	if r == nil {
		return fmt.Errorf("rnv proto: nil registry")
	}
	if id < CodecPrivateMin || id > CodecPrivateMax {
		if id == CodecJPEG || id == CodecOpus || id == CodecCodec2 || id == CodecOpaque {
			return fmt.Errorf("rnv proto: cannot replace builtin codec 0x%02x", id)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if encode != nil {
		r.encode[id] = encode
	}
	if decode != nil {
		r.decode[id] = decode
	}
	return nil
}

// Encode runs a registered encoder.
func (r *CodecRegistry) Encode(id byte, in []byte) ([]byte, error) {
	r.mu.RLock()
	fn := r.encode[id]
	r.mu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("rnv proto: no encoder for 0x%02x", id)
	}
	return fn(in)
}

// Decode runs a registered decoder.
func (r *CodecRegistry) Decode(id byte, in []byte) ([]byte, error) {
	r.mu.RLock()
	fn := r.decode[id]
	r.mu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("rnv proto: no decoder for 0x%02x", id)
	}
	return fn(in)
}

// Has reports whether encode or decode is registered for id.
func (r *CodecRegistry) Has(id byte) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.encode[id] != nil || r.decode[id] != nil
}

// KnownBuiltin reports whether codec is a v1 builtin.
func KnownBuiltin(id byte) bool {
	switch id {
	case CodecJPEG, CodecOpaque, CodecOpus, CodecCodec2, CodecPCM16:
		return true
	case CodecAVIF, CodecH264, CodecVP8:
		return false
	default:
		return false
	}
}
