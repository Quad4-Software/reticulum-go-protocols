// SPDX-License-Identifier: 0BSD
package proto

// ProfileLimits are hard bounds for a negotiated profile.
type ProfileLimits struct {
	StillMax    uint64
	ClipMax     uint64
	AllowVideo  bool
	AllowAudio  bool
	MaxFPS      int
	AudioCodecs []byte
	VideoCodecs []byte
}

// LimitsFor returns limits for a profile constant.
func LimitsFor(profile int) ProfileLimits {
	switch profile {
	case ProfileUltraLow:
		return ProfileLimits{
			StillMax:    UltraLowStillMax,
			ClipMax:     0,
			AllowVideo:  false,
			AllowAudio:  false,
			MaxFPS:      0,
			VideoCodecs: []byte{CodecJPEG},
		}
	case ProfileLow:
		return ProfileLimits{
			StillMax:    LowStillMax,
			ClipMax:     LowClipMax,
			AllowVideo:  false,
			AllowAudio:  true,
			MaxFPS:      0,
			AudioCodecs: []byte{CodecCodec2},
			VideoCodecs: []byte{CodecJPEG},
		}
	case ProfileMedium:
		return ProfileLimits{
			StillMax:    MaxStillBytes,
			ClipMax:     MediumClipMax,
			AllowVideo:  true,
			AllowAudio:  true,
			MaxFPS:      MediumMaxFPS,
			AudioCodecs: []byte{CodecOpus, CodecCodec2},
			VideoCodecs: []byte{CodecJPEG},
		}
	case ProfileHigh:
		return ProfileLimits{
			StillMax:    MaxStillBytes,
			ClipMax:     HighClipMax,
			AllowVideo:  true,
			AllowAudio:  true,
			MaxFPS:      HighMaxFPS,
			AudioCodecs: []byte{CodecOpus, CodecCodec2},
			VideoCodecs: []byte{CodecJPEG},
		}
	default:
		return LimitsFor(ProfileLow)
	}
}

// MinProfile returns the more constrained of two profiles.
func MinProfile(a, b int) int {
	if normalizeProfile(a) < normalizeProfile(b) {
		return normalizeProfile(a)
	}
	return normalizeProfile(b)
}

func normalizeProfile(p int) int {
	switch p {
	case ProfileUltraLow, ProfileLow, ProfileMedium, ProfileHigh:
		return p
	default:
		return ProfileLow
	}
}

// SupportsProfile reports whether caps list includes profile.
func SupportsProfile(caps Caps, profile int) bool {
	profile = normalizeProfile(profile)
	if len(caps.Profiles) == 0 {
		return true
	}
	for _, p := range caps.Profiles {
		if normalizeProfile(p) == profile {
			return true
		}
	}
	return false
}

// HasCodec reports whether codec is advertised.
func HasCodec(caps Caps, codec byte) bool {
	if len(caps.Codecs) == 0 {
		return true
	}
	for _, c := range caps.Codecs {
		if c == codec {
			return true
		}
	}
	return false
}

// DefaultCaps returns SafeConfig-oriented capabilities (Low preferred).
func DefaultCaps() Caps {
	return Caps{
		MaxStill:  MaxStillBytes,
		MaxClip:   MaxClipBytes,
		Profiles:  []int{ProfileUltraLow, ProfileLow, ProfileMedium, ProfileHigh},
		Codecs:    []byte{CodecJPEG, CodecOpaque, CodecOpus, CodecCodec2},
		Tracks:    TrackVideo | TrackAudio,
		Preferred: ProfileLow,
	}
}

// ValidateStreamOffer checks offer against local and remote caps and profile physics.
// Preferred is only a default when offer.Profile is zero. An explicit Medium/High
// offer is allowed even when Preferred is Low, if both peers list that profile.
func ValidateStreamOffer(local, remote Caps, offer StreamOffer) error {
	if offer.Tracks == 0 {
		return fmtError("empty track mask")
	}
	eff := normalizeProfile(offer.Profile)
	if offer.Profile == 0 {
		eff = MinProfile(local.Preferred, remote.Preferred)
	}
	if !SupportsProfile(local, eff) || !SupportsProfile(remote, eff) {
		return fmtError("profile not supported by both peers")
	}
	lim := LimitsFor(eff)
	if offer.Tracks&TrackVideo != 0 {
		if !lim.AllowVideo {
			return fmtError("video not allowed at effective profile")
		}
		if local.Tracks&TrackVideo == 0 || remote.Tracks&TrackVideo == 0 {
			return fmtError("video track not advertised")
		}
		vid := offer.Video
		if vid == 0 {
			vid = CodecJPEG
		}
		if !HasCodec(local, vid) || !HasCodec(remote, vid) {
			return fmtError("video codec not supported")
		}
	}
	if offer.Tracks&TrackAudio != 0 {
		if !lim.AllowAudio {
			return fmtError("audio not allowed at effective profile")
		}
		if local.Tracks&TrackAudio == 0 || remote.Tracks&TrackAudio == 0 {
			return fmtError("audio track not advertised")
		}
		aud := offer.Audio
		if aud == 0 {
			if eff == ProfileLow {
				aud = CodecCodec2
			} else {
				aud = CodecOpus
			}
		}
		if eff == ProfileLow && aud != CodecCodec2 {
			return fmtError("low profile audio requires codec2")
		}
		if !HasCodec(local, aud) || !HasCodec(remote, aud) {
			return fmtError("audio codec not supported")
		}
	}
	if offer.MaxFPS > 0 && lim.MaxFPS > 0 && offer.MaxFPS > lim.MaxFPS {
		return fmtError("fps exceeds profile")
	}
	return nil
}

type profileError string

func (e profileError) Error() string { return "rnv proto: " + string(e) }

func fmtError(msg string) error { return profileError(msg) }
