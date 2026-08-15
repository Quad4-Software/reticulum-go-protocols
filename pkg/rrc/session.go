// SPDX-License-Identifier: 0BSD
package rrc

import (
	"fmt"
	"log"
	"sync"

	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/packet"
)

// session wraps a single Reticulum Link for RRC framed I/O.
type session struct {
	mu       sync.Mutex
	lnk      *link.Link
	sender   []byte
	nick     string
	closed   bool
	onMsg    MessageHandler
	onClose  func()
	onBad    func(error)
	onBytes  func(n int)
	autoPong bool
}

func newSession(lnk *link.Link, sender []byte, autoPong bool, onMsg MessageHandler, onClose func()) *session {
	s := &session{
		lnk:      lnk,
		sender:   append([]byte(nil), sender...),
		autoPong: autoPong,
		onMsg:    onMsg,
		onClose:  onClose,
	}
	lnk.SetPacketCallback(func(data []byte, _ *packet.Packet) {
		s.handleInbound(data)
	})
	return s
}

func (s *session) setNick(nick string) {
	s.mu.Lock()
	s.nick = SanitizeNick(nick)
	s.mu.Unlock()
}

func (s *session) getNick() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nick
}

func (s *session) sendEnvelope(env *Envelope) error {
	if s == nil || s.lnk == nil {
		return ErrSessionClosed
	}
	s.mu.Lock()
	closed := s.closed
	s.mu.Unlock()
	if closed || !s.lnk.IsActive() {
		return ErrLinkInactive
	}
	raw, err := env.Marshal()
	if err != nil {
		return err
	}
	return s.lnk.SendPacket(raw)
}

func (s *session) sendType(msgType uint64, room string, body any, nick string) error {
	env, err := NewEnvelope(msgType, s.sender)
	if err != nil {
		return err
	}
	if room != "" {
		env.Room = room
		env.HasRoom = true
	}
	if body != nil {
		env.Body = body
		env.HasBody = true
	}
	if nick != "" {
		env.Nick = nick
		env.HasNick = true
	}
	return s.sendEnvelope(env)
}

func (s *session) handleInbound(data []byte) {
	defer func() {
		if r := recover(); r != nil {
			s.reportBad(fmt.Errorf("panic: %v", r))
		}
	}()
	if s.onBytes != nil {
		s.onBytes(len(data))
	}
	env, err := UnmarshalEnvelope(data)
	if err != nil {
		s.reportBad(err)
		return
	}
	if s.autoPong && env.Type == TypePing {
		_ = s.sendType(TypePong, "", env.Body, "")
	}
	s.mu.Lock()
	h := s.onMsg
	s.mu.Unlock()
	if h != nil {
		h(env)
	}
}

func (s *session) reportBad(err error) {
	if err == nil {
		return
	}
	log.Printf("rrc: bad packet: %v", err)
	s.mu.Lock()
	bad := s.onBad
	s.mu.Unlock()
	if bad != nil {
		bad(err)
	}
}

func (s *session) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	cb := s.onClose
	lnk := s.lnk
	s.mu.Unlock()
	if lnk != nil {
		lnk.Teardown()
	}
	if cb != nil {
		cb()
	}
}
