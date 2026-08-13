// SPDX-License-Identifier: 0BSD
package rrc

import (
	"testing"
)

func TestClientRefuseBeforeWelcome(t *testing.T) {
	c := &Client{
		state: ClientAwaitingWelcome,
		rooms: make(map[string]struct{}),
	}
	if err := c.Join("#x"); err != ErrNotWelcome {
		t.Fatalf("Join err = %v", err)
	}
	if err := c.SendMsg("#x", "hi"); err != ErrNotWelcome {
		t.Fatalf("SendMsg err = %v", err)
	}
}

func TestClientSendRequiresMembership(t *testing.T) {
	c := &Client{
		state: ClientActive,
		rooms: make(map[string]struct{}),
		sess:  &session{sender: make([]byte, IdentityLength)},
	}
	if err := c.SendMsg("#lobby", "hi"); err != ErrNotMember {
		t.Fatalf("SendMsg err = %v", err)
	}
}

func TestHubConfigDefaults(t *testing.T) {
	cfg := HubConfig{}
	cfg.applyDefaults()
	if cfg.Limits.MaxMsgBodyBytes != DefaultMaxMsgBodyBytes {
		t.Fatalf("limits = %+v", cfg.Limits)
	}
}

func TestHubRateLimit(t *testing.T) {
	h := &Hub{
		cfg: HubConfig{Limits: HubLimits{RateLimitMsgsPerMinute: 2}},
	}
	p := &hubPeer{}
	if !h.allowRate(p) || !h.allowRate(p) {
		t.Fatal("first two should pass")
	}
	if h.allowRate(p) {
		t.Fatal("third should fail")
	}
}
