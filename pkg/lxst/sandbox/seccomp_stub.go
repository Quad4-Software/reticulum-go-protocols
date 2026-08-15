//go:build !linux

// SPDX-License-Identifier: Apache-2.0
package sandbox

func restrictSeccomp() error {
	return nil
}

func seccompAvailable() bool {
	return false
}
