// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"quad4/msgpack/v5/pkg/msgpack"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/resource"
)

const (
	peerMaxUnreachable     = 14 * 24 * 60 * 60
	peerSyncBackoffStep    = 12 * 60
	peerPathRequestGrace   = 7500 * time.Millisecond
	peerLinkAttemptTimeout = 45 * time.Second
)

// Peer is a persisted propagation peer (msgpack on disk).
type Peer struct {
	router *Router

	DestinationHash []byte
	Identity        *identity.Identity
	Destination     *destination.Destination

	Alive        bool
	LastHeard    float64
	Metadata     map[byte]any
	SyncStrategy byte

	PeeringKey   []byte
	PeeringValue int
	PeeringCost  int

	NextSyncAttempt float64
	LastSyncAttempt float64
	SyncBackoff     float64
	PeeringTimebase int64

	LinkEstablishmentRate float64
	SyncTransferRate      float64

	PropagationTransferLimit float64
	PropagationSyncLimit     float64
	PropagationStampCost     int
	PropagationStampCostFlex int

	State byte
	Link  *link.Link

	LastOffer []string

	Offered  int64
	Outgoing int64
	Incoming int64
	RXBytes  int64
	TXBytes  int64

	peeringKeyMu sync.Mutex
}

// UnhandledCount returns messages queued for this peer.
func (p *Peer) UnhandledCount(r *Router) int {
	if r == nil || r.store == nil {
		return 0
	}
	count := 0
	peerHex := peerKey(p.DestinationHash)
	for _, ent := range r.store.entriesSnapshot() {
		if slices.Contains(ent.UnhandledPeers, peerHex) {
			count++
		}
	}
	return count
}

// AcceptanceRate returns outgoing/offered ratio.
func (p *Peer) AcceptanceRate() float64 {
	if p.Offered == 0 {
		return 0
	}
	return float64(p.Outgoing) / float64(p.Offered)
}

// Name returns PN metadata name when present.
func (p *Peer) Name() string {
	if p.Metadata == nil {
		return ""
	}
	v, ok := p.Metadata[PNMetaName]
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case []byte:
		return string(x)
	case string:
		return x
	default:
		return ""
	}
}

// PeeringKeyValue returns the generated peering key score.
func (p *Peer) PeeringKeyValue() int {
	return p.PeeringValue
}

func (p *Peer) peeringKeyReady() bool {
	if p.PeeringCost <= 0 {
		return false
	}
	return p.PeeringKey != nil && p.PeeringValue >= p.PeeringCost
}

func (p *Peer) generatePeeringKey() bool {
	if p.PeeringCost <= 0 || p.router == nil || p.router.identity == nil {
		return false
	}
	p.peeringKeyMu.Lock()
	defer p.peeringKeyMu.Unlock()
	if p.PeeringKey != nil && p.PeeringValue >= p.PeeringCost {
		return true
	}
	if p.Identity == nil {
		id, err := identity.Recall(p.DestinationHash)
		if err != nil || id == nil {
			return false
		}
		p.Identity = id
	}
	material := append(append([]byte(nil), p.Identity.Hash()...), p.router.identity.Hash()...)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	stamp, value, err := GenerateStamp(ctx, material, p.PeeringCost, WorkblockExpandRoundsPeering)
	if err != nil {
		return false
	}
	p.PeeringKey = stamp
	p.PeeringValue = value
	return value >= p.PeeringCost
}

func (p *Peer) queueUnhandled(transientID []byte) {
	if p.router == nil || p.router.store == nil {
		return
	}
	p.router.store.addUnhandledPeer(transientID, p.DestinationHash)
}

func (p *Peer) queueHandled(transientID []byte) {
	if p.router == nil || p.router.store == nil {
		return
	}
	p.router.store.markHandled(transientID, p.DestinationHash)
}

