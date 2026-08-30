// SPDX-License-Identifier: 0BSD
package session

import (
	"quad4/reticulum-go-protocols/pkg/rnv"
	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

// GuardStreamOffer applies capacity and profile footguns before OpenStream.
func GuardStreamOffer(local, remote proto.Caps, offer proto.StreamOffer) error {
	if offer.Tracks == 0 {
		return rnv.ErrInvalidOffer
	}
	if offer.Tracks&proto.TrackVideo != 0 && (offer.Profile == 0 || offer.Profile < proto.ProfileMedium) {
		return rnv.ErrVideoTrackDenied
	}
	eff := offer.Profile
	if eff == 0 {
		eff = EffectiveProfile(local, remote, 0)
	}
	lim := proto.LimitsFor(eff)
	if offer.Tracks&proto.TrackVideo != 0 && !lim.AllowVideo {
		return rnv.ErrCapacity
	}
	if offer.Tracks&proto.TrackAudio != 0 && !lim.AllowAudio {
		return rnv.ErrAudioTrackDenied
	}
	if err := proto.ValidateStreamOffer(local, remote, offer); err != nil {
		return rnv.ErrInvalidOffer
	}
	return nil
}

// EffectiveProfile returns min(local preferred, remote preferred, offer).
func EffectiveProfile(local, remote proto.Caps, offerProfile int) int {
	eff := proto.MinProfile(local.Preferred, remote.Preferred)
	if offerProfile != 0 {
		eff = proto.MinProfile(eff, offerProfile)
	}
	return eff
}
