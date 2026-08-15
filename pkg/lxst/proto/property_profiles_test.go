// SPDX-License-Identifier: Apache-2.0
package proto_test

import (
	"testing"

	"quad4/pbt/pkg/pbt"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

var propertyProfiles = []int{
	proto.ProfileBandwidthUltraLow,
	proto.ProfileBandwidthVeryLow,
	proto.ProfileBandwidthLow,
	proto.ProfileQualityMedium,
	proto.ProfileQualityHigh,
	proto.ProfileQualityMax,
	proto.ProfileLatencyLow,
	proto.ProfileLatencyUltraLow,
	proto.DefaultProfile,
}

func TestPropertyFrameSamplesMatchRate(t *testing.T) {
	pbt.Check(t, pbt.ForAll("frame samples = rate * ms / 1000", pbt.IntRange(0, len(propertyProfiles)-1), func(i int) bool {
		p := proto.ProfileParams(propertyProfiles[i])
		if p.SampleRate <= 0 || p.FrameMs <= 0 {
			return false
		}
		if p.Codec == proto.CodecOpus && p.Bitrate < proto.MinBitrate {
			return false
		}
		if p.FrameSamples() != p.SampleRate*p.FrameMs/1000 {
			return false
		}
		if p.PlaybackFrameSamples() != proto.PlaybackSampleRate*p.FrameMs/1000 {
			return false
		}
		return p.MaxBytesPerFrame() >= 8
	}), pbt.WithRuns(40))
}

func TestPropertyPreferredSignalRoundTrip(t *testing.T) {
	pbt.Check(t, pbt.ForAll("profile signal round trip", pbt.IntRange(0, len(propertyProfiles)-1), func(i int) bool {
		p := propertyProfiles[i]
		sig := proto.SignalPreferredProfile(p)
		if !proto.IsPreferredProfile(sig) {
			return false
		}
		return proto.ProfileFromSignal(sig) == p
	}), pbt.WithRuns(40))
}
