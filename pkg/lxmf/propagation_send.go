// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

// AttemptOrder returns reachable propagation nodes to try, preferred first, then by hop count.
func (r *PropagationRegistry) AttemptOrder(tr *transport.Transport, preferred []byte, skip map[string]struct{}, maxAttempts int) []*PropagationNode {
	if r == nil || tr == nil {
		return nil
	}

	reachable := r.reachable(tr)
	if len(reachable) == 0 {
		return nil
	}

	sort.Slice(reachable, func(i, j int) bool {
		if reachable[i].Hops != reachable[j].Hops {
			return reachable[i].Hops < reachable[j].Hops
		}
		return bytes.Compare(reachable[i].Hash, reachable[j].Hash) < 0
	})

	out := make([]*PropagationNode, 0, len(reachable))
	seen := make(map[destID]struct{}, len(reachable))

	appendNode := func(n *PropagationNode) {
		if n == nil {
			return
		}
		id, ok := destIDFrom(n.Hash)
		if !ok {
			return
		}
		if skip != nil {
			if _, hit := skip[hex.EncodeToString(n.Hash)]; hit {
				return
			}
		}
		if _, hit := seen[id]; hit {
			return
		}
		seen[id] = struct{}{}
		copy := *n
		copy.Hash = append([]byte(nil), n.Hash...)
		out = append(out, &copy)
	}

	if len(preferred) == DestinationLength {
		for _, n := range reachable {
			if bytesEqual(n.Hash, preferred) {
				appendNode(n)
				break
			}
		}
	}
	for _, n := range reachable {
		appendNode(n)
	}

	if maxAttempts > 0 && len(out) > maxAttempts {
		out = out[:maxAttempts]
	}
	return out
}

func (m *Messenger) packForPropagation(msg *LXMessage, pnStampCost int) error {
	if msg == nil {
		return errors.New("lxmf: nil message")
	}
	if len(msg.DestinationHash) != DestinationLength {
		return fmt.Errorf("destination: %w", ErrInvalidHashLength)
	}

	remoteIdentity, err := identity.Recall(msg.DestinationHash)
	if err != nil {
		return fmt.Errorf("destination identity not found: %w", err)
	}
	if remoteIdentity == nil {
		return ErrDestinationUnknown
	}

	recipient, err := destination.FromHash(msg.DestinationHash, remoteIdentity, destination.Single, m.transport)
	if err != nil {
		return fmt.Errorf("create recipient destination: %w", err)
	}

	signer := m.dest.GetIdentity()
	if signer == nil {
		return errors.New("lxmf: local destination has no identity")
	}
	if len(msg.Packed) == 0 {
		if _, err := msg.Pack(signer); err != nil {
			return err
		}
	}

	msg.PropagationStamp = nil
	msg.PropagationPacked = nil
	msg.TransientID = nil
	if err := msg.PackPropagated(recipient, pnStampCost); err != nil {
		return err
	}
	return nil
}

// SendPropagatedWithRetry uploads via propagation nodes, trying others when link or transfer fails.
func (m *Messenger) SendPropagatedWithRetry(msg *LXMessage, registry *PropagationRegistry, preferred []byte, maxAttempts int) (*PropagationNode, error) {
	if registry == nil {
		if err := m.SendPropagated(msg, preferred, PropagationStampCostMin); err != nil {
			return nil, err
		}
		return &PropagationNode{Hash: append([]byte(nil), preferred...)}, nil
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	candidates := registry.AttemptOrder(m.transport, preferred, nil, 0)
	if len(candidates) == 0 {
		return nil, ErrNoPropagationNode
	}
	if maxAttempts <= 0 || maxAttempts > len(candidates) {
		maxAttempts = len(candidates)
	}

	Info("propagation send starting", "candidates", len(candidates), "max_attempts", maxAttempts)

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		node := candidates[attempt]
		Info("propagation trying node",
			"attempt", attempt+1,
			"max", maxAttempts,
			"node", hex.EncodeToString(node.Hash),
			"name", node.Name,
			"stamp", node.StampCost,
			"hops", node.Hops,
		)

		if err := m.packForPropagation(msg, int(node.StampCost)); err != nil {
			return nil, err
		}

		m.clearPropagationLink()
		lnk, err := m.establishPropagationLink(node.Hash)
		if err != nil {
			lastErr = err
			Warning("propagation link failed", "node", hex.EncodeToString(node.Hash), "error", err)
			continue
		}

		if err := m.sendPropagationPayload(lnk, msg.PropagationPacked); err != nil {
			lastErr = err
			Warning("propagation transfer failed", "node", hex.EncodeToString(node.Hash), "error", err)
			m.clearPropagationLink()
			continue
		}

		msg.Method = MethodPropagated
		msg.Representation = RepresentationPacket
		if len(msg.PropagationPacked) > LinkPacketMaxContent {
			msg.Representation = RepresentationResource
		}
		msg.State = StateSent
		Info("propagation send succeeded", "node", hex.EncodeToString(node.Hash), "msg_hash", hex.EncodeToString(msg.Hash))
		return node, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("propagation failed after %d attempt(s): %w", maxAttempts, lastErr)
	}
	return nil, ErrNoPropagationNode
}

// SendStampedPropagatedWithRetry is SendPropagatedWithRetry after generating a delivery stamp.
func (m *Messenger) SendStampedPropagatedWithRetry(msg *LXMessage, registry *PropagationRegistry, preferred []byte, stampCost, maxAttempts int) (*PropagationNode, error) {
	if msg == nil {
		return nil, errors.New("lxmf: nil message")
	}
	signer := m.dest.GetIdentity()
	if signer == nil {
		return nil, errors.New("lxmf: local destination has no identity")
	}
	if _, err := msg.Pack(signer); err != nil {
		return nil, fmt.Errorf("pre-pack: %w", err)
	}
	if stampCost > 0 {
		stamp, value, err := generateStampWithLog(msg.Hash, stampCost)
		if err != nil {
			return nil, err
		}
		msg.Stamp = stamp
		msg.StampValue = value
		msg.StampValid = true
		if _, err := msg.Pack(signer); err != nil {
			return nil, fmt.Errorf("re-pack with stamp: %w", err)
		}
	}
	return m.SendPropagatedWithRetry(msg, registry, preferred, maxAttempts)
}
