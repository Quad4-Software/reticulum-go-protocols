//go:build !linux

// SPDX-License-Identifier: Apache-2.0
package sandbox

func restrictLandlock(policy) error {
	return nil
}

func landlockAvailable() bool {
	return false
}
