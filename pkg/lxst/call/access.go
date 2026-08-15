// SPDX-License-Identifier: Apache-2.0
package call

import (
	"bytes"
	"encoding/hex"
	"strings"

	"quad4/reticulum-go-protocols/pkg/lxst/phonebook"
	"quad4/reticulum-go/pkg/identity"
)

const fingerprintGroup = 4

func (c *Call) callerAllowed(id *identity.Identity) bool {
	if id == nil {
		return false
	}
	hash := id.Hash()
	for _, b := range c.cfg.Blocked {
		if bytes.Equal(b, hash) {
			return false
		}
	}
	if c.cfg.AllowFunc != nil {
		return c.cfg.AllowFunc(hash)
	}
	if c.cfg.AllowPolicy == phonebook.AllowNone {
		return false
	}
	if c.cfg.AllowPolicy == phonebook.AllowAll {
		return true
	}
	if c.cfg.AllowPolicy == 0 && len(c.cfg.Allowed) == 0 {
		return true
	}
	for _, a := range c.cfg.Allowed {
		if bytes.Equal(a, hash) {
			return true
		}
	}
	return false
}

// Fingerprint groups a 16-byte identity hash for read-back at RINGING.
func Fingerprint(id *identity.Identity) string {
	if id == nil {
		return ""
	}
	return FormatHash(id.Hash())
}

func FormatHash(hash []byte) string {
	if len(hash) == 0 {
		return ""
	}
	hexs := strings.ToLower(hex.EncodeToString(hash))
	var b strings.Builder
	for i := 0; i < len(hexs); i += fingerprintGroup {
		if i > 0 {
			b.WriteByte(' ')
		}
		end := min(i+fingerprintGroup, len(hexs))
		b.WriteString(hexs[i:end])
	}
	return b.String()
}
