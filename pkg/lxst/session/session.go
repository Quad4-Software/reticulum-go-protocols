// SPDX-License-Identifier: Apache-2.0

// Package session is a host-PCM telephone API for LXST calls.
//
// Open a Session on an existing Reticulum transport and identity. Feed 48 kHz
// mono PCM16 from the app with PushPCM. PullPCM or Events.OnPCM delivers
// speaker audio, including ring and dial tones. Attach copies the same PCM
// over a framed byte stream for other languages. No local sound device is
// opened.
package session

import (
	"context"
	"errors"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/audio/io"
	"quad4/reticulum-go-protocols/pkg/lxst/call"
	"quad4/reticulum-go-protocols/pkg/lxst/media"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
	"quad4/reticulum-go-protocols/pkg/lxst/rnsnode"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

var (
	ErrNoTransport = errors.New("transport required")
	ErrNoIdentity  = errors.New("identity required")
	ErrNoHost      = errors.New("session has no host pcm device")
	ErrNoCall      = errors.New("no active call")
)

const defaultRecall = 10 * time.Second

type Events struct {
	OnRinging  func(remote *identity.Identity, incoming bool)
	OnAnswered func()
	OnBusy     func()
	OnRejected func()
	OnEnded    func(reason string)
	OnStats    func(media.LinkMetrics)
	OnPCM      func([]int16)
}

type Config struct {
	Transport        *transport.Transport
	Identity         *identity.Identity
	Destination      *destination.Destination
	Device           io.Device
	Profile          int
	Mode             int
	RecallTimeout    time.Duration
	AnnounceInterval time.Duration
	AllowPolicy      byte
	Allowed          [][]byte
	Blocked          [][]byte
	AllowFunc        func([]byte) bool
	Events           Events
}

type Session struct {
	cfg    Config
	phone  *call.Phone
	host   *io.Host
	owned  bool
	recall time.Duration
	events Events
}

func Open(cfg Config) (*Session, error) {
	if cfg.Transport == nil {
		return nil, ErrNoTransport
	}
	if cfg.Identity == nil {
		return nil, ErrNoIdentity
	}
	recall := cfg.RecallTimeout
	if recall <= 0 {
		recall = defaultRecall
	}
	s := &Session{cfg: cfg, recall: recall, events: cfg.Events}
	dev := cfg.Device
	if dev == nil {
		s.host = io.NewHost()
		dev = s.host
		s.owned = true
	} else if h, ok := dev.(*io.Host); ok {
		s.host = h
	}
	if s.host != nil && s.events.OnPCM != nil {
		s.host.SetPlaybackHandler(s.events.OnPCM)
	}
	dest := cfg.Destination
	if dest == nil {
		var err error
		dest, err = destination.New(cfg.Identity, destination.In, destination.Single, proto.AppName, cfg.Transport, proto.AspectName)
		if err != nil {
			return nil, err
		}
	}
	phoneCfg := call.Config{
		Identity:         cfg.Identity,
		Device:           dev,
		UseAudio:         true,
		DuplexIO:         true,
		Profile:          cfg.Profile,
		Mode:             cfg.Mode,
		AnnounceInterval: cfg.AnnounceInterval,
		AllowPolicy:      cfg.AllowPolicy,
		Allowed:          cfg.Allowed,
		Blocked:          cfg.Blocked,
		AllowFunc:        cfg.AllowFunc,
		Events:           s.callEvents(),
	}
	s.phone = call.NewPhone(cfg.Transport, dest, phoneCfg)
	return s, nil
}

func (s *Session) Phone() *call.Phone {
	return s.phone
}

func (s *Session) Host() *io.Host {
	return s.host
}

func (s *Session) DestHash() []byte {
	return s.phone.DestHash()
}

func (s *Session) Active() *call.Call {
	return s.phone.Switchboard().Active()
}

func (s *Session) Announce() error {
	return s.phone.Announce()
}

func (s *Session) StartAnnounce(ctx context.Context) {
	s.phone.StartAnnounce(ctx)
}

func (s *Session) SetBusy(busy bool) {
	s.phone.SetBusy(busy)
}

func (s *Session) SetAllowed(policy byte, hashes [][]byte, fn func([]byte) bool) {
	s.phone.SetAllowed(policy, hashes, fn)
}

func (s *Session) SetBlocked(hashes [][]byte) {
	s.phone.SetBlocked(hashes)
}

func (s *Session) Dial(ctx context.Context, remote *identity.Identity) (*call.Call, error) {
	return s.phone.Dial(ctx, remote)
}

func (s *Session) DialHash(ctx context.Context, destHex string) (*call.Call, error) {
	raw, err := ParseHash(destHex)
	if err != nil {
		return nil, err
	}
	candidates := [][]byte{raw}
	if len(raw) == proto.IdentityHashLen {
		candidates = append(candidates, proto.TelephonyHash(raw))
	}
	remote, err := rnsnode.WaitRecall(s.cfg.Transport, candidates, s.recall)
	if err != nil {
		return nil, err
	}
	return s.Dial(ctx, remote)
}

func (s *Session) Answer(ctx context.Context) error {
	c := s.Active()
	if c == nil {
		return call.ErrNotRinging
	}
	return c.Answer(ctx)
}

func (s *Session) Reject() error {
	c := s.Active()
	if c == nil {
		return call.ErrNotRinging
	}
	return c.Reject("rejected")
}

func (s *Session) Hangup() error {
	c := s.Active()
	if c == nil {
		return nil
	}
	return c.Hangup("hangup")
}

func (s *Session) MuteTX(muted bool) error {
	c := s.Active()
	if c == nil {
		return ErrNoCall
	}
	return c.MuteTX(muted)
}

func (s *Session) MuteRX(muted bool) error {
	c := s.Active()
	if c == nil {
		return ErrNoCall
	}
	return c.MuteRX(muted)
}

func (s *Session) PTT(down bool) error {
	c := s.Active()
	if c == nil {
		return ErrNoCall
	}
	c.PTT(down)
	return nil
}

func (s *Session) SwitchProfile(profile int) error {
	c := s.Active()
	if c == nil {
		return ErrNoCall
	}
	return c.SwitchProfile(profile)
}

func (s *Session) SwitchMode(mode int) error {
	c := s.Active()
	if c == nil {
		return ErrNoCall
	}
	return c.SwitchMode(mode)
}

func (s *Session) PushPCM(pcm []int16) error {
	if s.host == nil {
		return ErrNoHost
	}
	return s.host.Push(pcm)
}

func (s *Session) PushPCMBytes(raw []byte) error {
	if s.host == nil {
		return ErrNoHost
	}
	return s.host.PushBytes(raw)
}

func (s *Session) PullPCM() ([]int16, bool) {
	if s.host == nil {
		return nil, false
	}
	return s.host.Pull()
}

func (s *Session) PullPCMBytes() ([]byte, bool) {
	if s.host == nil {
		return nil, false
	}
	return s.host.PullBytes()
}

func (s *Session) WaitPCM(ctx context.Context) ([]int16, error) {
	if s.host == nil {
		return nil, ErrNoHost
	}
	return s.host.WaitPlayback(ctx)
}

func (s *Session) Close() error {
	_ = s.Hangup()
	s.phone.Stop()
	if s.owned && s.host != nil {
		return s.host.Close()
	}
	return nil
}

func (s *Session) callEvents() call.Events {
	ev := s.events
	return call.Events{
		OnRinging: func(c *call.Call) {
			if ev.OnRinging != nil {
				ev.OnRinging(c.RemoteIdentity(), c.Incoming())
			}
		},
		OnAnswered: func(*call.Call) {
			if ev.OnAnswered != nil {
				ev.OnAnswered()
			}
		},
		OnBusy: func(*call.Call) {
			if ev.OnBusy != nil {
				ev.OnBusy()
			}
		},
		OnRejected: func(*call.Call) {
			if ev.OnRejected != nil {
				ev.OnRejected()
			}
		},
		OnEnded: func(_ *call.Call, reason string) {
			if ev.OnEnded != nil {
				ev.OnEnded(reason)
			}
		},
		OnStats: func(_ *call.Call, m media.LinkMetrics) {
			if ev.OnStats != nil {
				ev.OnStats(m)
			}
		},
	}
}
