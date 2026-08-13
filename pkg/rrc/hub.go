// SPDX-License-Identifier: 0BSD
package rrc

import (
	"fmt"
	"sync"
	"time"

	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/transport"
)

// HubConfig configures hub identity metadata and limits.
type HubConfig struct {
	Name     string
	Version  string
	Limits   HubLimits
	Handlers HubHandlers
	// IncludeMemberList puts an advisory member list in JOINED/PARTED bodies.
	IncludeMemberList bool
	// EnableResourceTransfer accepts TypeResourceEnvelope metadata (rrcd extension).
	EnableResourceTransfer bool
	// MaxResourceBytes is the advertised resource payload size limit when transfer is enabled.
	MaxResourceBytes uint64
	// Capabilities overrides WELCOME capability flags. Nil uses DefaultHubCapabilities.
	Capabilities map[uint64]any
}

func (c *HubConfig) applyDefaults() {
	if c.Name == "" {
		c.Name = "reticulum-go-protocols"
	}
	if c.Version == "" {
		c.Version = "0.1.0"
	}
	if c.Limits.MaxNickBytes == 0 {
		c.Limits.MaxNickBytes = DefaultMaxNickBytes
	}
	if c.Limits.MaxRoomsPerSession == 0 {
		c.Limits.MaxRoomsPerSession = DefaultMaxRoomsPerSession
	}
	if c.Limits.MaxRoomNameBytes == 0 {
		c.Limits.MaxRoomNameBytes = DefaultMaxRoomNameBytes
	}
	if c.Limits.MaxMsgBodyBytes == 0 {
		c.Limits.MaxMsgBodyBytes = DefaultMaxMsgBodyBytes
	}
	if c.Limits.RateLimitMsgsPerMinute == 0 {
		c.Limits.RateLimitMsgsPerMinute = DefaultRateLimitMsgsPerMinute
	}
	if c.MaxResourceBytes == 0 {
		c.MaxResourceBytes = DefaultMaxResourceBytes
	}
}

// NewHubDestination creates an inbound rrc.hub destination.
func NewHubDestination(id *identity.Identity, tr *transport.Transport) (*destination.Destination, error) {
	if id == nil || tr == nil {
		return nil, ErrNilArgument
	}
	dest, err := destination.New(id, destination.In, destination.Single, AppName, tr, HubAspect)
	if err != nil {
		return nil, err
	}
	dest.AcceptsLinks(true)
	return dest, nil
}

type hubPeer struct {
	sess         *session
	peerHash     []byte
	active       bool
	rooms        map[string]struct{}
	msgTimes     []time.Time
	pendingHello *Envelope
}

// Hub is an RRC hub that accepts Links and relays room traffic.
type Hub struct {
	mu       sync.Mutex
	tr       *transport.Transport
	dest     *destination.Destination
	id       *identity.Identity
	sender   []byte
	cfg      HubConfig
	peers    map[peerID]*hubPeer
	rooms    map[string]map[peerID]struct{}
	handlers HubHandlers
	started  bool
}

// NewHub builds a hub bound to dest. Call Start to accept sessions.
func NewHub(tr *transport.Transport, dest *destination.Destination, cfg HubConfig) (*Hub, error) {
	if tr == nil || dest == nil {
		return nil, ErrNilArgument
	}
	cfg.applyDefaults()
	id := dest.GetIdentity()
	if id == nil {
		return nil, ErrNilArgument
	}
	h := &Hub{
		tr:       tr,
		dest:     dest,
		id:       id,
		sender:   append([]byte(nil), id.Hash()...),
		cfg:      cfg,
		peers:    make(map[peerID]*hubPeer),
		rooms:    make(map[string]map[peerID]struct{}),
		handlers: cfg.Handlers,
	}
	return h, nil
}

// Start registers the link-established callback and begins accepting sessions.
func (h *Hub) Start() {
	h.mu.Lock()
	if h.started {
		h.mu.Unlock()
		return
	}
	h.started = true
	h.mu.Unlock()

	h.dest.SetLinkEstablishedCallback(func(v any) {
		lnk, ok := v.(*link.Link)
		if !ok || lnk == nil {
			return
		}
		lnk.Start()
		h.acceptLink(lnk)
	})
}

