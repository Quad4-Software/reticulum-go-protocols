// SPDX-License-Identifier: Apache-2.0
package proto_test

import (
	"bytes"
	"testing"

	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func TestOracleAvailableWire(t *testing.T) {
	raw, err := proto.PackSignalling([]int{proto.StatusAvailable})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x81, 0x00, 0x91, 0x03}
	if !bytes.Equal(raw, want) {
		t.Fatalf("AVAILABLE wire %x want %x", raw, want)
	}
	t.Log("AVAILABLE_WIRE_PROVED")
}

func TestOracleBusyWire(t *testing.T) {
	raw, err := proto.PackSignalling([]int{proto.StatusBusy})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x81, 0x00, 0x91, 0x00}
	if !bytes.Equal(raw, want) {
		t.Fatalf("BUSY wire %x want %x", raw, want)
	}
	t.Log("BUSY_WIRE_PROVED")
}

func TestOracleEstablishedWire(t *testing.T) {
	raw, err := proto.PackSignalling([]int{proto.StatusEstablished})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x81, 0x00, 0x91, 0x06}
	if !bytes.Equal(raw, want) {
		t.Fatalf("ESTABLISHED wire %x want %x", raw, want)
	}
}

func TestOraclePreferredProfileMedium(t *testing.T) {
	sig := proto.SignalPreferredProfile(proto.ProfileQualityMedium)
	if sig != proto.PreferredProfile+proto.ProfileQualityMedium {
		t.Fatalf("signal %d", sig)
	}
	if proto.ProfileFromSignal(sig) != proto.ProfileQualityMedium {
		t.Fatalf("profile %d", proto.ProfileFromSignal(sig))
	}
}
