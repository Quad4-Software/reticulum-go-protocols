// SPDX-License-Identifier: 0BSD
package session

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"

	"quad4/reticulum-go-protocols/pkg/rnv"
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

// SendStill sends a JPEG (or registered codec) still image.
func (c *Conn) SendStill(ctx context.Context, data []byte, meta proto.StillMeta) error {
	if err := c.requireHandshake(); err != nil {
		return err
	}
	if !c.rateStillOK() {
		return rnv.ErrRateLimited
	}
	if meta.Codec == 0 {
		meta.Codec = proto.CodecJPEG
	}
	meta.Size = uint64(len(data))
	max := c.effectiveStillMax()
	if err := rnv.ValidateStillMeta(meta, max); err != nil {
		return err
	}
	if uint64(len(data)) > max {
		return rnv.ErrStillTooLarge
	}
	if meta.Width == 0 || meta.Height == 0 {
		if meta.Codec == proto.CodecJPEG {
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err == nil {
				meta.Width = cfg.Width
				meta.Height = cfg.Height
			}
		}
	}
	if err := rnv.ValidateStillMeta(meta, max); err != nil {
		return err
	}
	if len(meta.ID) == 0 {
		meta.ID = contentHash(data)
	}

	// Prefer inline CBOR data when the packed envelope stays under the soft link budget.
	meta.Transfer = proto.TransferPacket
	meta.Data = data
	env := proto.NewTyped(proto.TypeStill, meta.ToBody())
	packed, err := env.Pack()
	if err != nil {
		return err
	}
	if len(packed) <= proto.LinkPacketSoftMax {
		return c.sendRaw(packed)
	}

	meta.Transfer = proto.TransferResource
	meta.Data = nil
	if err := c.sendEnvelope(proto.NewTyped(proto.TypeStill, meta.ToBody())); err != nil {
		return err
	}
	timeout := c.ep.cfg.StillTimeout
	if ctx == nil {
		ctx = context.Background()
	}
	return c.sendPayload(ctx, data, timeout)
}

func (c *Conn) handleStillMeta(meta proto.StillMeta) {
	max := c.effectiveStillMax()
	data := meta.Data
	meta.Data = nil
	if err := rnv.ValidateStillMeta(meta, max); err != nil {
		_ = c.sendReject(proto.RejectSize, err.Error())
		return
	}
	if meta.Transfer == proto.TransferPacket && len(data) > 0 {
		if h := c.ep.cfg.Handlers.OnStill; h != nil {
			h(c, meta, data)
		}
		return
	}
	if meta.Transfer == proto.TransferResource {
		c.mu.Lock()
		c.pendingStillID = cloneHash(meta.ID)
		c.mu.Unlock()
	}
}
