// SPDX-License-Identifier: Apache-2.0
package lxsttest

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Python returns an interpreter that can import LXST and RNS.
// Missing LXST skips unless REQUIRE_LXST=1, which fails the test.
func Python(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("LXST_PYTHON"),
		filepath.Join(os.Getenv("HOME"), ".local/share/pipx/venvs/lxst/bin/python"),
		"python3",
		"python",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		cmd := exec.Command(c, "-c", "from RNS.vendor import umsgpack; import LXST, RNS; from LXST._version import __version__; raise SystemExit(0 if __version__=='0.5.1' else 1)")
		if err := cmd.Run(); err == nil {
			return c
		}
	}
	if os.Getenv("REQUIRE_LXST") == "1" {
		t.Fatal("python LXST 0.5.1 required (set LXST_PYTHON or install lxst)")
	}
	t.Skip("python LXST not available")
	return ""
}

// SkipLive skips long UDP/shared-instance calls under -short unless REQUIRE_LXST=1.
func SkipLive(t *testing.T) {
	t.Helper()
	if os.Getenv("REQUIRE_LXST") == "1" {
		return
	}
	if testing.Short() {
		t.Skip("live python interop")
	}
}

// RepoRoot walks up from the test working directory to go.mod.
func RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for range 8 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("go.mod not found")
	return ""
}
