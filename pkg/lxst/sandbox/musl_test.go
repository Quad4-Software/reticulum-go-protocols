//go:build musl

// SPDX-License-Identifier: Apache-2.0
package sandbox

import "testing"

func TestMuslTagApplied(t *testing.T) {
	if !muslTagged() {
		t.Fatal("musl tag not applied")
	}
}
