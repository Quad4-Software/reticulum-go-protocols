//go:build linux && amd64

// SPDX-License-Identifier: Apache-2.0
package sandbox

import "golang.org/x/sys/unix"

func extraDenies() []int {
	return []int{unix.SYS_IOPL, unix.SYS_IOPERM}
}
