// SPDX-License-Identifier: 0BSD
package rnv

import "quad4/reticulum-go-protocols/pkg/rnv/proto"

const (
	AppName         = proto.AppName
	MediaAspect     = proto.MediaAspect
	ProtocolVersion = proto.ProtocolVersion
)

// Absolute package limits. Apps may tighten via Config but may not raise
// without DangerousRaiseLimits.
const (
	MaxStillBytes       = proto.MaxStillBytes
	MaxClipBytes        = proto.MaxClipBytes
	MaxStreamFrameBytes = proto.MaxStreamFrameBytes
	MaxAudioFrameBytes  = proto.MaxAudioFrameBytes
	MaxEnvelopeBytes    = proto.MaxEnvelopeBytes
	MaxWidth            = proto.MaxWidth
	MaxHeight           = proto.MaxHeight
	IdentityHashLen     = proto.IdentityHashLen
)
