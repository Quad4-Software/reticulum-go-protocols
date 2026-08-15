// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"quad4/msgpack/v5/pkg/msgpack"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/packet"
	"quad4/reticulum-go/pkg/resource"
)

const (
	propagationLinkAttemptTimeout = 45 * time.Second
	pathWaitForPropagation        = 60 * time.Second
	propagationStatusInterval     = 3 * time.Second
)

func (m *Messenger) clearPropagationLink() {
	m.propLinkMu.Lock()
	defer m.propLinkMu.Unlock()
	if m.propLink != nil {
		m.propLink.Teardown()
		m.propLink = nil
		m.propLinkNode = nil
	}
}

func (m *Messenger) ensurePropagationLink(propNodeHash []byte) (*link.Link, error) {
	if m == nil || m.transport == nil {
		return nil, errors.New("lxmf: messenger not initialized")
	}
	if len(propNodeHash) != DestinationLength {
		return nil, fmt.Errorf("propagation node: %w", ErrInvalidHashLength)
	}

	m.propLinkMu.Lock()
	if m.propLink != nil && m.propLink.IsActive() && bytes.Equal(m.propLinkNode, propNodeHash) {
		lnk := m.propLink
		m.propLinkMu.Unlock()
		Verbose("propagation reusing active link", "node", hex.EncodeToString(propNodeHash))
		return lnk, nil
	}
	if m.propLink != nil {
		m.propLink.Teardown()
		m.propLink = nil
		m.propLinkNode = nil
	}
	m.propLinkMu.Unlock()

	return m.establishPropagationLink(propNodeHash)
}

func (m *Messenger) establishPropagationLink(propNodeHash []byte) (*link.Link, error) {
	nodeHex := hex.EncodeToString(propNodeHash)
	hops := m.transport.HopsTo(propNodeHash)
	Info("propagation link start", "node", nodeHex, "hops", hops, "has_path", m.transport.HasPath(propNodeHash))

	if !m.transport.HasPath(propNodeHash) {
		Info("propagation requesting path", "node", nodeHex)
		if err := m.transport.RequestPath(propNodeHash, "", nil, true); err != nil {
			return nil, fmt.Errorf("propagation path request: %w", err)
		}
	}

	pathDeadline := time.Now().Add(pathWaitForPropagation)
	lastPathLog := time.Time{}
	for !m.transport.HasPath(propNodeHash) {
		if time.Now().After(pathDeadline) {
			return nil, fmt.Errorf("propagation node %s: no path within %s", nodeHex, pathWaitForPropagation)
		}
		if time.Since(lastPathLog) >= propagationStatusInterval {
			Verbose("propagation waiting for path", "node", nodeHex, "hops", m.transport.HopsTo(propNodeHash))
			lastPathLog = time.Now()
		}
		time.Sleep(pathPollInterval)
	}
	Info("propagation path ready", "node", nodeHex, "hops", m.transport.HopsTo(propNodeHash))

	pnIdentity, err := identity.Recall(propNodeHash)
	if err != nil {
		return nil, fmt.Errorf("propagation node identity: %w", err)
	}
	if pnIdentity == nil {
		return nil, fmt.Errorf("propagation node %s: %w", nodeHex, ErrDestinationUnknown)
	}

	destOut, err := destination.FromHash(propNodeHash, pnIdentity, destination.Single, m.transport)
	if err != nil {
		return nil, fmt.Errorf("propagation destination: %w", err)
	}

	lnk := link.NewLink(destOut, m.transport, nil, nil, func(closed *link.Link) {
		m.propLinkMu.Lock()
		if m.propLink == closed {
			m.propLink = nil
			m.propLinkNode = nil
		}
		m.propLinkMu.Unlock()
		Verbose("propagation link closed", "node", nodeHex)
	})
	lnk.SetPacketCallback(m.onPropagationSignalling)

	Info("propagation sending link request", "node", nodeHex)
	if err := lnk.Establish(); err != nil {
		return nil, fmt.Errorf("propagation link request: %w", err)
	}
	lnk.Start()

	timeout := propagationLinkAttemptTimeout
	if hops > 0 {
		hopTimeout := time.Duration(link.EstablishmentTimeoutPerHop)*time.Second*time.Duration(hops) + 5*time.Second
		if hopTimeout < timeout {
			timeout = hopTimeout
		}
	}

	waitDeadline := time.Now().Add(timeout)
	lastStatusLog := time.Time{}
	linkID := lnk.GetLinkID()
	Info("propagation waiting for link active", "node", nodeHex, "link_id", hex.EncodeToString(linkID), "timeout", timeout.String())

	for !lnk.IsActive() {
		if time.Now().After(waitDeadline) {
			status := lnk.GetStatus()
			lnk.Teardown()
			return nil, fmt.Errorf("propagation link establishment timeout on %s (status=%d, timeout=%s)", nodeHex, status, timeout)
		}
		if time.Since(lastStatusLog) >= propagationStatusInterval {
			Verbose("propagation link pending",
				"node", nodeHex,
				"link_id", hex.EncodeToString(linkID),
				"status", lnk.GetStatus(),
				"elapsed", time.Since(waitDeadline.Add(-timeout)).Round(time.Second).String(),
			)
			lastStatusLog = time.Now()
		}
		time.Sleep(pathPollInterval)
	}

	Info("propagation link active", "node", nodeHex, "link_id", hex.EncodeToString(linkID), "rtt", lnk.RTT())

	m.propLinkMu.Lock()
	m.propLink = lnk
	m.propLinkNode = append([]byte(nil), propNodeHash...)
	m.propLinkMu.Unlock()
	return lnk, nil
}

