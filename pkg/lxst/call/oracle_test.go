// SPDX-License-Identifier: Apache-2.0
package call_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/lxst/call"
	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func TestOracleDefaultConfigMatchesLXST(t *testing.T) {
	cfg := call.DefaultConfig()
	if cfg.AppName != "lxst" || cfg.AspectName != "telephony" {
		t.Fatalf("app %s aspect %s", cfg.AppName, cfg.AspectName)
	}
	if cfg.Profile != proto.ProfileQualityMedium {
		t.Fatalf("profile %d", cfg.Profile)
	}
	if cfg.Mode != proto.ModeFullDuplex {
		t.Fatalf("mode %d", cfg.Mode)
	}
	if cfg.RingTime != call.DefaultRingTime || cfg.WaitTime != call.DefaultWaitTime || cfg.ConnectTime != call.DefaultConnectTime {
		t.Fatalf("timings ring %s wait %s connect %s", cfg.RingTime, cfg.WaitTime, cfg.ConnectTime)
	}
	if cfg.AnnounceInterval != call.DefaultAnnounceInterval {
		t.Fatalf("announce %s", cfg.AnnounceInterval)
	}
	t.Log("DEFAULT_CONFIG_PROVED")
}

func TestOracleNewCallAppliesProfile(t *testing.T) {
	c := call.NewCall(nil, call.Config{UseAudio: false, Profile: proto.ProfileBandwidthLow})
	if c.Profile() != proto.ProfileBandwidthLow {
		t.Fatalf("profile %d", c.Profile())
	}
	if c.Status() != proto.StatusAvailable {
		t.Fatalf("status %d", c.Status())
	}
}