// Destination returns the hub destination.
func (h *Hub) Destination() *destination.Destination {
	return h.dest
}

// DestinationHash returns the hub destination hash.
func (h *Hub) DestinationHash() []byte {
	return h.dest.GetHash()
}

type peerID [IdentityLength]byte

func peerKey(hash []byte) peerID {
	var k peerID
	if len(hash) == IdentityLength {
		copy(k[:], hash)
	}
	return k
}

func (h *Hub) acceptLink(lnk *link.Link) {
	rateCap := int(h.cfg.Limits.RateLimitMsgsPerMinute)
	if rateCap < 8 {
		rateCap = 8
	}
	if rateCap > 256 {
		rateCap = 256
	}
	p := &hubPeer{
		rooms:    make(map[string]struct{}),
		msgTimes: make([]time.Time, 0, rateCap),
	}

	var registerOnce sync.Once
	register := func(hash []byte) {
		if len(hash) != IdentityLength {
			return
		}
		var old *hubPeer
		registerOnce.Do(func() {
			h.mu.Lock()
			p.peerHash = append([]byte(nil), hash...)
			key := peerKey(p.peerHash)
			if prev, ok := h.peers[key]; ok && prev != p {
				old = prev
				for room := range old.rooms {
					if members, ok := h.rooms[room]; ok {
						delete(members, key)
						if len(members) == 0 {
							delete(h.rooms, room)
						}
					}
				}
			}
			h.peers[key] = p
			h.mu.Unlock()
		})
		if old != nil && old.sess != nil {
			old.sess.close()
		}
		h.mu.Lock()
		pending := p.pendingHello
		p.pendingHello = nil
		ready := len(p.peerHash) == IdentityLength
		h.mu.Unlock()
		if ready && pending != nil {
			h.handlePeer(p, pending)
		}
	}

	p.sess = newSession(lnk, h.sender, false, func(env *Envelope) {
		if remote := lnk.GetRemoteIdentity(); remote != nil {
			register(remote.Hash())
		}
		h.mu.Lock()
		ready := len(p.peerHash) == IdentityLength
		if !ready {
			if env.Type == TypeHello {
				p.pendingHello = env
			}
			h.mu.Unlock()
			if remote := lnk.GetRemoteIdentity(); remote != nil {
				register(remote.Hash())
			}
			return
		}
		h.mu.Unlock()
		h.handlePeer(p, env)
	}, func() {
		h.dropPeerIf(p)
	})

	lnk.SetRemoteIdentifiedCallback(func(_ *link.Link, id *identity.Identity) {
		if id != nil {
			register(id.Hash())
		}
	})

	if remote := lnk.GetRemoteIdentity(); remote != nil {
		register(remote.Hash())
	}
}

func (h *Hub) dropPeerIf(p *hubPeer) {
	if p == nil || len(p.peerHash) == 0 {
		return
	}
	key := peerKey(p.peerHash)
	h.mu.Lock()
	cur, ok := h.peers[key]
	if !ok || cur != p {
		h.mu.Unlock()
		return
	}
	type roomNotify struct {
		room  string
		peers []*hubPeer
	}
	notes := make([]roomNotify, 0, len(p.rooms))
	nick := ""
	if p.sess != nil {
		nick = p.sess.getNick()
	}
	delete(h.peers, key)
	for room := range p.rooms {
		others := h.roomPeersLocked(room, key)
		if members, ok := h.rooms[room]; ok {
			delete(members, key)
			if len(members) == 0 {
				delete(h.rooms, room)
			}
		}
		notes = append(notes, roomNotify{room: room, peers: others})
	}
	p.rooms = map[string]struct{}{}
	cb := h.handlers.OnClose
	peer := append([]byte(nil), p.peerHash...)
	includeList := h.cfg.IncludeMemberList
	h.mu.Unlock()
	var presenceBody any
	if includeList {
		presenceBody = []any{append([]byte(nil), peer...)}
	}
	for _, n := range notes {
		for _, other := range n.peers {
			_ = other.sess.sendType(TypeParted, n.room, presenceBody, nick)
		}
	}
	if cb != nil {
		cb(peer)
	}
}