func (p *Peer) sync() {
	if p.router == nil || p.State != PeerStateIdle {
		return
	}
	p.LastSyncAttempt = float64(time.Now().Unix())

	if time.Now().Unix() < int64(p.NextSyncAttempt) {
		return
	}
	if p.PropagationStampCost == 0 || p.PeeringCost == 0 {
		return
	}
	if !p.peeringKeyReady() {
		go p.generatePeeringKey()
		return
	}

	tr := p.router.transport
	if !tr.HasPath(p.DestinationHash) {
		_ = tr.RequestPath(p.DestinationHash, "", nil, true)
		time.Sleep(peerPathRequestGrace)
	}
	if !tr.HasPath(p.DestinationHash) {
		return
	}

	if p.Identity == nil {
		id, err := identity.Recall(p.DestinationHash)
		if err != nil || id == nil {
			return
		}
		p.Identity = id
	}
	if p.Destination == nil {
		dest, err := destination.FromHash(p.DestinationHash, p.Identity, destination.Single, tr)
		if err != nil {
			return
		}
		p.Destination = dest
	}

	p.SyncBackoff += peerSyncBackoffStep
	p.NextSyncAttempt = float64(time.Now().Unix()) + p.SyncBackoff

	lnk := link.NewLink(p.Destination, tr, nil, nil, func(closed *link.Link) {
		if p.Link == closed {
			p.Link = nil
			p.State = PeerStateIdle
		}
	})
	if err := lnk.Establish(); err != nil {
		return
	}
	lnk.Start()
	p.Link = lnk
	p.State = PeerStateLinkEstablishing

	deadline := time.Now().Add(peerLinkAttemptTimeout)
	for !lnk.IsActive() && time.Now().Before(deadline) {
		time.Sleep(pathPollInterval)
	}
	if !lnk.IsActive() {
		lnk.Teardown()
		p.Link = nil
		p.State = PeerStateIdle
		return
	}

	p.State = PeerStateLinkReady
	p.Alive = true
	p.LastHeard = float64(time.Now().Unix())
	p.SyncBackoff = 0
	p.sendOffer(lnk)
}

func (p *Peer) sendOffer(lnk *link.Link) {
	if p.router == nil || p.router.store == nil {
		return
	}
	minCost := max(0, p.PropagationStampCost-p.PropagationStampCostFlex)
	ids := make([][]byte, 0)
	p.LastOffer = nil

	for key, ent := range p.router.store.entriesSnapshot() {
		tid, err := hex.DecodeString(key)
		if err != nil {
			continue
		}
		if ent.StampValue < int64(minCost) {
			continue
		}
		handled := false
		peerKey := peerKey(p.DestinationHash)
		if slices.Contains(ent.HandledPeers, peerKey) {
			handled = true
		}
		if handled {
			continue
		}
		ids = append(ids, tid)
		p.LastOffer = append(p.LastOffer, key)
	}
	if len(ids) == 0 {
		lnk.Teardown()
		p.State = PeerStateIdle
		return
	}

	offer := []any{p.PeeringKey, ids}
	receipt, err := lnk.Request(PathOffer, offer, 0)
	if err != nil {
		lnk.Teardown()
		p.State = PeerStateIdle
		return
	}
	p.State = PeerStateRequestSent
	receipt.SetResponseCallback(func(r *link.RequestReceipt) {
		p.handleOfferResponse(r)
	})
	receipt.SetFailedCallback(func(r *link.RequestReceipt) {
		_ = r
		if p.Link != nil {
			p.Link.Teardown()
		}
		p.State = PeerStateIdle
	})
}

func (p *Peer) handleOfferResponse(receipt *link.RequestReceipt) {
	p.State = PeerStateResponseReceived
	resp := receipt.GetResponseValue()

	switch v := resp.(type) {
	case []byte:
		if len(v) == 1 {
			switch v[0] {
			case PeerErrorThrottled:
				p.NextSyncAttempt = float64(time.Now().Unix()) + pnStampThrottleSeconds
				return
			case PeerErrorNoAccess:
				p.router.unpeer(p.DestinationHash, 0)
				return
			}
		}
	case bool:
		if !v {
			for _, key := range p.LastOffer {
				tid, _ := hex.DecodeString(key)
				p.router.store.markHandled(tid, p.DestinationHash)
			}
			return
		}
	}

	wantedKeys := p.LastOffer
	if list, ok := resp.([]any); ok {
		wantedKeys = nil
		for _, item := range list {
			if b, ok := item.([]byte); ok {
				wantedKeys = append(wantedKeys, peerKey(b))
			}
		}
	}

	payloads := make([][]byte, 0, len(wantedKeys))
	for _, key := range wantedKeys {
		if _, err := hex.DecodeString(key); err != nil {
			continue
		}
		data, err := os.ReadFile(p.router.store.entryPath(key))
		if err != nil {
			continue
		}
		payloads = append(payloads, data)
	}
	if len(payloads) == 0 {
		if p.Link != nil {
			p.Link.Teardown()
		}
		p.State = PeerStateIdle
		return
	}

	packed, err := msgpack.Marshal([]any{float64(time.Now().Unix()), payloads})
	if err != nil || p.Link == nil {
		return
	}
	res, err := resource.New(packed, true)
	if err != nil {
		return
	}
	p.State = PeerStateResourceTransferring
	_ = p.Link.SendResource(res)
	p.Offered += int64(len(wantedKeys))
	p.Outgoing += int64(len(wantedKeys))
}

