//go:build linux && musl

// SPDX-License-Identifier: Apache-2.0
package sandbox

func muslTagged() bool { return true }