func (h *Hub) applyInboundNick(p *hubPeer, nick string) error {
	nick = SanitizeNick(nick)
	if nick == "" {
		return nil
	}
	if uint64(len(nick)) > h.cfg.Limits.MaxNickBytes {
		return ErrNickTooLong
	}
	p.sess.setNick(nick)
	return nil
}

func (h *Hub) handlePeer(p *hubPeer, env *Envelope) {
	h.mu.Lock()
	active := p.active
	h.mu.Unlock()

	if env.HasNick {
		if err := h.applyInboundNick(p, env.Nick); err != nil {
			_ = h.sendError(p, "nickname too long")
			if !active {
				p.sess.close()
			}
			return
		}
	} else if env.Type == TypeHello {
		if n := HelloLegacyNick(env.Body); n != "" {
			if err := h.applyInboundNick(p, n); err != nil {
				_ = h.sendError(p, "nickname too long")
				if !active {
					p.sess.close()
					return
				}
			}
		}
	}

	if !active {
		if env.Type != TypeHello {
			return
		}
		h.onHello(p, env)
		return
	}

	switch env.Type {
	case TypeHello:
		return
	case TypeJoin:
		h.onJoin(p, env)
	case TypePart:
		h.onPart(p, env)
	case TypeNotice:
		if env.HasDestination {
			h.onDirectNotice(p, env)
			return
		}
		h.onRoomContent(p, env)
	case TypeMsg, TypeAction:
		h.onRoomContent(p, env)
	case TypeResourceEnvelope:
		h.onResourceEnvelope(p, env)
	case TypePing:
		h.onPing(p, env)
	case TypeError:
	default:
	}
}

func (h *Hub) onHello(p *hubPeer, env *Envelope) {
	body, _ := ParseHelloBody(env.Body)
	// Nick already applied in handlePeer with length enforcement.

	h.mu.Lock()
	p.active = true
	cb := h.handlers.OnHello
	h.mu.Unlock()

	caps := h.cfg.Capabilities
	if caps == nil {
		caps = DefaultHubCapabilities(h.cfg.EnableResourceTransfer)
	}
	wb := &WelcomeBody{
		HubName:      h.cfg.Name,
		HasName:      true,
		HubVersion:   h.cfg.Version,
		HasVersion:   true,
		Capabilities: caps,
		HasCaps:      caps != nil,
		Limits:       h.cfg.Limits,
		HasLimits:    true,
	}
	if err := p.sess.sendType(TypeWelcome, "", wb.ToMap(), ""); err != nil {
		p.sess.close()
		return
	}
	if cb != nil {
		cb(p.peerHash, body, env)
	}
}

func (h *Hub) onJoin(p *hubPeer, env *Envelope) {
	room := NormalizeRoom(env.Room)
	if room == "" {
		_ = h.sendError(p, "missing room")
		return
	}
	if uint64(len(room)) > h.cfg.Limits.MaxRoomNameBytes {
		_ = h.sendError(p, "room name too long")
		return
	}

	key := peerKey(p.peerHash)
	h.mu.Lock()
	if _, ok := p.rooms[room]; !ok {
		if uint64(len(p.rooms)) >= h.cfg.Limits.MaxRoomsPerSession {
			h.mu.Unlock()
			_ = h.sendError(p, "room limit exceeded")
			return
		}
		p.rooms[room] = struct{}{}
		if h.rooms[room] == nil {
			h.rooms[room] = make(map[peerID]struct{})
		}
		h.rooms[room][key] = struct{}{}
	}
	var memberList []any
	if h.cfg.IncludeMemberList {
		for pk := range h.rooms[room] {
			if peer, ok := h.peers[pk]; ok {
				memberList = append(memberList, append([]byte(nil), peer.peerHash...))
			}
		}
	}
	cb := h.handlers.OnJoin
	others := h.roomPeersLocked(room, key)
	nick := p.sess.getNick()
	h.mu.Unlock()

	var body any
	if len(memberList) > 0 {
		body = memberList
	}
	var presenceBody any
	if h.cfg.IncludeMemberList {
		presenceBody = []any{append([]byte(nil), p.peerHash...)}
	}
	for _, peer := range others {
		_ = peer.sess.sendType(TypeJoined, room, presenceBody, nick)
	}
	_ = p.sess.sendType(TypeJoined, room, body, "")
	if cb != nil {
		cb(p.peerHash, room, env)
	}
}

