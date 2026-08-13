// SPDX-License-Identifier: 0BSD
package rrc

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// Envelope is the top-level RRC CBOR map (3-RRC).
type Envelope struct {
	Version        uint64
	Type           uint64
	MsgID          []byte
	Timestamp      uint64
	Sender         []byte
	Room           string
	Body           any
	Nick           string
	Destination    []byte
	HasRoom        bool
	HasBody        bool
	HasNick        bool
	HasDestination bool
}

// NewEnvelope builds a version-1 envelope with a fresh message ID and timestamp.
func NewEnvelope(msgType uint64, sender []byte) (*Envelope, error) {
	if len(sender) != IdentityLength {
		return nil, fmt.Errorf("%w: sender identity", ErrBadFieldLength)
	}
	id := make([]byte, MessageIDLength)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	return &Envelope{
		Version:   ProtocolVersion,
		Type:      msgType,
		MsgID:     id,
		Timestamp: uint64(time.Now().UnixMilli()),
		Sender:    append([]byte(nil), sender...),
	}, nil
}

// Marshal encodes the envelope as a CBOR map with unsigned integer keys.
func (e *Envelope) Marshal() ([]byte, error) {
	if e == nil {
		return nil, ErrNilArgument
	}
	if e.Version != ProtocolVersion {
		return nil, ErrWrongVersion
	}
	if len(e.MsgID) != MessageIDLength {
		return nil, fmt.Errorf("%w: message id", ErrBadFieldLength)
	}
	if len(e.Sender) != IdentityLength {
		return nil, fmt.Errorf("%w: sender identity", ErrBadFieldLength)
	}

	m := map[uint64]any{
		KeyVersion:   e.Version,
		KeyType:      e.Type,
		KeyMsgID:     e.MsgID,
		KeyTimestamp: e.Timestamp,
		KeySender:    e.Sender,
	}
	if e.HasRoom {
		m[KeyRoom] = e.Room
	}
	if e.HasBody {
		m[KeyBody] = e.Body
	}
	if e.HasNick {
		m[KeyNick] = e.Nick
	}
	if e.HasDestination {
		if len(e.Destination) != IdentityLength {
			return nil, fmt.Errorf("%w: destination identity", ErrBadFieldLength)
		}
		m[KeyDestination] = e.Destination
	}
	return cbor.Marshal(m)
}

// UnmarshalEnvelope decodes a CBOR map into an Envelope, ignoring unknown keys.
func UnmarshalEnvelope(data []byte) (*Envelope, error) {
	if len(data) == 0 {
		return nil, ErrInvalidEnvelope
	}
	var raw map[uint64]any
	if err := cbor.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEnvelope, err)
	}

	e := &Envelope{}
	ver, ok := asUint64(raw[KeyVersion])
	if !ok {
		return nil, fmt.Errorf("%w: version", ErrMissingField)
	}
	if ver != ProtocolVersion {
		return nil, ErrWrongVersion
	}
	e.Version = ver

	typ, ok := asUint64(raw[KeyType])
	if !ok {
		return nil, fmt.Errorf("%w: type", ErrMissingField)
	}
	e.Type = typ

	msgID, ok := asBytes(raw[KeyMsgID])
	if !ok {
		return nil, fmt.Errorf("%w: message id", ErrMissingField)
	}
	if len(msgID) != MessageIDLength {
		return nil, fmt.Errorf("%w: message id", ErrBadFieldLength)
	}
	e.MsgID = msgID

	ts, ok := asUint64(raw[KeyTimestamp])
	if !ok {
		return nil, fmt.Errorf("%w: timestamp", ErrMissingField)
	}
	e.Timestamp = ts

	sender, ok := asBytes(raw[KeySender])
	if !ok {
		return nil, fmt.Errorf("%w: sender", ErrMissingField)
	}
	if len(sender) != IdentityLength {
		return nil, fmt.Errorf("%w: sender identity", ErrBadFieldLength)
	}
	e.Sender = sender

	if v, present := raw[KeyRoom]; present {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%w: room", ErrInvalidEnvelope)
		}
		e.Room = s
		e.HasRoom = true
	}
	if v, present := raw[KeyBody]; present {
		e.Body = v
		e.HasBody = true
	}
	if v, present := raw[KeyNick]; present {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%w: nick", ErrInvalidEnvelope)
		}
		e.Nick = s
		e.HasNick = true
	}
	if v, present := raw[KeyDestination]; present {
		dst, ok := asBytes(v)
		if !ok {
			return nil, fmt.Errorf("%w: destination", ErrInvalidEnvelope)
		}
		if len(dst) != IdentityLength {
			return nil, fmt.Errorf("%w: destination identity", ErrBadFieldLength)
		}
		e.Destination = dst
		e.HasDestination = true
	}
	return e, nil
}

func asUint64(v any) (uint64, bool) {
	switch n := v.(type) {
	case uint64:
		return n, true
	case uint32:
		return uint64(n), true
	case uint16:
		return uint64(n), true
	case uint8:
		return uint64(n), true
	case uint:
		return uint64(n), true
	case int64:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int32:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	case int:
		if n < 0 {
			return 0, false
		}
		return uint64(n), true
	default:
		return 0, false
	}
}

func asBytes(v any) ([]byte, bool) {
	b, ok := v.([]byte)
	return b, ok
}

func asString(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}
