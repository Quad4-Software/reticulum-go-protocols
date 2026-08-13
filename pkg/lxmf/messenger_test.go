// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sync"
	"testing"
	"time"

	"quad4/reticulum-go/pkg/common"
	"quad4/reticulum-go/pkg/cryptography"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/interfaces"
	"quad4/reticulum-go/pkg/packet"
	"quad4/reticulum-go/pkg/transport"
)

const (
	udp1              = "UDPL1"
	udp2              = "UDPL2"
	loopback          = "127.0.0.1:0"
	pathEstablishWait = 10 * time.Second
)

func TestMessenger_Compose(t *testing.T) {
	cfg := common.DefaultConfig()
	tr := transport.NewTransport(cfg)
	id, err := identity.NewIdentity()
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	dest, err := NewDeliveryDestination(id, tr)
	if err != nil {
		t.Fatalf("destination: %v", err)
	}
	m := NewMessenger(tr, dest)

	remote := make([]byte, DestinationLength)
	for i := range remote {
		remote[i] = byte(i + 1)
	}

	msg, err := m.Compose(remote, "hi", "there", nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !bytes.Equal(msg.SourceHash, m.DestinationHash()) {
		t.Errorf("source hash mismatch")
	}
	if msg.TitleString() != "hi" || msg.ContentString() != "there" {
		t.Errorf("title/content mismatch: %q / %q", msg.TitleString(), msg.ContentString())
	}
}

func TestMessenger_Send_RejectsUnknownDestination(t *testing.T) {
	cfg := common.DefaultConfig()
	tr := transport.NewTransport(cfg)
	id, _ := identity.NewIdentity()
	m, err := NewDeliveryMessenger(id, tr)
	if err != nil {
		t.Fatalf("messenger: %v", err)
	}

	remote := make([]byte, DestinationLength)
	if _, err := m.SendText(remote, "", "x"); err == nil {
		t.Fatal("expected error for unknown destination identity")
	}
}

func TestMessenger_SetSourceResolver(t *testing.T) {
	cfg := common.DefaultConfig()
	tr := transport.NewTransport(cfg)
	id, _ := identity.NewIdentity()
	m, err := NewDeliveryMessenger(id, tr)
	if err != nil {
		t.Fatalf("messenger: %v", err)
	}

	called := false
	m.SetSourceResolver(func(_ []byte) (*identity.Identity, error) {
		called = true
		return nil, errors.New("forced")
	})

	if m.resolver == nil {
		t.Fatal("resolver not set")
	}
	if _, err := m.resolver(nil); err == nil || !called {
		t.Fatal("custom resolver was not used")
	}

	m.SetSourceResolver(nil)
	if m.resolver == nil {
		t.Fatal("nil resolver should restore default")
	}
}

func TestMessenger_TwoWayLoopback(t *testing.T) {
	if testing.Short() {
		t.Skip("loopback messenger test skipped in -short mode")
	}

	cfg1 := common.DefaultConfig()
	cfg1.Interfaces = map[string]*common.InterfaceConfig{
		udp1: {Type: "UDPInterface", Enabled: true, Address: loopback, TargetHost: loopback, Name: udp1},
	}
	cfg2 := common.DefaultConfig()
	cfg2.Interfaces = map[string]*common.InterfaceConfig{
		udp2: {Type: "UDPInterface", Enabled: true, Address: loopback, TargetHost: loopback, Name: udp2},
	}

	tr1 := transport.NewTransport(cfg1)
	tr2 := transport.NewTransport(cfg2)
	if err := tr1.Start(); err != nil {
		t.Fatalf("tr1.Start: %v", err)
	}
	defer tr1.Close()
	if err := tr2.Start(); err != nil {
		t.Fatalf("tr2.Start: %v", err)
	}
	defer tr2.Close()

	addr1 := "127.0.0.1:42440"
	addr2 := "127.0.0.1:42441"

	var iface1 interfaces.Interface
	iface1, err := interfaces.NewUDPInterface(udp1, addr1, addr2, true)
	if err != nil {
		t.Fatalf("iface1: %v", err)
	}
	iface1.SetPacketCallback(func(d []byte, ni common.NetworkInterface) { tr1.HandlePacket(d, ni) })
	if err := iface1.Start(); err != nil {
		t.Fatalf("iface1.Start: %v", err)
	}
	defer iface1.Stop()
	if ni, ok := iface1.(common.NetworkInterface); ok {
		if err := tr1.RegisterInterface(udp1, ni); err != nil {
			t.Fatalf("register iface1: %v", err)
		}
	}

	var iface2 interfaces.Interface
	iface2, err = interfaces.NewUDPInterface(udp2, addr2, addr1, true)
	if err != nil {
		t.Fatalf("iface2: %v", err)
	}
	iface2.SetPacketCallback(func(d []byte, ni common.NetworkInterface) { tr2.HandlePacket(d, ni) })
	if err := iface2.Start(); err != nil {
		t.Fatalf("iface2.Start: %v", err)
	}
	defer iface2.Stop()
	if ni, ok := iface2.(common.NetworkInterface); ok {
		if err := tr2.RegisterInterface(udp2, ni); err != nil {
			t.Fatalf("register iface2: %v", err)
		}
	}

	id1, _ := identity.NewIdentity()
	id2, _ := identity.NewIdentity()

	dest1, err := NewDeliveryDestination(id1, tr1)
	if err != nil {
		t.Fatalf("dest1: %v", err)
	}
	dest2, err := NewDeliveryDestination(id2, tr2)
	if err != nil {
		t.Fatalf("dest2: %v", err)
	}

	identity.Remember(nil, dest1.GetHash(), id1.GetPublicKey(), nil)
	identity.Remember(nil, dest2.GetHash(), id2.GetPublicKey(), nil)

	m1 := NewMessenger(tr1, dest1)
	m2 := NewMessenger(tr2, dest2)

	var wg sync.WaitGroup
	wg.Add(2)
	var got1, got2 *LXMessage
	m1.SetMessageHandler(func(msg *LXMessage, _ common.NetworkInterface) {
		got1 = msg
		wg.Done()
	})
	m2.SetMessageHandler(func(msg *LXMessage, _ common.NetworkInterface) {
		got2 = msg
		wg.Done()
	})

	if err := dest1.Announce(false, nil, nil); err != nil {
		t.Fatalf("announce 1: %v", err)
	}
	if err := dest2.Announce(false, nil, nil); err != nil {
		t.Fatalf("announce 2: %v", err)
	}

	deadline := time.Now().Add(pathEstablishWait)
	for {
		if tr1.HasPath(dest2.GetHash()) && tr2.HasPath(dest1.GetHash()) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout establishing paths")
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("paths established d1=%s d2=%s", hex.EncodeToString(dest1.GetHash()), hex.EncodeToString(dest2.GetHash()))

	if _, err := m1.SendText(dest2.GetHash(), "from1", "hello two"); err != nil {
		t.Fatalf("send 1->2: %v", err)
	}
	if _, err := m2.SendText(dest1.GetHash(), "from2", "hello one"); err != nil {
		t.Fatalf("send 2->1: %v", err)
	}

	doneCh := make(chan struct{})
	go func() { wg.Wait(); close(doneCh) }()
	select {
	case <-doneCh:
	case <-time.After(pathEstablishWait):
		t.Fatalf("timeout waiting for inbound messages")
	}

	if got1 == nil || got1.ContentString() != "hello one" {
		t.Errorf("messenger 1 received: %#v", got1)
	}
	if got2 == nil || got2.ContentString() != "hello two" {
		t.Errorf("messenger 2 received: %#v", got2)
	}
	if got1 != nil && !got1.SignatureValidated {
		t.Errorf("inbound 1 signature not validated: reason=%d", got1.UnverifiedReason)
	}
	if got2 != nil && !got2.SignatureValidated {
		t.Errorf("inbound 2 signature not validated: reason=%d", got2.UnverifiedReason)
	}
}

func TestMessenger_RatchetEncryptedInbound(t *testing.T) {
	cfg := common.DefaultConfig()
	tr := transport.NewTransport(cfg)
	id, err := identity.NewIdentity()
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	dest, err := NewDeliveryDestination(id, tr)
	if err != nil {
		t.Fatalf("destination: %v", err)
	}
	m := NewMessenger(tr, dest)

	ratchetPriv := id.GetCurrentRatchetKey()
	if ratchetPriv == nil {
		t.Fatal("expected identity ratchet key")
	}
	ratchetPub, err := cryptography.PublicKeyFromPrivate(ratchetPriv)
	if err != nil {
		t.Fatalf("ratchet public key: %v", err)
	}

	remote := make([]byte, DestinationLength)
	msg, err := m.Compose(remote, "ratchet", "encrypted inbound", nil)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if _, err := msg.Pack(id); err != nil {
		t.Fatalf("Pack: %v", err)
	}
	inner, err := msg.EncryptedPayload()
	if err != nil {
		t.Fatalf("EncryptedPayload: %v", err)
	}

	senderView := identity.FromPublicKey(id.GetPublicKey())
	ciphertext, err := senderView.Encrypt(inner, ratchetPub)
	if err != nil {
		t.Fatalf("Encrypt with ratchet: %v", err)
	}

	done := make(chan *LXMessage, 1)
	m.SetMessageHandler(func(msg *LXMessage, _ common.NetworkInterface) {
		done <- msg
	})

	pkt := packet.NewPacket(
		packet.DestinationSingle,
		ciphertext,
		packet.PacketTypeData,
		packet.ContextNone,
		packet.PropagationBroadcast,
		packet.HeaderType1,
		nil,
		true,
		packet.FlagUnset,
	)
	m.Receive(pkt, nil)

	select {
	case got := <-done:
		if got.ContentString() != "encrypted inbound" {
			t.Errorf("content = %q", got.ContentString())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ratchet-encrypted inbound message")
	}
}
