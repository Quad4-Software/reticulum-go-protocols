// SPDX-License-Identifier: 0BSD
package rnv

import (
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

// UseCase selects which Reticulum media stack to prefer.
type UseCase int

const (
	UseCaseVoiceOnly UseCase = iota
	UseCaseAV
	UseCaseStills
	UseCaseClips
)

// Stack is a recommended application protocol.
type Stack string

const (
	StackLXST Stack = "lxst"
	StackRNV  Stack = "rnv"
)

// RecommendStack returns lxst for telephony voice UX and rnv for media/A/V.
func RecommendStack(u UseCase) Stack {
	switch u {
	case UseCaseVoiceOnly:
		return StackLXST
	default:
		return StackRNV
	}
}

// ValidateOffer checks a stream offer against local and remote HELLO caps.
func ValidateOffer(local, remote proto.Caps, offer proto.StreamOffer) error {
	if err := proto.ValidateStreamOffer(local, remote, offer); err != nil {
		return ErrInvalidOffer
	}
	return nil
}

// ValidateStillMeta rejects absurd dimensions and sizes.
func ValidateStillMeta(meta proto.StillMeta, maxBytes uint64) error {
	if meta.Width < 0 || meta.Height < 0 || meta.Width > MaxWidth || meta.Height > MaxHeight {
		return ErrBadDimensions
	}
	if maxBytes == 0 {
		maxBytes = MaxStillBytes
	}
	if meta.Size > maxBytes || meta.Size > MaxStillBytes {
		return ErrStillTooLarge
	}
	return nil
}

// ValidateClipMeta rejects oversized clips.
func ValidateClipMeta(meta proto.ClipMeta, maxBytes uint64) error {
	if maxBytes == 0 {
		maxBytes = MaxClipBytes
	}
	if meta.Size > maxBytes || meta.Size > MaxClipBytes {
		return ErrClipTooLarge
	}
	return nil
}
