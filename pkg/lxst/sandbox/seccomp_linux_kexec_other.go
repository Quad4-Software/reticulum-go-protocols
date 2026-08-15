//go:build linux && !amd64 && !386

// SPDX-License-Identifier: Apache-2.0
package sandbox

func kexecFileDenies() []int {
	return nil
}
