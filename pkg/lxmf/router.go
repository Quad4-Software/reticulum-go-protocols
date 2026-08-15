// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

// RouterOptions configures an lxmd-style LXMF router instance.
type RouterOptions struct {
	Config      Config
	StoragePath string
	MessagesDir string
	OnInbound   string
}

// Router is the lxmd server core for delivery and optional propagation.
type Router struct {
	mu sync.RWMutex

	identity  *identity.Identity
	transport *transport.Transport
	cfg       Config

	storagePath string
	messagesDir string
	onInbound   string

	deliveryDest    *destination.Destination
	propagationDest *destination.Destination
	controlDest     *destination.Destination

	propagationEnabled bool
	propagationStart   time.Time

	store *MessageStore

	peersMu      sync.RWMutex
	peers        map[string]*Peer
	staticPeers  map[string]struct{}
	controlAllow map[string]struct{}

	inboundStampCost *int
	enforceStamps    bool

	ignoredList map[string]struct{}
	allowedList map[string]struct{}

	deliveredMu      sync.Mutex
	locallyDelivered map[string]float64
	processedMu      sync.Mutex
	locallyProcessed map[string]float64

	clientPropagationReceived   int64
	clientPropagationServed     int64
	unpeeredPropagationIncoming int64
	unpeeredPropagationRxBytes  int64

	propMu               sync.Mutex
	validatedPeerLinks   map[string]bool
	acceptedOfferLinks   map[string]byte
	validatingFrom       map[string]float64
	throttledPeers       map[string]float64
	peerDistributionQ    []peerDistEntry
	propagationResources int

	deliveryHandler *deliveryAnnounceHandler
	propHandler     *propagationAnnounceHandler

	stop chan struct{}
	wg   sync.WaitGroup
}

type peerDistEntry struct {
	transientID []byte
	fromPeer    *Peer
}

// NewRouter creates a router bound to identity and transport. Call RegisterDelivery and optionally EnablePropagation before Start.
func NewRouter(id *identity.Identity, tr *transport.Transport, opts RouterOptions) (*Router, error) {
	if id == nil {
		return nil, errors.New("lxmf: nil identity")
	}
	if tr == nil {
		return nil, errors.New("lxmf: nil transport")
	}
	if opts.StoragePath == "" {
		return nil, errors.New("lxmf: storage path required")
	}
	cfg := opts.Config
	if cfg.LXMF.DisplayName == "" && cfg.Propagation.NodeName == "" {
		cfg = DefaultConfig()
		if opts.Config.LXMF.DisplayName != "" {
			cfg.LXMF.DisplayName = opts.Config.LXMF.DisplayName
		}
		if opts.Config.Propagation.NodeName != "" {
			cfg.Propagation.NodeName = opts.Config.Propagation.NodeName
		}
	}

	storagePath := filepath.Clean(opts.StoragePath)
	if err := os.MkdirAll(storagePath, 0o700); err != nil {
		return nil, fmt.Errorf("lxmf: storage path: %w", err)
	}

	onInbound := opts.OnInbound
	if onInbound == "" {
		onInbound = cfg.LXMF.OnInbound
	}

	r := &Router{
		identity:           id,
		transport:          tr,
		cfg:                cfg,
		storagePath:        storagePath,
		messagesDir:        opts.MessagesDir,
		onInbound:          onInbound,
		peers:              make(map[string]*Peer),
		staticPeers:        make(map[string]struct{}),
		controlAllow:       make(map[string]struct{}),
		ignoredList:        make(map[string]struct{}),
		allowedList:        make(map[string]struct{}),
		locallyDelivered:   make(map[string]float64),
		locallyProcessed:   make(map[string]float64),
		validatedPeerLinks: make(map[string]bool),
		acceptedOfferLinks: make(map[string]byte),
		validatingFrom:     make(map[string]float64),
		throttledPeers:     make(map[string]float64),
		stop:               make(chan struct{}),
	}

	for _, h := range cfg.Propagation.ControlAllowed {
		decoded, err := decodeHashHex(h)
		if err != nil {
			return nil, err
		}
		r.controlAllow[peerKey(decoded)] = struct{}{}
	}
	r.controlAllow[peerKey(id.Hash())] = struct{}{}

	staticHashes, err := cfg.Propagation.StaticPeerHashes()
	if err != nil {
		return nil, err
	}
	for _, h := range staticHashes {
		r.staticPeers[peerKey(h)] = struct{}{}
	}

	if cfg.Propagation.AuthRequired {
		for _, h := range cfg.Propagation.ControlAllowed {
			decoded, err := decodeHashHex(h)
			if err == nil {
				r.allowedList[peerKey(decoded)] = struct{}{}
			}
		}
	}

	r.deliveryHandler = &deliveryAnnounceHandler{router: r}
	r.propHandler = &propagationAnnounceHandler{router: r}
	tr.RegisterAnnounceHandler(r.deliveryHandler)
	tr.RegisterAnnounceHandler(r.propHandler)

	r.loadPersistedState()
	return r, nil
}

