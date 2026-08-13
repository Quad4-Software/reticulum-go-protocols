// SPDX-License-Identifier: 0BSD
package rrc

import (
	"bytes"
	"testing"
)

func BenchmarkRRC_Marshal(b *testing.B) {
	sender := bytes.Repeat([]byte{0x9c}, IdentityLength)
	env, err := NewEnvelope(TypeMsg, sender)
	if err != nil {
		b.Fatal(err)
	}
	env.Room = "#lobby"
	env.HasRoom = true
	env.Body = "Hello, RRC benchmark."
	env.HasBody = true
	env.Nick = "alice"
	env.HasNick = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := env.Marshal(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRRC_Unmarshal(b *testing.B) {
	sender := bytes.Repeat([]byte{0x9c}, IdentityLength)
	env, err := NewEnvelope(TypeMsg, sender)
	if err != nil {
		b.Fatal(err)
	}
	env.Room = "#lobby"
	env.HasRoom = true
	env.Body = "Hello, RRC benchmark."
	env.HasBody = true
	raw, err := env.Marshal()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := UnmarshalEnvelope(raw); err != nil {
			b.Fatal(err)
		}
	}
}
