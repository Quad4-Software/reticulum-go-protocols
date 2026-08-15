// SPDX-License-Identifier: 0BSD
package session

import (
	"errors"
	"testing"

	"quad4/reticulum-go-protocols/pkg/lxmf"
	"quad4/reticulum-go/pkg/common"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

func TestUnverifiedDelivered(t *testing.T) {
	s := openTestSession(t, Config{NoRateLimit: true})
	unv := make(chan struct{}, 1)
	got := make(chan struct{}, 1)
	s.events.OnUnverified = func(*lxmf.LXMessage) { unv <- struct{}{} }
	s.events.OnMessage = func(*lxmf.LXMessage) { got <- struct{}{} }
	src := make([]byte, lxmf.DestinationLength)
	src[0] = 0x11
	s.onInbound(&lxmf.LXMessage{SourceHash: src, Title: []byte("t")}, nil)
	select {
	case <-unv:
	default:
		t.Fatal("OnUnverified")
	}
	select {
	case <-got:
	default:
		t.Fatal("OnMessage")
	}
	if s.LastError() != nil {
		t.Fatalf("last error %v", s.LastError())
	}
}

func TestDropUnverifiedFails(t *testing.T) {
	s := openTestSession(t, Config{NoRateLimit: true, DropUnverified: true})
	got := make(chan struct{}, 1)
	s.events.OnMessage = func(*lxmf.LXMessage) { got <- struct{}{} }
	src := make([]byte, lxmf.DestinationLength)
	src[0] = 0x22
	s.onInbound(&lxmf.LXMessage{SourceHash: src}, nil)
	if !errors.Is(s.LastError(), ErrUnverified) {
		t.Fatalf("last error %v", s.LastError())
	}
	select {
	case <-got:
		t.Fatal("OnMessage")
	default:
	}
}

func openTestSession(t *testing.T, cfg Config) *Session {
	t.Helper()
	c := common.DefaultConfig()
	c.ShareInstance = false
	c.InMemoryPathTable = true
	c.InMemoryKnownDestinations = true
	c.ConfigPath = t.TempDir() + "/config"
	tr := transport.NewTransport(c)
	if err := tr.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	id, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Transport = tr
	cfg.Identity = id
	s, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
