// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"quad4/reticulum-go-protocols/pkg/lxst/proto"
	"quad4/reticulum-go-protocols/pkg/lxst/rnsnode"
	"quad4/reticulum-go/pkg/identity"
	"quad4/reticulum-go/pkg/transport"
)

const recallTimeout = 10 * time.Second

func defaultIdentityPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "rgesp-dial", "identity"), nil
}

func destFromArgs(flagDest string, args []string) string {
	if len(args) > 0 {
		return normalizeHash(args[0])
	}
	return normalizeHash(flagDest)
}

func normalizeHash(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	return s
}

func resolveRemote(t *transport.Transport, hexHash string) (*identity.Identity, error) {
	raw, err := hex.DecodeString(hexHash)
	if err != nil {
		return nil, err
	}
	candidates := [][]byte{raw}
	if len(raw) == proto.IdentityHashLen {
		candidates = append(candidates, proto.TelephonyHash(raw))
	}
	remote, err := rnsnode.WaitRecall(t, candidates, recallTimeout)
	if err != nil {
		return nil, fmt.Errorf("could not recall identity for %s", hexHash)
	}
	return remote, nil
}
