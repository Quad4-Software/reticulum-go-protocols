//go:build cgo && (linux || darwin || windows || android || freebsd || openbsd || netbsd || dragonfly)

// SPDX-License-Identifier: Apache-2.0
package io_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/lxst/audio/io"
)

func TestBackendMiniaudio(t *testing.T) {
	if io.Backend() != "miniaudio" {
		t.Fatalf("backend %q", io.Backend())
	}
}
