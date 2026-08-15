// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/hex"
	"fmt"
	"strings"

	"quad4/reticulum-go-protocols/pkg/lxst/proto"
)

func ParseHash(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		return nil, fmt.Errorf("%w: empty", ErrInvalidHash)
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHash, err)
	}
	if len(raw) != proto.IdentityHashLen && len(raw) != proto.DestHashLen {
		return nil, fmt.Errorf("%w: length %d", ErrInvalidHash, len(raw))
	}
	return raw, nil
}