func (h *Hub) onPart(p *hubPeer, env *Envelope) {
	room := NormalizeRoom(env.Room)
	if room == "" {
		return
	}
	key := peerKey(p.peerHash)
	h.mu.Lock()
	if _, ok := p.rooms[room]; !ok {
		h.mu.Unlock()
		return
	}
	delete(p.rooms, room)
	if members, ok := h.rooms[room]; ok {
		delete(members, key)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}
	cb := h.handlers.OnPart
	others := h.roomPeersLocked(room, key)
	nick := p.sess.getNick()
	h.mu.Unlock()

	var presenceBody any
	if h.cfg.IncludeMemberList {
		presenceBody = []any{append([]byte(nil), p.peerHash...)}
	}
	for _, peer := range others {
		_ = peer.sess.sendType(TypeParted, room, presenceBody, nick)
	}
	_ = p.sess.sendType(TypeParted, room, presenceBody, "")
	if cb != nil {
		cb(p.peerHash, room, env)
	}
}

func (h *Hub) onRoomContent(p *hubPeer, env *Envelope) {
	room := NormalizeRoom(env.Room)
	if room == "" {
		_ = h.sendError(p, "missing room")
		return
	}

	if !h.allowRate(p) {
		_ = h.sendError(p, "Rate limit exceeded. Try again later.")
		return
	}

	if env.HasBody && BodySizeBytes(env.Body) > h.cfg.Limits.MaxMsgBodyBytes {
		_ = h.sendError(p, "message body too large")
		return
	}

	h.mu.Lock()
	if _, ok := p.rooms[room]; !ok {
		h.mu.Unlock()
		_ = h.sendError(p, "not a member of room")
		return
	}
	roomMembers := h.rooms[room]
	members := make([]*hubPeer, 0, len(roomMembers))
	for pk := range roomMembers {
		if peer, ok := h.peers[pk]; ok && peer.active {
			members = append(members, peer)
		}
	}
	cb := h.handlers.OnMsg
	nick := p.sess.getNick()
	h.mu.Unlock()

	// Always stamp authenticated peer identity. Never trust wire Sender.
	fwd, err := envelopeFrom(env.Type, p.peerHash, env.MsgID, env.Timestamp)
	if err != nil {
		return
	}
	fwd.Room = room
	fwd.HasRoom = true
	if env.HasBody {
		fwd.Body = env.Body
		fwd.HasBody = true
	}
	if nick != "" {
		fwd.Nick = nick
		fwd.HasNick = true
	}

	for _, peer := range members {
		_ = peer.sess.sendEnvelope(fwd)
	}
	if cb != nil {
		cb(p.peerHash, env)
	}
}

