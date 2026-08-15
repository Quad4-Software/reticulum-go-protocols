// SPDX-License-Identifier: Apache-2.0
package call

import (
	"context"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/proto"
	"quad4/reticulum-go/pkg/debug"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/packet"
)

func (c *Call) resendAvailableUntilIdentified() {
	for range availableResendCount {
		time.Sleep(availableResendGap)
		if !c.incoming.Load() {
			return
		}
		if c.state.Load() != int32(StateConnecting) {
			return
		}
		if c.status.Load() >= int32(proto.StatusRinging) {
			return
		}
		_ = c.sendSignals(proto.StatusAvailable)
	}
}

func (c *Call) onOutgoingEstablished(l *link.Link) {
	c.setLink(l)
	l.SetPacketCallback(c.onPacket)
	l.SetLinkClosedCallback(c.onLinkClosed)
	c.armTimeout(c.cfg.ConnectTime, func() bool {
		return c.status.Load() < int32(proto.StatusRinging)
	}, "connect timeout")
	go c.identifyIfStillConnecting()
}

func (c *Call) identifyOnce() {
	if c.incoming.Load() || c.cfg.Identity == nil {
		return
	}
	if c.status.Load() >= int32(proto.StatusRinging) {
		return
	}
	l := c.getLink()
	if l == nil || !l.IsActive() {
		return
	}
	if !c.identified.CompareAndSwap(false, true) {
		return
	}
	if err := l.Identify(c.cfg.Identity); err != nil {
		c.identified.Store(false)
		debug.Log(debug.DebugInfo, "call identify failed", "error", err)
	}
}

func (c *Call) identifyIfStillConnecting() {
	time.Sleep(identifyDelay)
	for range identifyRetryCount {
		if c.state.Load() != int32(StateConnecting) {
			return
		}
		if c.status.Load() >= int32(proto.StatusRinging) {
			return
		}
		c.identifyOnce()
		if c.identified.Load() {
			return
		}
		time.Sleep(identifyRetryGap)
	}
}

func (c *Call) onIdentified(_ *link.Link, id *identity.Identity) {
	if !c.incoming.Load() {
		return
	}
	if !c.callerAllowed(id) {
		_ = c.sendSignals(proto.StatusBusy)
		c.end("blocked")
		return
	}
	if c.limiter != nil && id != nil && !c.limiter.Allow(id.Hash()) {
		_ = c.sendSignals(proto.StatusBusy)
		c.end("rate limited")
		return
	}
	if !c.state.CompareAndSwap(int32(StateConnecting), int32(StateRinging)) {
		return
	}
	c.status.Store(int32(proto.StatusRinging))
	_ = c.sendSignals(proto.StatusRinging)
	if c.events.OnRinging != nil {
		c.events.OnRinging(c)
	}
	c.startRingtone()
	if c.cfg.AutoAnswer > 0 {
		go c.autoAnswer()
	}
	c.armTimeout(c.cfg.RingTime, func() bool {
		return c.status.Load() < int32(proto.StatusEstablished)
	}, "ring timeout")
}

func (c *Call) autoAnswer() {
	time.Sleep(c.cfg.AutoAnswer)
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.ConnectTime)
	defer cancel()
	_ = c.Answer(ctx)
}

func (c *Call) onLinkClosed(_ *link.Link) {
	if !c.incoming.Load() && c.state.Load() == int32(StateRinging) && c.events.OnRejected != nil {
		c.events.OnRejected(c)
	}
	c.end("link closed")
}

func (c *Call) onPacket(data []byte, _ *packet.Packet) {
	if len(data) > proto.MaxUnpackBytes {
		return
	}
	msg, err := proto.Unpack(data)
	if err != nil {
		debug.Log(debug.DebugInfo, "lxst unpack failed", "error", err, "bytes", len(data))
		return
	}
	if len(msg.Signals) > 0 {
		c.handleSignals(msg.Signals)
	}
	for _, frame := range msg.Frames {
		c.handleFrame(frame)
	}
}

func (c *Call) handleSignals(signals []int) {
	c.sigMu.Lock()
	defer c.sigMu.Unlock()
	for _, signal := range signals {
		c.handleSignal(signal)
	}
}

func (c *Call) handleSignal(signal int) {
	if c.incoming.Load() && !c.answered.Load() && signal < proto.PreferredMode {
		return
	}
	switch {
	case signal == proto.StatusBusy:
		c.signalBusy()
	case signal == proto.StatusRejected:
		c.signalRejected()
	case signal == proto.StatusAvailable:
		c.status.Store(int32(signal))
		c.identifyOnce()
	case signal == proto.StatusRinging:
		c.signalRinging()
	case signal == proto.StatusConnecting:
		c.signalConnecting()
	case signal == proto.StatusEstablished:
		c.signalEstablished()
	case proto.IsPreferredProfile(signal):
		c.applyPreferredProfile(signal)
	case proto.IsPreferredMode(signal):
		c.applyPreferredMode(signal)
	}
}

