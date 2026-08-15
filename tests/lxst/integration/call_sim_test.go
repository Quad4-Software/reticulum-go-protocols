// SPDX-License-Identifier: Apache-2.0
package integration_test

import (
	"context"
	"testing"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/call"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
	"quad4/reticulum-go/pkg/common"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

type simIface struct {
	common.BaseInterface
	peer      *simIface
	inbox     chan []byte
	dropEvery int
	sent      int
}

func newSimIface(name string) *simIface {
	s := &simIface{
		BaseInterface: common.NewBaseInterface(name, common.IFTypeAuto, true),
		inbox:         make(chan []byte, 256),
	}
	s.MTU = common.DefaultMTU
	s.Bitrate = 100000
	s.In = true
	s.Out = true
	s.Enable()
	go func() {
		for data := range s.inbox {
			s.ProcessIncoming(data)
		}
	}()
	return s
}

func (s *simIface) Send(data []byte, _ string) error {
	if s.peer == nil {
		return nil
	}
	s.sent++
	if s.dropEvery > 0 && s.sent%s.dropEvery == 0 {
		return nil
	}
	cp := append([]byte(nil), data...)
	select {
	case s.peer.inbox <- cp:
	default:
	}
	return nil
}

func (s *simIface) ProcessOutgoing(data []byte) error { return s.Send(data, "") }

func wireSim(a, b *simIface) {
	a.peer = b
	b.peer = a
}

func TestSimCallSignaling(t *testing.T) {
	if testing.Short() {
		t.Skip("live mesh integration test")
	}
	cfgA := common.DefaultConfig()
	cfgB := common.DefaultConfig()
	cfgA.ShareInstance = false
	cfgB.ShareInstance = false
	cfgA.InMemoryPathTable = true
	cfgB.InMemoryPathTable = true
	cfgA.InMemoryKnownDestinations = true
	cfgB.InMemoryKnownDestinations = true
	cfgA.ConfigPath = t.TempDir() + "/config"
	cfgB.ConfigPath = t.TempDir() + "/config"
	tA := transport.NewTransport(cfgA)
	tB := transport.NewTransport(cfgB)
	if err := tA.Start(); err != nil {
		t.Fatal(err)
	}
	if err := tB.Start(); err != nil {
		t.Fatal(err)
	}

	ifA := newSimIface("a")
	ifB := newSimIface("b")
	wireSim(ifA, ifB)
	if err := tA.RegisterInterface("a", ifA); err != nil {
		t.Fatal(err)
	}
	if err := tB.RegisterInterface("b", ifB); err != nil {
		t.Fatal(err)
	}

	idA, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	idB, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}

	destB, err := destination.New(idB, destination.In, destination.Single, proto.AppName, tB, proto.AspectName)
	if err != nil {
		t.Fatal(err)
	}
	destB.AcceptsLinks(true)

	incoming := make(chan *call.Call, 1)
	sb := call.NewSwitchboard(tB, call.Config{
		Identity: idB,
		UseAudio: false,
		Events: call.Events{
			OnRinging: func(c *call.Call) { incoming <- c },
		},
	}, nil)
	sb.Bind(destB)

	_ = destB.Announce(false, nil, nil)
	time.Sleep(100 * time.Millisecond)

	caller := call.NewCall(tA, call.Config{Identity: idA, UseAudio: false})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		select {
		case callee := <-incoming:
			_ = callee.Answer(ctx)
		case <-ctx.Done():
		}
	}()

	if err := caller.Dial(ctx, idB); err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if caller.State() != call.StateActive {
		t.Fatalf("expected active call, got %v", caller.State())
	}
	_ = caller.Hangup("test done")
}

func TestSimLossyCall(t *testing.T) {
	if testing.Short() {
		t.Skip("live mesh integration test")
	}
	cfgA := common.DefaultConfig()
	cfgB := common.DefaultConfig()
	cfgA.ShareInstance = false
	cfgB.ShareInstance = false
	cfgA.InMemoryPathTable = true
	cfgB.InMemoryPathTable = true
	cfgA.InMemoryKnownDestinations = true
	cfgB.InMemoryKnownDestinations = true
	cfgA.ConfigPath = t.TempDir() + "/config"
	cfgB.ConfigPath = t.TempDir() + "/config"
	tA := transport.NewTransport(cfgA)
	tB := transport.NewTransport(cfgB)
	if err := tA.Start(); err != nil {
		t.Fatal(err)
	}
	if err := tB.Start(); err != nil {
		t.Fatal(err)
	}

	ifA := newSimIface("a")
	ifB := newSimIface("b")
	ifA.dropEvery = 5
	ifB.dropEvery = 5
	wireSim(ifA, ifB)
	if err := tA.RegisterInterface("a", ifA); err != nil {
		t.Fatal(err)
	}
	if err := tB.RegisterInterface("b", ifB); err != nil {
		t.Fatal(err)
	}

	idA, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	idB, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}

	destB, err := destination.New(idB, destination.In, destination.Single, proto.AppName, tB, proto.AspectName)
	if err != nil {
		t.Fatal(err)
	}
	destB.AcceptsLinks(true)

	incoming := make(chan *call.Call, 1)
	sb := call.NewSwitchboard(tB, call.Config{
		Identity: idB,
		UseAudio: false,
		Events: call.Events{
			OnRinging: func(c *call.Call) { incoming <- c },
		},
	}, nil)
	sb.Bind(destB)

	_ = destB.Announce(false, nil, nil)
	time.Sleep(150 * time.Millisecond)

	caller := call.NewCall(tA, call.Config{Identity: idA, UseAudio: false})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	go func() {
		select {
		case callee := <-incoming:
			_ = callee.Answer(ctx)
		case <-ctx.Done():
		}
	}()

	if err := caller.Dial(ctx, idB); err != nil {
		t.Fatalf("lossy dial failed: %v", err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if caller.RecvFrames() > 0 && caller.State() == call.StateActive {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if caller.State() != call.StateActive {
		t.Fatalf("expected active call on lossy path, got %v", caller.State())
	}
	if caller.RecvFrames() == 0 {
		t.Fatal("expected media frames on lossy path")
	}
	_ = caller.Hangup("lossy done")
}