// RegisterDelivery registers the inbound lxmf.delivery destination and enables ratchets.
func (r *Router) RegisterDelivery(displayName string, stampCost *int) (*destination.Destination, error) {
	dest, err := NewDeliveryDestination(r.identity, r.transport)
	if err != nil {
		return nil, err
	}

	ratchetDir := filepath.Join(r.storagePath, "ratchets")
	if err := os.MkdirAll(ratchetDir, 0o700); err != nil {
		return nil, err
	}
	ratchetPath := filepath.Join(ratchetDir, peerKey(dest.GetHash())+".ratchets")
	dest.EnableRatchets(ratchetPath)

	dest.SetLinkEstablishedCallback(func(v any) {
		r.deliveryLinkEstablished(v)
	})

	if displayName == "" {
		displayName = r.cfg.LXMF.DisplayName
	}
	appData, err := EncodeAnnounceAppDataV5(displayName, -1)
	if err == nil {
		dest.SetDefaultAppData(appData)
	}

	r.mu.Lock()
	r.deliveryDest = dest
	r.inboundStampCost = stampCost
	r.mu.Unlock()

	r.transport.RegisterDestination(dest.GetHash(), r)
	return dest, nil
}

// EnablePropagation activates the propagation node, message store, and peer sync handlers.
func (r *Router) EnablePropagation() error {
	propDest, err := destination.New(r.identity, destination.In, destination.Single, AppName, r.transport, "propagation")
	if err != nil {
		return err
	}

	prioritised, err := r.cfg.Propagation.PrioritisedHashes()
	if err != nil {
		return err
	}
	storeDir := filepath.Join(r.storagePath, "messagestore")
	store, err := NewMessageStore(storeDir, r.cfg.Propagation.MessageStorageLimitMB, prioritised)
	if err != nil {
		return err
	}

	appData, err := r.propagationAnnounceAppData()
	if err != nil {
		return err
	}
	propDest.SetDefaultAppData(appData)
	propDest.SetLinkEstablishedCallback(func(v any) {
		r.propagationLinkEstablished(v)
	})

	if err := propDest.RegisterRequestHandlerAny(PathOffer, r.offerRequestHandler, destination.AllowAll, nil); err != nil {
		return err
	}
	if err := propDest.RegisterRequestHandlerAny(PathMessageGet, r.messageGetRequestHandler, destination.AllowAll, nil); err != nil {
		return err
	}

	controlDest, err := destination.New(r.identity, destination.In, destination.Single, AppName, r.transport, "propagation", "control")
	if err != nil {
		return err
	}
	allowed := make([][]byte, 0, len(r.controlAllow))
	for k := range r.controlAllow {
		h, err := hexDecodeKey(k)
		if err == nil {
			allowed = append(allowed, h)
		}
	}
	if err := controlDest.RegisterRequestHandlerAny(PathStatsGet, r.statsGetRequestHandler, destination.AllowList, allowed); err != nil {
		return err
	}
	if err := controlDest.RegisterRequestHandlerAny(PathSyncRequest, r.peerSyncRequestHandler, destination.AllowList, allowed); err != nil {
		return err
	}
	if err := controlDest.RegisterRequestHandlerAny(PathUnpeerRequest, r.peerUnpeerRequestHandler, destination.AllowList, allowed); err != nil {
		return err
	}

	r.mu.Lock()
	r.propagationDest = propDest
	r.controlDest = controlDest
	r.store = store
	r.propagationEnabled = true
	r.propagationStart = time.Now()
	r.cfg.Propagation.EnableNode = true
	r.mu.Unlock()

	r.transport.RegisterDestination(controlDest.GetHash(), controlDest)

	for key := range r.staticPeers {
		h, err := hexDecodeKey(key)
		if err != nil {
			continue
		}
		r.peerFromStatic(h)
	}

	if r.cfg.Propagation.AnnounceAtStart {
		go r.announcePropagationNode()
	}
	return nil
}