func (h *Hub) allowRate(p *hubPeer) bool {
	limit := h.cfg.Limits.RateLimitMsgsPerMinute
	if limit == 0 {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := p.msgTimes[:0]
	for _, t := range p.msgTimes {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	p.msgTimes = kept
	if uint64(len(p.msgTimes)) >= limit {
		return false
	}
	p.msgTimes = append(p.msgTimes, now)
	return true
}

func (h *Hub) sendError(p *hubPeer, msg string) error {
	return p.sess.sendType(TypeError, "", msg, "")
}

func (h *Hub) roomPeersLocked(room string, exceptKey peerID) []*hubPeer {
	members := h.rooms[room]
	out := make([]*hubPeer, 0, len(members))
	for pk := range members {
		if pk == exceptKey {
			continue
		}
		if peer, ok := h.peers[pk]; ok && peer.active {
			out = append(out, peer)
		}
	}
	return out
}

func (h *Hub) onDirectNotice(p *hubPeer, env *Envelope) {
	if env.HasRoom {
		_ = h.sendError(p, "direct notice must not include room")
		return
	}
	if !env.HasDestination || len(env.Destination) != IdentityLength {
		_ = h.sendError(p, "direct notice requires destination identity")
		return
	}
	if !h.allowRate(p) {
		_ = h.sendError(p, "Rate limit exceeded. Try again later.")
		return
	}
	if env.HasBody && BodySizeBytes(env.Body) > h.cfg.Limits.MaxMsgBodyBytes {
		_ = h.sendError(p, "message body too large")
		return
	}

	h.mu.Lock()
	target, ok := h.peers[peerKey(env.Destination)]
	active := ok && target.active
	nick := p.sess.getNick()
	cb := h.handlers.OnMsg
	h.mu.Unlock()
	if !active {
		_ = h.sendError(p, "destination not connected")
		return
	}

	fwd, err := envelopeFrom(TypeNotice, p.peerHash, env.MsgID, env.Timestamp)
	if err != nil {
		return
	}
	fwd.Destination = cloneBytes(env.Destination)
	fwd.HasDestination = true
	if env.HasBody {
		fwd.Body = env.Body
		fwd.HasBody = true
	}
	if nick != "" {
		fwd.Nick = nick
		fwd.HasNick = true
	}
	_ = target.sess.sendEnvelope(fwd)
	if cb != nil {
		cb(p.peerHash, env)
	}
}

func (h *Hub) onPing(p *hubPeer, env *Envelope) {
	var body any
	if env != nil && env.HasBody {
		body = env.Body
	}
	_ = p.sess.sendType(TypePong, "", body, "")
}

func (h *Hub) onResourceEnvelope(p *hubPeer, env *Envelope) {
	if !h.cfg.EnableResourceTransfer {
		_ = h.sendError(p, "resource transfer disabled")
		return
	}
	body, reason := ValidateResourceEnvelopeBody(env.Body)
	if reason != "" {
		_ = h.sendError(p, reason)
		return
	}
	if body.Size > h.cfg.MaxResourceBytes {
		_ = h.sendError(p, fmt.Sprintf("resource too large: %d > %d", body.Size, h.cfg.MaxResourceBytes))
		return
	}
	if cb := h.handlers.OnResource; cb != nil {
		cb(p.peerHash, env)
	}
}

// Close tears down all peer sessions.
func (h *Hub) Close() {
	h.mu.Lock()
	peers := make([]*hubPeer, 0, len(h.peers))
	for _, p := range h.peers {
		peers = append(peers, p)
	}
	h.peers = make(map[peerID]*hubPeer)
	h.rooms = make(map[string]map[peerID]struct{})
	h.mu.Unlock()
	for _, p := range peers {
		p.sess.close()
	}
}

// PeerCount returns the number of connected peers.
func (h *Hub) PeerCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.peers)
}

// RoomMembers returns identity hashes currently in room.
func (h *Hub) RoomMembers(room string) [][]byte {
	room = NormalizeRoom(room)
	h.mu.Lock()
	defer h.mu.Unlock()
	members := h.rooms[room]
	out := make([][]byte, 0, len(members))
	for pk := range members {
		if p, ok := h.peers[pk]; ok {
			out = append(out, append([]byte(nil), p.peerHash...))
		}
	}
	return out
}

// HasPeer reports whether peerHash is currently connected.
func (h *Hub) HasPeer(peerHash []byte) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.peers[peerKey(peerHash)]
	return ok
}

// FormatError returns a short error description from an ERROR envelope body.
func FormatError(env *Envelope) string {
	if env == nil {
		return ""
	}
	if s, ok := BodyAsString(env.Body); ok {
		return s
	}
	return fmt.Sprint(env.Body)
}
