// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
)

const (
	deliveryLinkAttemptTimeout = 45 * time.Second
	pathWaitForDelivery        = 60 * time.Second
)

func (m *Messenger) clearDeliveryLink() {
	m.deliveryLinkMu.Lock()
	defer m.deliveryLinkMu.Unlock()
	if m.deliveryLink != nil {
		m.deliveryLink.Teardown()
		m.deliveryLink = nil
		m.deliveryLinkPeer = nil
	}
}

func (m *Messenger) ensureDeliveryLink(peerHash []byte) (*link.Link, error) {
	if m == nil || m.transport == nil {
		return nil, errors.New("lxmf: messenger not initialized")
	}
	if len(peerHash) != DestinationLength {
		return nil, fmt.Errorf("peer: %w", ErrInvalidHashLength)
	}

	m.deliveryLinkMu.Lock()
	if m.deliveryLink != nil && m.deliveryLink.IsActive() && bytes.Equal(m.deliveryLinkPeer, peerHash) {
		lnk := m.deliveryLink
		m.deliveryLinkMu.Unlock()
		return lnk, nil
	}
	if m.deliveryLink != nil {
		m.deliveryLink.Teardown()
		m.deliveryLink = nil
		m.deliveryLinkPeer = nil
	}
	m.deliveryLinkMu.Unlock()

	return m.establishDeliveryLink(peerHash)
}

func (m *Messenger) establishDeliveryLink(peerHash []byte) (*link.Link, error) {
	peerHex := hex.EncodeToString(peerHash)
	hops := m.transport.HopsTo(peerHash)
	Info("delivery link start", "peer", peerHex, "hops", hops, "has_path", m.transport.HasPath(peerHash))

	if !m.transport.HasPath(peerHash) {
		if err := m.transport.RequestPath(peerHash, "", nil, true); err != nil {
			return nil, fmt.Errorf("delivery path request: %w", err)
		}
	}

	pathDeadline := time.Now().Add(pathWaitForDelivery)
	for !m.transport.HasPath(peerHash) {
		if time.Now().After(pathDeadline) {
			return nil, fmt.Errorf("peer %s: no path within %s", peerHex, pathWaitForDelivery)
		}
		time.Sleep(pathPollInterval)
	}

	remoteIdentity, err := identity.Recall(peerHash)
	if err != nil {
		return nil, fmt.Errorf("peer identity: %w", err)
	}
	if remoteIdentity == nil {
		return nil, fmt.Errorf("peer %s: %w", peerHex, ErrDestinationUnknown)
	}

	destOut, err := destination.FromHash(peerHash, remoteIdentity, destination.Single, m.transport)
	if err != nil {
		return nil, fmt.Errorf("delivery destination: %w", err)
	}

	lnk := link.NewLink(destOut, m.transport, nil, nil, func(closed *link.Link) {
		m.deliveryLinkMu.Lock()
		if m.deliveryLink == closed {
			m.deliveryLink = nil
			m.deliveryLinkPeer = nil
		}
		m.deliveryLinkMu.Unlock()
	})

	if err := lnk.Establish(); err != nil {
		return nil, fmt.Errorf("delivery link request: %w", err)
	}
	lnk.Start()

	timeout := deliveryLinkAttemptTimeout
	if hops > 0 {
		hopTimeout := time.Duration(link.EstablishmentTimeoutPerHop)*time.Second*time.Duration(hops) + 5*time.Second
		if hopTimeout < timeout {
			timeout = hopTimeout
		}
	}

	waitDeadline := time.Now().Add(timeout)
	for !lnk.IsActive() {
		if time.Now().After(waitDeadline) {
			status := lnk.GetStatus()
			lnk.Teardown()
			return nil, fmt.Errorf("delivery link timeout on %s (status=%d)", peerHex, status)
		}
		time.Sleep(pathPollInterval)
	}

	m.deliveryLinkMu.Lock()
	m.deliveryLink = lnk
	m.deliveryLinkPeer = append([]byte(nil), peerHash...)
	m.deliveryLinkMu.Unlock()
	return lnk, nil
}

func (m *Messenger) sendDirectPacked(msg *LXMessage) error {
	if msg == nil {
		return errors.New("lxmf: nil message")
	}
	if len(msg.DestinationHash) != DestinationLength {
		return fmt.Errorf("destination: %w", ErrInvalidHashLength)
	}
	if len(msg.Packed) == 0 {
		return errors.New("lxmf: message not packed")
	}

	lnk, err := m.ensureDeliveryLink(msg.DestinationHash)
	if err != nil {
		return err
	}
	if err := m.sendPropagationPayload(lnk, msg.Packed); err != nil {
		return err
	}

	msg.Method = MethodDirect
	if len(msg.Packed) > LinkPacketMaxContent {
		msg.Representation = RepresentationResource
	} else {
		msg.Representation = RepresentationPacket
	}
	msg.State = StateSent
	return nil
}

// SendDirect packs, signs, and sends via an RNS link (packet or resource).
func (m *Messenger) SendDirect(msg *LXMessage) error {
	if msg == nil {
		return errors.New("lxmf: nil message")
	}
	signer := m.dest.GetIdentity()
	if signer == nil {
		return errors.New("lxmf: local destination has no identity")
	}
	if _, err := msg.Pack(signer); err != nil {
		return err
	}
	return m.sendDirectPacked(msg)
}

// SendStampedDirect is SendDirect after generating a delivery stamp.
func (m *Messenger) SendStampedDirect(msg *LXMessage, stampCost int) error {
	if msg == nil {
		return errors.New("lxmf: nil message")
	}
	if stampCost <= 0 {
		return m.SendDirect(msg)
	}
	signer := m.dest.GetIdentity()
	if signer == nil {
		return errors.New("lxmf: local destination has no identity")
	}
	if _, err := msg.Pack(signer); err != nil {
		return fmt.Errorf("pre-pack: %w", err)
	}
	stamp, value, err := generateStampWithLog(msg.Hash, stampCost)
	if err != nil {
		return fmt.Errorf("stamp generation: %w", err)
	}
	msg.Stamp = stamp
	msg.StampValue = value
	msg.StampValid = true
	if _, err := msg.Pack(signer); err != nil {
		return fmt.Errorf("re-pack with stamp: %w", err)
	}
	return m.sendDirectPacked(msg)
}

// NeedsDirectDelivery reports whether opportunistic send is likely to fail.
func NeedsDirectDelivery(msg *LXMessage) bool {
	if msg == nil {
		return false
	}
	if len(msg.Packed) == 0 {
		return false
	}
	contentSize, err := msg.ContentSize()
	if err != nil {
		return true
	}
	limit, ok := MaxContentForMethod(MethodOpportunistic, DestinationTypeSingle)
	if !ok {
		return true
	}
	return contentSize > limit
}
