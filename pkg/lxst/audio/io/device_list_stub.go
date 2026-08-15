//go:build !cgo || !(linux || darwin || windows || android || freebsd || openbsd || netbsd || dragonfly)

// SPDX-License-Identifier: Apache-2.0
//
//revive:disable:var-naming
package io

type DeviceInfo struct {
	Name    string
	Capture bool
}

func ListDevices() ([]DeviceInfo, error) {
	return nil, nil
}
