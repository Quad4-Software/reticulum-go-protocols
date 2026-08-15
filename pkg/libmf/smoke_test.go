// SPDX-License-Identifier: 0BSD
package libmf_test

import (
	"testing"

	"quad4/reticulum-go-protocols/pkg/libmf"
)

func TestMFCodecRoundTrip(t *testing.T) {
	sender := make([]byte, 16)
	for i := range sender {
		sender[i] = byte(i)
	}
	data, code := libmf.Pack(sender, "hello mf")
	if code != libmf.OK {
		t.Fatal(code)
	}
	gotSender, text, code := libmf.Unpack(data)
	if code != libmf.OK {
		t.Fatal(code)
	}
	if text != "hello mf" {
		t.Fatalf("text %q", text)
	}
	if len(gotSender) != 16 {
		t.Fatalf("sender len %d", len(gotSender))
	}
}
