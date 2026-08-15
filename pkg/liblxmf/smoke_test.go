// SPDX-License-Identifier: 0BSD
package liblxmf_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/liblxmf"
)

func TestLXMFCodecRoundTrip(t *testing.T) {
	id, code := liblxmf.IdentityGenerate()
	if code != liblxmf.OK {
		t.Fatal(code)
	}
	defer liblxmf.IdentityDestroy(id)

	hash, code := liblxmf.IdentityHash(id)
	if code != liblxmf.OK || len(hash) != 16 {
		t.Fatalf("hash %d len=%d", code, len(hash))
	}

	dest := make([]byte, 16)
	copy(dest, hash)
	source := make([]byte, 16)
	copy(source, hash)

	msg, code := liblxmf.MessageCreate(dest, source, "hi", "body")
	if code != liblxmf.OK {
		t.Fatal(code)
	}
	defer liblxmf.MessageDestroy(msg)

	data, code := liblxmf.MessagePack(msg, id)
	if code != liblxmf.OK || len(data) == 0 {
		t.Fatalf("pack %d len=%d", code, len(data))
	}

	got, code := liblxmf.MessageUnpack(data)
	if code != liblxmf.OK {
		t.Fatal(code)
	}
	defer liblxmf.MessageDestroy(got)

	content, code := liblxmf.MessageGetContent(got)
	if code != liblxmf.OK || content != "body" {
		t.Fatalf("content %q code=%d", content, code)
	}
}
