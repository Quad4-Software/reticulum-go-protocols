// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"quad4/msgpack/v5/pkg/msgpack"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/packet"
	"quad4/reticulum-go/pkg/resource"
)

const (
	pnStampThrottleSeconds = 180
	maxInboundSyncsDefault = 3
)

func (r *Router) propagationLinkEstablished(v any) {
	lnk, ok := v.(*link.Link)
	if !ok || lnk == nil {
		return
	}
	_ = lnk.SetResourceStrategy(link.AcceptApp)
	lnk.SetPacketCallback(func(data []byte, pkt *packet.Packet) {
		r.handlePropagationPacket(data, pkt, lnk)
	})
	lnk.SetResourceCallback(func(adv any) bool {
		return r.propagationResourceAdvertised(adv, lnk)
	})
	lnk.SetResourceConcludedCallback(func(res any) {
		r.propagationResourceConcluded(res, lnk)
	})
}

func (r *Router) handlePropagationPacket(data []byte, pkt *packet.Packet, lnk *link.Link) {
	defer func() {
		if rec := recover(); rec != nil {
			Error("propagation packet panic", "error", rec)
		}
	}()

	var payload []any
	if err := msgpack.Unmarshal(data, &payload); err != nil || len(payload) < 2 {
		return
	}
	messages, ok := payload[1].([]any)
	if !ok {
		return
	}
	raw := make([][]byte, 0, len(messages))
	for _, m := range messages {
		b, ok := m.([]byte)
		if ok && len(b) > 0 {
			raw = append(raw, b)
		}
	}

	if len(raw) == 0 {
		return
	}

	minCost := max(0, r.cfg.Propagation.PropagationStampCostTarget-r.cfg.Propagation.PropagationStampCostFlexibility)
	validated := ValidatePNStamps(raw, minCost)

	for _, entry := range validated {
		r.lxmfPropagation(entry.LxmData, int64(entry.Value), entry.Stamp, nil)
		r.clientPropagationReceived++
	}

	if len(validated) == len(raw) {
		if lnk != nil && pkt != nil {
			if err := lnk.ProvePacket(pkt); err != nil {
				Warning("propagation client packet proof failed", "error", err)
			} else {
				Verbose("propagation client packet proof sent", "messages", len(raw))
			}
		} else {
			Warning("propagation client transfer missing proof context", "has_link", lnk != nil, "has_packet", pkt != nil)
		}
		return
	}
	Warning("propagation client transfer had invalid stamps", "valid", len(validated), "total", len(raw))
	if lnk != nil {
		reject, _ := msgpack.Marshal([]any{PeerErrorInvalidStamp})
		_ = lnk.SendPacket(reject)
		lnk.Teardown()
	}
}

func (r *Router) propagationResourceAdvertised(adv any, lnk *link.Link) bool {
	if r.cfg.Propagation.FromStaticOnly {
		remote := lnk.GetRemoteIdentity()
		if remote == nil {
			return false
		}
		propHash, err := PropagationDestinationHash(remote)
		if err != nil || !r.isStaticPeer(propHash) {
			return false
		}
	}
	var size int64
	switch a := adv.(type) {
	case *resource.ResourceAdvertisement:
		size = a.DataSize
	}
	limit := int64(r.cfg.Propagation.PropagationSyncMaxAcceptedKB * 1000)
	if limit > 0 && size > limit {
		return false
	}
	linkID := hex.EncodeToString(lnk.GetLinkID())
	r.propMu.Lock()
	if state, ok := r.acceptedOfferLinks[linkID]; ok && state >= OfferAccepted {
		r.acceptedOfferLinks[linkID] = OfferTransferring
	}
	r.propMu.Unlock()
	return true
}

