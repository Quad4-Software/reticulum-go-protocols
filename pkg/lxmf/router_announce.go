// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"encoding/hex"
	"time"

	"quad4/reticulum-go/pkg/identity"
)

type deliveryAnnounceHandler struct {
	router *Router
}

func (h *deliveryAnnounceHandler) AspectFilter() []string { return []string{AppName + ".delivery"} }

func (h *deliveryAnnounceHandler) ReceivePathResponses() bool { return true }

func (h *deliveryAnnounceHandler) ReceivedAnnounce(destHash []byte, identAny any, appData []byte, hops uint8) error {
	_ = hops
	if h.router == nil || h.router.deliveryDest == nil {
		return nil
	}
	if !bytes.Equal(destHash, h.router.deliveryDest.GetHash()) {
		return nil
	}
	cost, ok, err := StampCostFromAppData(appData)
	if err != nil {
		return err
	}
	if ok {
		c := int(cost)
		h.router.SetInboundStampCost(&c)
	}
	return nil
}

type propagationAnnounceHandler struct {
	router *Router
}

func (h *propagationAnnounceHandler) AspectFilter() []string {
	return []string{AppName + ".propagation"}
}

func (h *propagationAnnounceHandler) ReceivePathResponses() bool { return true }

func (h *propagationAnnounceHandler) ReceivedAnnounce(destHash []byte, identAny any, appData []byte, hops uint8) error {
	if h.router == nil || !PNAnnounceDataIsValid(appData) {
		return nil
	}
	id, err := identityFromAnnounce(identAny, destHash)
	if err != nil || id == nil {
		return nil
	}
	expected, err := PropagationDestinationHash(id)
	if err != nil || !bytes.Equal(expected, destHash) {
		return nil
	}
	if h.router.isOwnPropagationHash(destHash) {
		return nil
	}

	arr, err := decodePNAnnounceArray(appData)
	if err != nil || len(arr) < 7 {
		return nil
	}
	timebase, _ := asInt64(arr[1])
	enabled, _ := arr[2].(bool)
	transferLimit, _ := asInt64(arr[3])
	syncLimit, _ := asInt64(arr[4])
	costs, _ := arr[5].([]any)
	if len(costs) < 3 {
		return nil
	}
	stampCost, _ := asInt64(costs[0])
	stampFlex, _ := asInt64(costs[1])
	peeringCost, _ := asInt64(costs[2])
	metadata := map[byte]any{}
	if md, ok := arr[6].(map[any]any); ok {
		metadata = mapAnyToByteKey(md)
	} else if md, ok := arr[6].(map[string]any); ok {
		for k, v := range md {
			if len(k) == 1 {
				metadata[k[0]] = v
			}
		}
	}

	cfg := h.router.cfg.Propagation
	if h.router.isStaticPeer(destHash) {
		h.router.peer(destHash, timebase, float64(transferLimit), float64(syncLimit), int(stampCost), int(stampFlex), int(peeringCost), metadata)
		return nil
	}

	if !h.router.propagationEnabled {
		return nil
	}

	tr := h.router.transport
	if cfg.Autopeer && enabled && tr != nil && int(tr.HopsTo(destHash)) <= cfg.AutopeerMaxDepth {
		h.router.peer(destHash, timebase, float64(transferLimit), float64(syncLimit), int(stampCost), int(stampFlex), int(peeringCost), metadata)
		return nil
	}

	if !enabled {
		h.router.unpeer(destHash, timebase)
	}
	return nil
}

func (r *Router) announceDelivery() {
	dest := r.DeliveryDestination()
	if dest == nil {
		return
	}
	name := r.cfg.LXMF.DisplayName
	appData, err := EncodeAnnounceAppDataV5(name, -1)
	if err != nil {
		return
	}
	_ = dest.Announce(true, appData, nil)
}

func (r *Router) announcePropagationNode() {
	dest := r.PropagationDestination()
	if dest == nil {
		return
	}
	time.Sleep(nodeAnnounceDelay)
	appData, err := r.propagationAnnounceAppData()
	if err != nil {
		return
	}
	dest.SetDefaultAppData(appData)
	_ = dest.Announce(true, appData, nil)
	if r.controlDest != nil && len(r.controlAllow) > 1 {
		_ = r.controlDest.Announce(true, nil, nil)
	}
}

const nodeAnnounceDelay = 20 * time.Second

func (r *Router) updateStampCost(destHash []byte, stampCost int64) {
	if r.deliveryDest == nil || !bytes.Equal(destHash, r.deliveryDest.GetHash()) {
		return
	}
	if stampCost < 0 {
		r.SetInboundStampCost(nil)
		return
	}
	c := int(stampCost)
	r.SetInboundStampCost(&c)
}

// decodePNAnnouncePayload unpacks propagation announce fields for peering.
func decodePNAnnouncePayload(appData []byte) (timebase int64, enabled bool, transfer, sync, stampCost, stampFlex, peeringCost int64, metadata map[byte]any, ok bool) {
	arr, err := decodePNAnnounceArray(appData)
	if err != nil || len(arr) < 7 {
		return
	}
	timebase, _ = asInt64(arr[1])
	enabled, _ = arr[2].(bool)
	transfer, _ = asInt64(arr[3])
	sync, _ = asInt64(arr[4])
	costs, _ := arr[5].([]any)
	if len(costs) < 3 {
		return
	}
	stampCost, _ = asInt64(costs[0])
	stampFlex, _ = asInt64(costs[1])
	peeringCost, _ = asInt64(costs[2])
	if md, mOk := arr[6].(map[any]any); mOk {
		metadata = mapAnyToByteKey(md)
	}
	ok = true
	return
}

func recallAppData(destHash []byte) []byte {
	data, ok := identity.GetKnownDestination(hex.EncodeToString(destHash))
	if !ok || len(data) < 4 {
		return nil
	}
	appData, _ := data[3].([]byte)
	return appData
}

func (r *Router) tryAutopeerFromIncomingSync(remotePropHash []byte) {
	if remotePropHash == nil || r.isOwnPropagationHash(remotePropHash) {
		return
	}
	r.peersMu.RLock()
	_, known := r.peers[peerKey(remotePropHash)]
	r.peersMu.RUnlock()
	if known {
		return
	}
	appData := recallAppData(remotePropHash)
	if !PNAnnounceDataIsValid(appData) {
		return
	}
	timebase, enabled, transfer, sync, stampCost, stampFlex, peeringCost, metadata, ok := decodePNAnnouncePayload(appData)
	if !ok {
		return
	}
	cfg := r.cfg.Propagation
	if !cfg.Autopeer || !enabled || r.transport == nil {
		return
	}
	if int(r.transport.HopsTo(remotePropHash)) > cfg.AutopeerMaxDepth {
		return
	}
	Verbose("autopeer from incoming sync", "peer", hex.EncodeToString(remotePropHash))
	r.peer(remotePropHash, timebase, float64(transfer), float64(sync), int(stampCost), int(stampFlex), int(peeringCost), metadata)
}
