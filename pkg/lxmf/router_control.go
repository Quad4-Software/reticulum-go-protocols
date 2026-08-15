// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"quad4/reticulum-go/pkg/identity"
)

func (r *Router) statsGetRequestHandler(path string, data []byte, requestID, linkID []byte, remoteIdentity *identity.Identity, requestedAt int64) any {
	_ = path
	_ = data
	_ = requestID
	_ = linkID
	_ = requestedAt

	if remoteIdentity == nil {
		return []byte{PeerErrorNoIdentity}
	}
	if !r.controlAllowed(remoteIdentity.Hash()) {
		return []byte{PeerErrorNoAccess}
	}
	return r.compileStats()
}

func (r *Router) peerSyncRequestHandler(path string, data []byte, requestID, linkID []byte, remoteIdentity *identity.Identity, requestedAt int64) any {
	_ = path
	_ = requestID
	_ = linkID
	_ = requestedAt

	if remoteIdentity == nil {
		return []byte{PeerErrorNoIdentity}
	}
	if !r.controlAllowed(remoteIdentity.Hash()) {
		return []byte{PeerErrorNoAccess}
	}
	if len(data) != DestinationLength {
		return []byte{PeerErrorInvalidData}
	}
	r.peersMu.RLock()
	peer, ok := r.peers[peerKey(data)]
	r.peersMu.RUnlock()
	if !ok {
		return []byte{PeerErrorNotFound}
	}
	go peer.sync()
	return true
}

func (r *Router) peerUnpeerRequestHandler(path string, data []byte, requestID, linkID []byte, remoteIdentity *identity.Identity, requestedAt int64) any {
	_ = path
	_ = requestID
	_ = linkID
	_ = requestedAt

	if remoteIdentity == nil {
		return []byte{PeerErrorNoIdentity}
	}
	if !r.controlAllowed(remoteIdentity.Hash()) {
		return []byte{PeerErrorNoAccess}
	}
	if len(data) != DestinationLength {
		return []byte{PeerErrorInvalidData}
	}
	r.peersMu.RLock()
	_, ok := r.peers[peerKey(data)]
	r.peersMu.RUnlock()
	if !ok {
		return []byte{PeerErrorNotFound}
	}
	r.unpeer(data, 0)
	return true
}

func (r *Router) controlAllowed(hash []byte) bool {
	_, ok := r.controlAllow[peerKey(hash)]
	return ok
}
