// SPDX-License-Identifier: 0BSD
package lxmf

import (
	"bytes"
	"testing"

	"quad4/reticulum-go/pkg/identity"
)

func TestDestHashMatchesDeliveryDestination(t *testing.T) {
	id, err := identity.New()
	if err != nil {
		t.Fatal(err)
	}
	want, err := DeliveryHash(id)
	if err != nil {
		t.Fatal(err)
	}
	got := DestHash(id.Hash())
	if !bytes.Equal(got, want) {
		t.Fatalf("DestHash=%x DeliveryHash=%x", got, want)
	}
}

func TestDestHashRejectsShort(t *testing.T) {
	if DestHash([]byte{1, 2, 3}) != nil {
		t.Fatal("expected nil")
	}
}
