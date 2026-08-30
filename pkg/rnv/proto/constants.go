// SPDX-License-Identifier: 0BSD
package proto

const (
	AppName                = "rnv"
	MediaAspect            = "media"
	ProtocolVersion uint64 = 1

	IdentityHashLen = 16

	FieldVersion    uint64 = 0
	FieldType       uint64 = 1
	FieldBody       uint64 = 2
	FieldExtensions uint64 = 90

	TypeHello        uint64 = 1
	TypeReject       uint64 = 2
	TypeStill        uint64 = 10
	TypeClipOffer    uint64 = 20
	TypeClipAccept   uint64 = 21
	TypeClipDone     uint64 = 22
	TypeStreamOffer  uint64 = 30
	TypeStreamAccept uint64 = 31
	TypeStreamCtrl   uint64 = 32
	TypeBye          uint64 = 40
	TypePrivateMin   uint64 = 1000

	CodecJPEG       byte = 0x01
	CodecAVIF       byte = 0x02
	CodecOpaque     byte = 0x10
	CodecH264       byte = 0x20
	CodecVP8        byte = 0x21
	CodecPCM16      byte = 0x80
	CodecOpus       byte = 0x81
	CodecCodec2     byte = 0x82
	CodecPrivateMin byte = 0xE0
	CodecPrivateMax byte = 0xFE

	MagicVideo byte = 0xF1
	MagicAudio byte = 0xF2

	FlagKeyframe byte = 0x01
	FlagEOS      byte = 0x02

	TrackVideo byte = 0x01
	TrackAudio byte = 0x02

	ProfileUltraLow = 0x10
	ProfileLow      = 0x20
	ProfileMedium   = 0x30
	ProfileHigh     = 0x40

	TransferPacket   = 0
	TransferResource = 1

	RejectCapacity = 1
	RejectSize     = 2
	RejectUnknown  = 3
	RejectPolicy   = 4
	RejectBusy     = 5

	HelloKeyMaxStill   uint64 = 0
	HelloKeyMaxClip    uint64 = 1
	HelloKeyProfiles   uint64 = 2
	HelloKeyCodecs     uint64 = 3
	HelloKeyTracks     uint64 = 4
	HelloKeyPreferred  uint64 = 5
	HelloKeyStrictExt  uint64 = 6
	HelloKeyExtensions uint64 = 7

	StillKeyWidth    uint64 = 0
	StillKeyHeight   uint64 = 1
	StillKeyCodec    uint64 = 2
	StillKeySize     uint64 = 3
	StillKeyTransfer uint64 = 4
	StillKeyID       uint64 = 5
	StillKeyData     uint64 = 6

	ClipKeyID     uint64 = 0
	ClipKeySize   uint64 = 1
	ClipKeyCodec  uint64 = 2
	ClipKeyMime   uint64 = 3
	ClipKeySHA256 uint64 = 4

	StreamKeyProfile uint64 = 0
	StreamKeyTracks  uint64 = 1
	StreamKeyVideo   uint64 = 2
	StreamKeyAudio   uint64 = 3
	StreamKeyMaxFPS  uint64 = 4

	CtrlKeyBitrate  uint64 = 0
	CtrlKeyKeyframe uint64 = 1
	CtrlKeyPause    uint64 = 2

	RejectKeyCode   uint64 = 0
	RejectKeyReason uint64 = 1

	AnnounceKeyVersion  uint64 = 0
	AnnounceKeyCaps     uint64 = 1
	AnnounceKeyProfile  uint64 = 2
	AnnounceKeyExtBloom uint64 = 3

	CapStill  uint64 = 1 << 0
	CapClip   uint64 = 1 << 1
	CapStream uint64 = 1 << 2
	CapAudio  uint64 = 1 << 3

	MaxStillBytes       = 256 << 10
	MaxClipBytes        = 8 << 20
	MaxStreamFrameBytes = 400
	MaxAudioFrameBytes  = 256
	MaxEnvelopeBytes    = 4 << 10
	MaxWidth            = 4096
	MaxHeight           = 4096
	FrameHeaderLen      = 5

	UltraLowStillMax = 32 << 10
	LowStillMax      = 128 << 10
	LowClipMax       = 1 << 20
	MediumClipMax    = 4 << 20
	HighClipMax      = 8 << 20

	MediumMaxFPS = 2
	HighMaxFPS   = 5

	LinkPacketSoftMax = 400
)