// Start launches background maintenance jobs.
func (r *Router) Start() {
	r.wg.Add(1)
	go r.jobLoop()
}

// Stop shuts down background jobs and persists state.
func (r *Router) Stop() {
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
	r.wg.Wait()
	r.saveLocallyDelivered()
	r.saveLocallyProcessed()
	r.saveNodeStats()
	r.savePeers()
}

// DeliveryDestination returns the registered delivery destination.
func (r *Router) DeliveryDestination() *destination.Destination {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.deliveryDest
}

// PropagationDestination returns the propagation destination when enabled.
func (r *Router) PropagationDestination() *destination.Destination {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.propagationDest
}

func (r *Router) isOwnPropagationHash(h []byte) bool {
	if len(h) != DestinationLength {
		return false
	}
	pd := r.PropagationDestination()
	if pd == nil {
		return false
	}
	return bytes.Equal(h, pd.GetHash())
}

// IgnoreDestination adds a source hash to the inbound ignore list.
func (r *Router) IgnoreDestination(hash []byte) {
	r.mu.Lock()
	r.ignoredList[peerKey(hash)] = struct{}{}
	r.mu.Unlock()
}

// AllowDestination adds an identity hash allowed when auth_required is set.
func (r *Router) AllowDestination(hash []byte) {
	r.mu.Lock()
	r.allowedList[peerKey(hash)] = struct{}{}
	r.mu.Unlock()
}

// SetInboundStampCost sets the required inbound delivery stamp cost.
func (r *Router) SetInboundStampCost(cost *int) {
	r.mu.Lock()
	r.inboundStampCost = cost
	r.mu.Unlock()
}

// EnforceStamps enables inbound stamp enforcement.
func (r *Router) EnforceStamps(enabled bool) {
	r.mu.Lock()
	r.enforceStamps = enabled
	r.mu.Unlock()
}

func (r *Router) propagationAnnounceAppData() ([]byte, error) {
	nodeName := r.cfg.Propagation.NodeName
	if nodeName == "" {
		nodeName = "Anonymous Propagation Node"
	}
	transfer := int(r.cfg.Propagation.PropagationTransferMaxAcceptedKB)
	syncLimit := int(r.cfg.Propagation.PropagationSyncMaxAcceptedKB)
	isPN := !r.cfg.Propagation.FromStaticOnly
	if !isPN {
		transfer = 0
	}
	return EncodePNAnnounceAppData(
		time.Now().Unix(),
		transfer,
		syncLimit,
		r.cfg.Propagation.PropagationStampCostTarget,
		r.cfg.Propagation.PropagationStampCostFlexibility,
		r.cfg.Propagation.PeeringCost,
		nodeName,
	)
}

func (r *Router) isStaticPeer(hash []byte) bool {
	_, ok := r.staticPeers[peerKey(hash)]
	return ok
}

func (r *Router) hasMessage(hash []byte) bool {
	key := peerKey(hash)
	r.deliveredMu.Lock()
	_, ok := r.locallyDelivered[key]
	r.deliveredMu.Unlock()
	return ok
}

func hexDecodeKey(key string) ([]byte, error) {
	return decodeHashHex(key)
}