func (r *Router) propagationResourceConcluded(res any, lnk *link.Link) {
	data := extractResourceData(res)
	if len(data) == 0 {
		return
	}
	var payload []any
	if err := msgpack.Unmarshal(data, &payload); err != nil || len(payload) < 2 {
		return
	}
	messages, ok := payload[1].([]any)
	if !ok {
		return
	}

	remote := lnk.GetRemoteIdentity()
	var remotePropHash []byte
	if remote != nil {
		remotePropHash, _ = PropagationDestinationHash(remote)
	}
	r.tryAutopeerFromIncomingSync(remotePropHash)

	linkID := hex.EncodeToString(lnk.GetLinkID())
	r.propMu.Lock()
	peeringValidated := r.validatedPeerLinks[linkID]
	r.propMu.Unlock()

	raw := make([][]byte, 0, len(messages))
	for _, m := range messages {
		b, ok := m.([]byte)
		if ok && len(b) > 0 {
			raw = append(raw, b)
		}
	}
	if len(raw) == 0 {
		return
	}

	if !peeringValidated && len(raw) > 1 {
		Warning("rejecting multi-message propagation without peering key", "peer", hex.EncodeToString(remotePropHash))
		lnk.Teardown()
		return
	}

	bypassSeq := !r.cfg.Propagation.FromStaticOnly && remotePropHash != nil && r.isStaticPeer(remotePropHash)
	if !bypassSeq && r.cfg.Propagation.EnableNode {
		r.propMu.Lock()
		if remotePropHash != nil {
			r.validatingFrom[peerKey(remotePropHash)] = float64(time.Now().Unix())
		}
		r.propMu.Unlock()
	}

	minCost := max(0, r.cfg.Propagation.PropagationStampCostTarget-r.cfg.Propagation.PropagationStampCostFlexibility)
	validated := ValidatePNStamps(raw, minCost)

	var fromPeer *Peer
	if remotePropHash != nil {
		r.peersMu.RLock()
		fromPeer = r.peers[peerKey(remotePropHash)]
		r.peersMu.RUnlock()
	}

	for _, entry := range validated {
		r.lxmfPropagation(entry.LxmData, int64(entry.Value), entry.Stamp, fromPeer)
		if fromPeer != nil {
			fromPeer.Incoming++
			fromPeer.RXBytes += int64(len(entry.LxmData))
			fromPeer.queueHandled(entry.TransientID)
		} else if remote != nil {
			r.unpeeredPropagationIncoming++
			r.unpeeredPropagationRxBytes += int64(len(entry.LxmData))
		} else {
			r.clientPropagationReceived++
		}
	}

	r.propMu.Lock()
	if remotePropHash != nil {
		delete(r.validatingFrom, peerKey(remotePropHash))
	}
	r.propMu.Unlock()

	invalidCount := len(raw) - len(validated)
	if invalidCount > 0 {
		Warning("propagation resource had invalid stamps", "valid", len(validated), "total", len(raw), "peer", hex.EncodeToString(remotePropHash))
		if remotePropHash != nil {
			r.throttlePeer(remotePropHash)
		}
		lnk.Teardown()
	}
}

