// SPDX-License-Identifier: 0BSD
package mf

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const hashGroup = 4

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
	if len(raw) != SenderHashLength {
		return nil, fmt.Errorf("%w: length %d", ErrInvalidHash, len(raw))
	}
	return raw, nil
}

func FormatHash(hash []byte) string {
	if len(hash) == 0 {
		return ""
	}
	hexs := strings.ToLower(hex.EncodeToString(hash))
	var b strings.Builder
	for i := 0; i < len(hexs); i += hashGroup {
		if i > 0 {
			b.WriteByte(' ')
		}
		end := min(i+hashGroup, len(hexs))
		b.WriteString(hexs[i:end])
	}
	return b.String()
}