func (m *Messenger) onPropagationSignalling(data []byte, _ *packet.Packet) {
	var payload []any
	if err := msgpack.Unmarshal(data, &payload); err != nil || len(payload) < 1 {
		return
	}
	code, ok := signalByte(payload[0])
	if !ok || code != PeerErrorInvalidStamp {
		return
	}
	Warning("propagation upload rejected: invalid stamp")
	m.clearPropagationLink()
}

func signalByte(v any) (byte, bool) {
	switch x := v.(type) {
	case byte:
		return x, true
	case int8:
		if x < 0 {
			return 0, false
		}
		return byte(x), true
	case int:
		if x < 0 || x > 255 {
			return 0, false
		}
		return byte(x), true
	default:
		return 0, false
	}
}

func (m *Messenger) sendPropagationPayload(lnk *link.Link, payload []byte) error {
	if lnk == nil {
		return errors.New("lxmf: nil propagation link")
	}
	if len(payload) == 0 {
		return errors.New("lxmf: empty propagation payload")
	}

	mode := "packet"
	if len(payload) > LinkPacketMaxContent {
		mode = "resource"
	}
	Info("propagation uploading", "bytes", len(payload), "mode", mode)

	if len(payload) <= LinkPacketMaxContent {
		if err := lnk.SendPacket(payload); err != nil {
			return fmt.Errorf("propagation link packet: %w", err)
		}
		Verbose("propagation packet sent", "bytes", len(payload))
		return nil
	}

	res, err := resource.New(payload, true)
	if err != nil {
		return fmt.Errorf("propagation resource: %w", err)
	}
	if err := lnk.SendResource(res); err != nil {
		return fmt.Errorf("propagation link resource: %w", err)
	}

	deadline := time.Now().Add(5 * time.Minute)
	lastLog := time.Time{}
	for {
		status := res.GetStatus()
		switch status {
		case resource.StatusComplete:
			Info("propagation resource complete", "bytes", len(payload))
			return nil
		case resource.StatusFailed, resource.StatusCancelled:
			return errors.New("propagation resource transfer failed")
		}
		if time.Now().After(deadline) {
			return errors.New("propagation resource transfer timeout")
		}
		if time.Since(lastLog) >= propagationStatusInterval {
			Verbose("propagation resource in progress", "status", status, "progress", res.GetProgress())
			lastLog = time.Now()
		}
		time.Sleep(pathPollInterval)
	}
}