func peerFromBytes(r *Router, data []byte) (*Peer, error) {
	var dict map[string]any
	if err := msgpack.Unmarshal(data, &dict); err != nil {
		return nil, err
	}
	dest, ok := dict["destination_hash"].([]byte)
	if !ok || len(dest) != DestinationLength {
		return nil, fmt.Errorf("lxmf: invalid peer destination hash")
	}
	peer := newPeer(r, dest)
	if v, ok := dict["peering_timebase"].(int64); ok {
		peer.PeeringTimebase = v
	}
	if v, ok := dict["alive"].(bool); ok {
		peer.Alive = v
	}
	if v, ok := dict["last_heard"].(float64); ok {
		peer.LastHeard = v
	}
	if v, ok := dict["peering_key"].([]byte); ok {
		peer.PeeringKey = v
	}
	if v, ok := dict["metadata"].(map[any]any); ok {
		peer.Metadata = mapAnyToByteKey(v)
	}
	if v, ok := dict["propagation_stamp_cost"].(int64); ok {
		peer.PropagationStampCost = int(v)
	}
	if v, ok := dict["propagation_stamp_cost_flexibility"].(int64); ok {
		peer.PropagationStampCostFlex = int(v)
	}
	if v, ok := dict["peering_cost"].(int64); ok {
		peer.PeeringCost = int(v)
	}
	if v, ok := dict["propagation_transfer_limit"].(float64); ok {
		peer.PropagationTransferLimit = v
	}
	if v, ok := dict["propagation_sync_limit"].(float64); ok {
		peer.PropagationSyncLimit = v
	}
	return peer, nil
}

func (p *Peer) toBytes() ([]byte, error) {
	dict := map[string]any{
		"destination_hash":                   p.DestinationHash,
		"peering_timebase":                   p.PeeringTimebase,
		"alive":                              p.Alive,
		"last_heard":                         p.LastHeard,
		"sync_strategy":                      p.SyncStrategy,
		"peering_key":                        p.PeeringKey,
		"metadata":                           p.Metadata,
		"link_establishment_rate":            p.LinkEstablishmentRate,
		"sync_transfer_rate":                 p.SyncTransferRate,
		"propagation_transfer_limit":         p.PropagationTransferLimit,
		"propagation_sync_limit":             p.PropagationSyncLimit,
		"propagation_stamp_cost":             p.PropagationStampCost,
		"propagation_stamp_cost_flexibility": p.PropagationStampCostFlex,
		"peering_cost":                       p.PeeringCost,
		"last_sync_attempt":                  p.LastSyncAttempt,
		"offered":                            p.Offered,
		"outgoing":                           p.Outgoing,
		"incoming":                           p.Incoming,
		"rx_bytes":                           p.RXBytes,
		"tx_bytes":                           p.TXBytes,
		"handled_ids":                        []any{},
		"unhandled_ids":                      []any{},
	}
	return msgpack.Marshal(dict)
}

func newPeer(r *Router, destHash []byte) *Peer {
	return &Peer{
		router:          r,
		DestinationHash: append([]byte(nil), destHash...),
		SyncStrategy:    PeerStrategyPersistent,
	}
}