func (r *Router) offerRequestHandler(path string, data []byte, requestID, linkID []byte, remoteIdentity *identity.Identity, requestedAt int64) any {
	_ = path
	_ = requestID
	_ = requestedAt

	if remoteIdentity == nil {
		return []byte{PeerErrorNoIdentity}
	}
	remotePropHash, err := PropagationDestinationHash(remoteIdentity)
	if err != nil {
		return []byte{PeerErrorInvalidData}
	}
	remoteKey := peerKey(remotePropHash)
	linkKey := hex.EncodeToString(linkID)

	if r.throttled(remoteKey) {
		return []byte{PeerErrorThrottled}
	}

	bypassSeq := r.isStaticPeer(remotePropHash)
	r.propMu.Lock()
	seqBusy := len(r.validatingFrom) > 0
	syncBusy := r.propagationResourcesTransferring() >= maxInboundSyncs(r)
	r.propMu.Unlock()

	if !bypassSeq && seqBusy {
		return []byte{PeerErrorThrottled}
	}
	if !bypassSeq && syncBusy {
		return []byte{PeerErrorThrottled}
	}

	if r.cfg.Propagation.FromStaticOnly && !r.isStaticPeer(remotePropHash) {
		return []byte{PeerErrorNoAccess}
	}

	var req []any
	if err := msgpack.Unmarshal(data, &req); err != nil || len(req) < 2 {
		return []byte{PeerErrorInvalidData}
	}
	peeringKey, ok := req[0].([]byte)
	if !ok {
		return []byte{PeerErrorInvalidData}
	}
	transientIDs, ok := req[1].([]any)
	if !ok {
		return []byte{PeerErrorInvalidData}
	}

	peeringID := append(append([]byte(nil), r.identity.Hash()...), remoteIdentity.Hash()...)
	if !ValidatePeeringKey(peeringID, peeringKey, r.cfg.Propagation.PeeringCost) {
		return []byte{PeerErrorInvalidKey}
	}

	r.propMu.Lock()
	r.validatedPeerLinks[linkKey] = true
	r.propMu.Unlock()

	wanted := make([][]byte, 0)
	for _, tidAny := range transientIDs {
		tid, ok := tidAny.([]byte)
		if !ok || len(tid) == 0 {
			continue
		}
		if _, ok := r.store.Get(tid); !ok {
			wanted = append(wanted, tid)
		}
	}

	if len(wanted) == 0 {
		return false
	}
	if len(wanted) == len(transientIDs) {
		r.propMu.Lock()
		r.acceptedOfferLinks[linkKey] = OfferAccepted
		r.propMu.Unlock()
		return true
	}
	r.propMu.Lock()
	r.acceptedOfferLinks[linkKey] = OfferAccepted
	r.propMu.Unlock()
	return wanted
}

func (r *Router) messageGetRequestHandler(path string, data []byte, requestID, linkID []byte, remoteIdentity *identity.Identity, requestedAt int64) any {
	_ = path
	_ = requestID
	_ = linkID
	_ = requestedAt

	if remoteIdentity == nil {
		return []byte{PeerErrorNoIdentity}
	}
	if !r.identityAllowed(remoteIdentity) {
		return []byte{PeerErrorNoAccess}
	}

	deliveryHash, err := deliveryDestinationHash(remoteIdentity)
	if err != nil {
		return []byte{PeerErrorInvalidData}
	}

	var req []any
	if err := msgpack.Unmarshal(data, &req); err != nil || len(req) < 2 {
		return []byte{PeerErrorInvalidData}
	}

	wants := decodeByteList(req[0])
	haves := decodeByteList(req[1])
	var transferLimitKB float64
	if len(req) >= 3 {
		transferLimitKB = asFloat(req[2])
	}

	if len(wants) == 0 && len(haves) == 0 {
		return r.store.ListForDestination(deliveryHash)
	}

	for _, tid := range haves {
		ent, ok := r.store.Get(tid)
		if !ok {
			continue
		}
		if !bytes.Equal(ent.DestinationHash, deliveryHash) {
			continue
		}
		_ = r.store.Remove(tid)
	}

	if len(wants) == 0 {
		return []any{}
	}

	limitBytes := int64(transferLimitKB * 1000)
	const perMessageOverhead = 16
	cumulative := int64(24)
	response := make([][]byte, 0)
	for _, tid := range wants {
		ent, ok := r.store.Get(tid)
		if !ok || !bytes.Equal(ent.DestinationHash, deliveryHash) {
			continue
		}
		lxm, err := r.store.ReadLXMData(tid)
		if err != nil {
			continue
		}
		next := cumulative + int64(len(lxm)+perMessageOverhead)
		if limitBytes > 0 && next > limitBytes {
			continue
		}
		response = append(response, lxm)
		cumulative = next
	}
	r.clientPropagationServed += int64(len(response))
	return response
}

