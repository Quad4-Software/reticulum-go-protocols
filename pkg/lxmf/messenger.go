// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"quad4/reticulum-go/pkg/common"
	"quad4/reticulum-go/pkg/destination"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/link"
	"quad4/reticulum-go/pkg/packet"
	"quad4/reticulum-go/pkg/transport"
)

// MessageHandler receives one unpacked inbound LXMessage per packet (signature state is on msg).
type MessageHandler func(msg *LXMessage, iface common.NetworkInterface)

// Messenger sends and receives LXMF over a Transport. The destination must be inbound Single.
type Messenger struct {
	transport *transport.Transport
	dest      *destination.Destination

	mu        sync.RWMutex
	handler   MessageHandler
	resolver  SourceResolver
	onRecvErr func(error)

	propLinkMu   sync.Mutex
	propLink     *link.Link
	propLinkNode []byte

	deliveryLinkMu   sync.Mutex
	deliveryLink     *link.Link
	deliveryLinkPeer []byte
}

// NewMessenger registers d's packet callback for inbound LXMF. Use NewDeliveryDestination for lxmf.delivery naming.
func NewMessenger(t *transport.Transport, d *destination.Destination) *Messenger {
	m := &Messenger{
		transport: t,
		dest:      d,
		resolver:  RecallSource,
	}
	d.SetPacketCallback(m.onPacket)
	t.RegisterDestination(d.GetHash(), m)
	return m
}

// NewDeliveryDestination returns the inbound lxmf.delivery destination for id.
func NewDeliveryDestination(id *identity.Identity, t *transport.Transport) (*destination.Destination, error) {
	return destination.New(id, destination.In, destination.Single, AppName, t, "delivery")
}

// NewDeliveryMessenger is NewDeliveryDestination plus NewMessenger.
func NewDeliveryMessenger(id *identity.Identity, t *transport.Transport) (*Messenger, error) {
	dest, err := NewDeliveryDestination(id, t)
	if err != nil {
		return nil, err
	}
	return NewMessenger(t, dest), nil
}

// Destination returns the local RNS destination.
func (m *Messenger) Destination() *destination.Destination {
	return m.dest
}

// DestinationHash returns the local destination hash.
func (m *Messenger) DestinationHash() []byte {
	return m.dest.GetHash()
}

// SetMessageHandler sets the inbound callback; nil disables delivery.
func (m *Messenger) SetMessageHandler(h MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = h
}

// SetSourceResolver sets signature verification lookup; default is RecallSource.
func (m *Messenger) SetSourceResolver(r SourceResolver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r == nil {
		m.resolver = RecallSource
		return
	}
	m.resolver = r
}

