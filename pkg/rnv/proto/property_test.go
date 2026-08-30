// SPDX-License-Identifier: 0BSD
package proto_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv/proto"
)

func TestPropertyMinProfile(t *testing.T) {
	profiles := []int{proto.ProfileUltraLow, proto.ProfileLow, proto.ProfileMedium, proto.ProfileHigh}
	for i := range profiles {
		for j := range profiles {
			a, b := profiles[i], profiles[j]
			m := proto.MinProfile(a, b)
			if m > a || m > b {
				t.Fatalf("min(%x,%x)=%x", a, b, m)
			}
		}
	}
}
