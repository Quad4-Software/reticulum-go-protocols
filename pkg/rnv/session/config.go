// SPDX-License-Identifier: 0BSD
package session

import (
	"time"

	"quad4/reticulum-go-protocols/pkg/rnv"
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

// Config configures an RNV endpoint. Prefer SafeConfig.
type Config struct {
	Caps                 proto.Caps
	DialTimeout          time.Duration
	HelloTimeout         time.Duration
	StillTimeout         time.Duration
	ClipTimeout          time.Duration
	AllowParallelLXST    bool
	StrictExtensions     bool
	DangerousRaiseLimits bool
	CustomLimits         Limits
	MaxStillsPerMinute   int
	MaxClipsPerMinute    int
	LXSTActive           func(peerHash []byte) bool
	Handlers             Handlers
	Registry             *proto.CodecRegistry
}

// Limits tighten absolute package caps.
type Limits struct {
	MaxStillBytes uint64
	MaxClipBytes  uint64
}

// Handlers receive inbound media and control events.
type Handlers struct {
	OnConn      func(*Conn)
	OnStill     func(*Conn, proto.StillMeta, []byte)
	OnClip      func(*Conn, proto.ClipMeta, []byte)
	OnStream    func(*Conn, proto.StreamOffer)
	OnVideo     func(*Conn, proto.Frame)
	OnAudio     func(*Conn, proto.Frame)
	OnReject    func(*Conn, proto.RejectBody)
	OnExtension func(*Conn, uint64, []byte)
	OnBye       func(*Conn)
	OnCtrl      func(*Conn, proto.CtrlBody)
}

// SafeConfig returns Low-preferred defaults with sane caps and no auto-announce.
func SafeConfig() Config {
	return Config{
		Caps:               proto.DefaultCaps(),
		DialTimeout:        45 * time.Second,
		HelloTimeout:       20 * time.Second,
		StillTimeout:       2 * time.Minute,
		ClipTimeout:        10 * time.Minute,
		AllowParallelLXST:  false,
		MaxStillsPerMinute: 30,
		MaxClipsPerMinute:  6,
		Registry:           proto.DefaultRegistry,
	}
}

func (c Config) withDefaults() Config {
	d := SafeConfig()
	if c.Caps.MaxStill == 0 && c.Caps.Preferred == 0 && len(c.Caps.Profiles) == 0 {
		c.Caps = d.Caps
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = d.DialTimeout
	}
	if c.HelloTimeout <= 0 {
		c.HelloTimeout = d.HelloTimeout
	}
	if c.StillTimeout <= 0 {
		c.StillTimeout = d.StillTimeout
	}
	if c.ClipTimeout <= 0 {
		c.ClipTimeout = d.ClipTimeout
	}
	if c.MaxStillsPerMinute <= 0 {
		c.MaxStillsPerMinute = d.MaxStillsPerMinute
	}
	if c.MaxClipsPerMinute <= 0 {
		c.MaxClipsPerMinute = d.MaxClipsPerMinute
	}
	if c.Registry == nil {
		c.Registry = d.Registry
	}
	c.Caps.StrictExt = c.StrictExtensions || c.Caps.StrictExt
	c.applyLimitGuards()
	return c
}

func (c *Config) applyLimitGuards() {
	maxStill := uint64(rnv.MaxStillBytes)
	maxClip := uint64(rnv.MaxClipBytes)
	if c.CustomLimits.MaxStillBytes > 0 {
		if c.CustomLimits.MaxStillBytes > maxStill && !c.DangerousRaiseLimits {
			c.CustomLimits.MaxStillBytes = maxStill
		}
		c.Caps.MaxStill = c.CustomLimits.MaxStillBytes
	}
	if c.CustomLimits.MaxClipBytes > 0 {
		if c.CustomLimits.MaxClipBytes > maxClip && !c.DangerousRaiseLimits {
			c.CustomLimits.MaxClipBytes = maxClip
		}
		c.Caps.MaxClip = c.CustomLimits.MaxClipBytes
	}
	if c.Caps.MaxStill == 0 || (c.Caps.MaxStill > maxStill && !c.DangerousRaiseLimits) {
		c.Caps.MaxStill = maxStill
	}
	if c.Caps.MaxClip == 0 || (c.Caps.MaxClip > maxClip && !c.DangerousRaiseLimits) {
		c.Caps.MaxClip = maxClip
	}
}

func cloneHash(h []byte) []byte {
	if len(h) == 0 {
		return nil
	}
	out := make([]byte, len(h))
	copy(out, h)
	return out
}