func (c *Call) signalBusy() {
	if c.events.OnBusy != nil {
		c.events.OnBusy(c)
	}
	c.startBusyTone()
	c.end("busy")
}

func (c *Call) signalRejected() {
	if c.events.OnRejected != nil {
		c.events.OnRejected(c)
	}
	c.startBusyTone()
	c.end("rejected")
}

func (c *Call) signalRinging() {
	c.status.Store(int32(proto.StatusRinging))
	if c.state.CompareAndSwap(int32(StateConnecting), int32(StateRinging)) || c.state.Load() == int32(StateRinging) {
		c.applyProfile(int(c.profile.Load()))
		_ = c.sendSignals(
			proto.SignalPreferredProfile(int(c.profile.Load())),
			proto.SignalPreferredMode(int(c.callMode.Load())),
		)
		if c.events.OnRinging != nil {
			c.events.OnRinging(c)
		}
		if !c.incoming.Load() {
			c.startDialTone()
		}
	}
}

func (c *Call) signalConnecting() {
	c.status.Store(int32(proto.StatusConnecting))
	if !c.incoming.Load() {
		_ = c.openPipelines()
	}
}

func (c *Call) signalEstablished() {
	if !c.incoming.Load() {
		_ = c.openPipelines()
		_ = c.startPipelines()
		if c.state.CompareAndSwap(int32(StateRinging), int32(StateActive)) ||
			c.state.CompareAndSwap(int32(StateConnecting), int32(StateActive)) {
			c.status.Store(int32(proto.StatusEstablished))
			if c.events.OnAnswered != nil {
				c.events.OnAnswered(c)
			}
		}
		return
	}
	c.status.Store(int32(proto.StatusEstablished))
}

func (c *Call) applyPreferredProfile(signal int) {
	profile := proto.ProfileFromSignal(signal)
	if profile == 0 {
		profile = proto.DefaultProfile
	}
	if c.state.Load() == int32(StateActive) {
		c.switchProfile(profile)
		return
	}
	c.applyProfile(profile)
}

func (c *Call) applyPreferredMode(signal int) {
	if c.modeFollowOff.Load() {
		return
	}
	mode := proto.ModeFromSignal(signal)
	if mode != proto.ModeFullDuplex && mode != proto.ModeHalfDuplex {
		return
	}
	c.callMode.Store(int32(mode))
	if c.state.Load() == int32(StateActive) {
		c.setSquelchState(mode == proto.ModeHalfDuplex)
	} else {
		c.squelched.Store(mode == proto.ModeHalfDuplex)
	}
}

func (c *Call) handleFrame(frame []byte) {
	if c.incoming.Load() && !c.answered.Load() {
		return
	}
	st := c.state.Load()
	if st == int32(StateIdle) || st == int32(StateEnded) {
		return
	}
	if len(frame) < 1 {
		return
	}
	seq := uint16(c.frameSeq.Add(1)) // #nosec G115 -- 16-bit media sequence wraps
	ms := max(time.Now().UnixMilli(), 0)
	c.jitter.Push(seq, uint32(ms), frame) // #nosec G115 -- jitter timestamps are 32-bit and wrap
	c.recvCount.Add(1)
}

func (c *Call) sendSignals(signals ...int) error {
	l := c.getLink()
	if l == nil || !l.IsActive() {
		return ErrNotActive
	}
	for _, s := range signals {
		if proto.IsAutoStatus(s) && s >= int(c.status.Load()) {
			c.status.Store(clampInt32(s))
		}
	}
	payload, err := proto.PackSignalling(signals)
	if err != nil {
		return err
	}
	return l.SendPacket(payload)
}

func (c *Call) applyProfile(profile int) {
	if profile == 0 {
		profile = proto.DefaultProfile
	}
	c.profile.Store(clampInt32(profile))
	params := proto.ProfileParams(profile)
	c.mutex.Lock()
	c.params = params
	c.mutex.Unlock()
	c.jitter.SetTargetMs(params.FrameMs * params.BufferN)
}

func (c *Call) switchProfile(profile int) {
	c.applyProfile(profile)
	if c.state.Load() != int32(StateActive) {
		return
	}
	params := proto.ProfileParams(profile)
	enc, err := newProfileEncoder(params)
	if err != nil {
		return
	}
	dec, err := newProfileDecoder(params)
	if err != nil {
		_ = enc.Close()
		return
	}
	c.mutex.Lock()
	oldEnc := c.encoder
	oldDec := c.decoder
	c.encoder = enc
	c.decoder = dec
	c.params = params
	c.mutex.Unlock()
	if oldEnc != nil {
		_ = oldEnc.Close()
	}
	if oldDec != nil {
		_ = oldDec.Close()
	}
}
