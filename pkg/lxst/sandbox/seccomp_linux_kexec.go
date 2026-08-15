//go:build linux && amd64

// SPDX-License-Identifier: Apache-2.0
package sandbox

import "golang.org/x/sys/unix"

func kexecFileDenies() []int {
	return []int{unix.SYS_KEXEC_FILE_LOAD}
}