func (m *Messenger) SetReceiveError(fn func(error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onRecvErr = fn
}

// Compose builds an outbound message from this destination as source.
func (m *Messenger) Compose(destinationHash []byte, title, content string, fields map[byte]any) (*LXMessage, error) {
	return NewMessage(destinationHash, m.DestinationHash(), []byte(title), []byte(content), fields)
}

// Send packs, signs, and sends one opportunistic encrypted packet. The peer must be in identity.Recall.
func (m *Messenger) Send(msg *LXMessage) error {
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

	target, err := destination.FromHash(msg.DestinationHash, remoteIdentity, destination.Single, m.transport)
	if err != nil {
		return fmt.Errorf("create target destination: %w", err)
	}

	signer := m.dest.GetIdentity()
	if signer == nil {
		return errors.New("lxmf: local destination has no identity")
	}

	if _, err := msg.Pack(signer); err != nil {
		return err
	}

	innerPayload, err := msg.EncryptedPayload()
	if err != nil {
		return err
	}

	encrypted, err := target.Encrypt(innerPayload)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	pkt := packet.NewPacket(
		packet.DestinationSingle,
		encrypted,
		packet.PacketTypeData,
		packet.ContextNone,
		packet.PropagationBroadcast,
		packet.HeaderType1,
		nil,
		true,
		packet.FlagUnset,
	)
	pkt.DestinationHash = append([]byte(nil), msg.DestinationHash...)

	if err := pkt.Pack(); err != nil {
		return fmt.Errorf("packet packing failed: %w", err)
	}

	if err := m.transport.SendPacket(pkt); err != nil {
		return fmt.Errorf("packet sending failed: %w", err)
	}

	msg.Method = MethodOpportunistic
	msg.Representation = RepresentationPacket
	msg.State = StateSent
	return nil
}

// SendText composes and sends a text-only message.
func (m *Messenger) SendText(destinationHash []byte, title, content string) (*LXMessage, error) {
	msg, err := m.Compose(destinationHash, title, content, nil)
	if err != nil {
		return nil, err
	}
	if err := m.Send(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// SendStamped packs the message, generates a PoW stamp for stampCost, and sends opportunistically.
func (m *Messenger) SendStamped(msg *LXMessage, stampCost int) error {
	return m.SendStampedContext(context.Background(), msg, stampCost)
}

func (m *Messenger) SendStampedContext(ctx context.Context, msg *LXMessage, stampCost int) error {
	if msg == nil {
		return errors.New("lxmf: nil message")
	}
	if stampCost <= 0 {
		return m.Send(msg)
	}
	signer := m.dest.GetIdentity()
	if signer == nil {
		return errors.New("lxmf: local destination has no identity")
	}
	if _, err := msg.Pack(signer); err != nil {
		return fmt.Errorf("pre-pack: %w", err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
	}
	stamp, value, err := GenerateStamp(ctx, msg.Hash, stampCost, WorkblockExpandRounds)
	if err != nil {
		return fmt.Errorf("stamp generation: %w", err)
	}
	msg.Stamp = stamp
	msg.StampValue = value
	msg.StampValid = true
	return m.Send(msg)
}

// SendPropagated uploads a packed message to propNodeHash via an RNS link.
func (m *Messenger) SendPropagated(msg *LXMessage, propNodeHash []byte, pnStampCost int) error {
	if err := m.packForPropagation(msg, pnStampCost); err != nil {
		return err
	}

	lnk, err := m.ensurePropagationLink(propNodeHash)
	if err != nil {
		return err
	}
	if err := m.sendPropagationPayload(lnk, msg.PropagationPacked); err != nil {
		return err
	}

	msg.Method = MethodPropagated
	msg.Representation = RepresentationPacket
	if len(msg.PropagationPacked) > LinkPacketMaxContent {
		msg.Representation = RepresentationResource
	}
	msg.State = StateSent
	return nil
}

// SendStampedPropagated is SendPropagated after generating a delivery stamp.
func (m *Messenger) SendStampedPropagated(msg *LXMessage, propNodeHash []byte, stampCost, pnStampCost int) error {
	if msg == nil {
		return errors.New("lxmf: nil message")
	}
	signer := m.dest.GetIdentity()
	if signer == nil {
		return errors.New("lxmf: local destination has no identity")
	}
	if _, err := msg.Pack(signer); err != nil {
		return fmt.Errorf("pre-pack: %w", err)
	}
	if stampCost > 0 {
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
	}
	return m.SendPropagated(msg, propNodeHash, pnStampCost)
}

// Receive implements inbound delivery for the lxmf.delivery destination, decrypts,
// sends a delivery proof to the sender, and dispatches the LXMF payload.
func (m *Messenger) Receive(pkt *packet.Packet, iface common.NetworkInterface) bool {
	if pkt == nil {
		return false
	}
	if pkt.PacketType == packet.PacketTypeLinkReq {
		return m.dest.Receive(pkt, iface)
	}

	plaintext, err := m.decryptInbound(pkt.Data)
	if err != nil {
		Warning("inbound lxmf decrypt failed", "error", err, "packet_len", len(pkt.Data))
		m.receiveError(fmt.Errorf("decrypt: %w", err))
		return false
	}

	if err := sendDeliveryProof(m.dest, pkt, iface); err != nil {
		Warning("inbound lxmf proof failed", "error", err)
	}

	m.onPacket(plaintext, iface)
	return true
}

// EnableRatchets enables destination ratchet keys for inbound decryption and persistence.
func (m *Messenger) EnableRatchets(path string) bool {
	return m.dest.EnableRatchets(path)
}

func (m *Messenger) decryptInbound(ciphertext []byte) ([]byte, error) {
	id := m.dest.GetIdentity()
	if id == nil {
		return nil, errors.New("no identity available for decryption")
	}

	ratchets := m.dest.GetRatchets()
	if idRatchets := id.GetRatchets(); len(idRatchets) > 0 {
		ratchets = append(ratchets, idRatchets...)
	}

	receiver := &common.RatchetIDReceiver{}
	return id.Decrypt(ciphertext, ratchets, false, receiver)
}

func (m *Messenger) onPacket(plaintext []byte, iface common.NetworkInterface) {
	m.mu.RLock()
	handler := m.handler
	resolver := m.resolver
	m.mu.RUnlock()

	if len(plaintext) < DestinationLength+SignatureLength {
		m.receiveError(fmt.Errorf("inbound: %w", ErrMessageTooShort))
		return
	}

	if handler == nil {
		return
	}

	msg, err := UnpackFromBytes(m.DestinationHash(), plaintext, resolver)
	if err != nil && msg == nil {
		Warning("inbound lxmf unpack failed", "error", err, "plaintext_len", len(plaintext))
		m.receiveError(fmt.Errorf("unpack: %w", err))
		return
	}
	if err != nil {
		Debug("inbound lxmf unpack completed with error", "error", err,
			"signature_validated", msg.SignatureValidated, "unverified_reason", msg.UnverifiedReason)
	}

	handler(msg, iface)
}

func (m *Messenger) receiveError(err error) {
	if err == nil {
		return
	}
	m.mu.RLock()
	fn := m.onRecvErr
	m.mu.RUnlock()
	if fn != nil {
		fn(err)
	}
}