func (r *Router) lxmfPropagation(lxmData []byte, stampValue int64, stampData []byte, fromPeer *Peer) bool {
	if len(lxmData) < Overhead {
		return false
	}
	sum := sha256Sum(lxmData)
	tidKey := peerKey(sum[:])

	r.processedMu.Lock()
	if _, seen := r.locallyProcessed[tidKey]; seen {
		r.processedMu.Unlock()
		return false
	}
	r.locallyProcessed[tidKey] = float64(time.Now().Unix())
	r.processedMu.Unlock()

	destHash := lxmData[:DestinationLength]
	localDelivery := false

	r.mu.RLock()
	deliveryDest := r.deliveryDest
	r.mu.RUnlock()

	if deliveryDest != nil {
		localHash := deliveryDest.GetHash()
		if bytes.Equal(destHash, localHash) {
			decrypted, err := deliveryDest.Decrypt(lxmData[DestinationLength:])
			if err == nil && len(decrypted) > 0 {
				payload := append(append([]byte(nil), destHash...), decrypted...)
				r.handleDeliveryPayload(payload, MethodPropagated, true, false)
				r.deliveredMu.Lock()
				r.locallyDelivered[tidKey] = float64(time.Now().Unix())
				r.deliveredMu.Unlock()
				localDelivery = true
			}
		}
	}

	if localDelivery {
		return true
	}

	if !r.propagationEnabled || r.store == nil {
		return false
	}

	entry, err := r.store.Add(lxmData, stampData, stampValue)
	if err != nil {
		Warning("propagation store add failed", "error", err)
		return false
	}
	r.enqueuePeerDistribution(sum[:], fromPeer)
	Verbose("stored propagated message", "transient", tidKey, "stamp", stampValue, "path", entry.FilePath)
	return true
}

func (r *Router) identityAllowed(id *identity.Identity) bool {
	if !r.cfg.Propagation.AuthRequired {
		return true
	}
	_, ok := r.allowedList[peerKey(id.Hash())]
	return ok
}

func (r *Router) throttled(peerKey string) bool {
	r.propMu.Lock()
	defer r.propMu.Unlock()
	until, ok := r.throttledPeers[peerKey]
	if !ok {
		return false
	}
	if float64(time.Now().Unix()) >= until {
		delete(r.throttledPeers, peerKey)
		return false
	}
	return true
}

func (r *Router) throttlePeer(destHash []byte) {
	if len(destHash) != DestinationLength {
		return
	}
	key := peerKey(destHash)
	r.propMu.Lock()
	r.throttledPeers[key] = float64(time.Now().Unix()) + pnStampThrottleSeconds
	r.propMu.Unlock()
}

func (r *Router) propagationResourcesTransferring() int {
	count := 0
	for _, state := range r.acceptedOfferLinks {
		if state > OfferAccepted {
			count++
		}
	}
	return count
}

func maxInboundSyncs(r *Router) int {
	if r.cfg.Propagation.MaxPeers > 0 {
		return maxInboundSyncsDefault
	}
	return maxInboundSyncsDefault
}

func (r *Router) enqueuePeerDistribution(transientID []byte, fromPeer *Peer) {
	r.propMu.Lock()
	r.peerDistributionQ = append(r.peerDistributionQ, peerDistEntry{
		transientID: append([]byte(nil), transientID...),
		fromPeer:    fromPeer,
	})
	r.propMu.Unlock()
}

func (r *Router) flushPeerDistribution() {
	r.propMu.Lock()
	queue := r.peerDistributionQ
	r.peerDistributionQ = nil
	r.propMu.Unlock()

	if len(queue) == 0 || r.store == nil {
		return
	}

	r.peersMu.RLock()
	defer r.peersMu.RUnlock()
	for _, item := range queue {
		for key, peer := range r.peers {
			if item.fromPeer != nil && peerKey(item.fromPeer.DestinationHash) == key {
				continue
			}
			r.store.addUnhandledPeer(item.transientID, peer.DestinationHash)
			peer.queueUnhandled(item.transientID)
		}
	}
}

func deliveryDestinationHash(id *identity.Identity) ([]byte, error) {
	dest, err := destination.New(id, destination.Out, destination.Single, AppName, nil, "delivery")
	if err != nil {
		return nil, err
	}
	return dest.GetHash(), nil
}

func decodeByteList(v any) [][]byte {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([][]byte, 0, len(list))
	for _, item := range list {
		if b, ok := item.([]byte); ok {
			out = append(out, b)
		}
	}
	return out
}

func sha256Sum(data []byte) [32]byte {
	return sha256.Sum256(data)
}