func (r *Router) peer(destHash []byte, timebase int64, transferLimit, syncLimit float64, stampCost, stampFlex, peeringCost int, metadata map[byte]any) {
	if r.isOwnPropagationHash(destHash) {
		return
	}
	if peeringCost > r.cfg.Propagation.RemotePeeringCostMax {
		if _, ok := r.peers[peerKey(destHash)]; ok {
			r.unpeer(destHash, timebase)
		}
		return
	}

	key := peerKey(destHash)
	r.peersMu.Lock()

	if peer, ok := r.peers[key]; ok {
		if timebase > peer.PeeringTimebase {
			peer.Alive = true
			peer.Metadata = metadata
			peer.SyncBackoff = 0
			peer.NextSyncAttempt = 0
			peer.PeeringTimebase = timebase
			peer.LastHeard = float64(time.Now().Unix())
			peer.PropagationStampCost = stampCost
			peer.PropagationStampCostFlex = stampFlex
			peer.PeeringCost = peeringCost
			peer.PropagationTransferLimit = transferLimit
			peer.PropagationSyncLimit = syncLimit
			r.peersMu.Unlock()
			r.seedPeerSync(peer)
			return
		}
		r.peersMu.Unlock()
		return
	}

	if len(r.peers) >= r.cfg.Propagation.MaxPeers {
		r.peersMu.Unlock()
		return
	}

	peer := newPeer(r, destHash)
	peer.Alive = true
	peer.Metadata = metadata
	peer.LastHeard = float64(time.Now().Unix())
	peer.PeeringTimebase = timebase
	peer.PropagationStampCost = stampCost
	peer.PropagationStampCostFlex = stampFlex
	peer.PeeringCost = peeringCost
	peer.PropagationTransferLimit = transferLimit
	peer.PropagationSyncLimit = syncLimit
	r.peers[key] = peer
	r.peersMu.Unlock()
	Info("peered with propagation node", "peer", key)
	r.seedPeerSync(peer)
}

func (r *Router) queuePeerStoreSync(peer *Peer) int {
	if peer == nil || r.store == nil {
		return 0
	}
	queued := 0
	pk := peerKey(peer.DestinationHash)
	for key, ent := range r.store.entriesSnapshot() {
		if slices.Contains(ent.HandledPeers, pk) {
			continue
		}
		tid, err := hex.DecodeString(key)
		if err != nil {
			continue
		}
		r.store.addUnhandledPeer(tid, peer.DestinationHash)
		queued++
	}
	return queued
}

func (r *Router) seedPeerSync(peer *Peer) {
	if peer == nil {
		return
	}
	r.queuePeerStoreSync(peer)
	go peer.sync()
}

func (p *Peer) needsSync(r *Router) bool {
	if p.UnhandledCount(r) > 0 {
		return true
	}
	if r == nil || r.store == nil {
		return p.LastSyncAttempt == 0
	}
	pk := peerKey(p.DestinationHash)
	for _, ent := range r.store.entriesSnapshot() {
		if !slices.Contains(ent.HandledPeers, pk) {
			return true
		}
	}
	return false
}

func (r *Router) unpeer(destHash []byte, timestamp int64) {
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	key := peerKey(destHash)
	r.peersMu.Lock()
	defer r.peersMu.Unlock()
	peer, ok := r.peers[key]
	if !ok {
		return
	}
	if int64(timestamp) >= peer.PeeringTimebase {
		delete(r.peers, key)
		_ = os.Remove(filepath.Join(r.peersDir(), key))
		Info("unpeered propagation node", "peer", key)
	}
}

func (r *Router) peerFromStatic(destHash []byte) {
	r.peer(destHash, time.Now().Unix(),
		r.cfg.Propagation.PropagationTransferMaxAcceptedKB,
		r.cfg.Propagation.PropagationSyncMaxAcceptedKB,
		r.cfg.Propagation.PropagationStampCostTarget,
		r.cfg.Propagation.PropagationStampCostFlexibility,
		r.cfg.Propagation.PeeringCost,
		nil,
	)
}

func (r *Router) syncPeers() {
	r.peersMu.RLock()
	peers := make([]*Peer, 0, len(r.peers))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	r.peersMu.RUnlock()

	for _, peer := range peers {
		if peer.State != PeerStateIdle {
			continue
		}
		if time.Now().Unix() > int64(peer.LastHeard)+peerMaxUnreachable && !r.isStaticPeer(peer.DestinationHash) {
			r.unpeer(peer.DestinationHash, 0)
			continue
		}
		if peer.needsSync(r) {
			go peer.sync()
		}
	}
}

func mapAnyToByteKey(m map[any]any) map[byte]any {
	out := make(map[byte]any, len(m))
	for k, v := range m {
		ki, ok := asInt64(k)
		if !ok || ki < 0 || ki > 0xff {
			continue
		}
		out[byte(ki)] = v
	}
	return out
}
