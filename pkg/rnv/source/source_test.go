// SPDX-License-Identifier: 0BSD
package source_test

import (
	"bytes"
	"testing"

	"quad4/reticulum-go-protocols/pkg/rnv/source"
)

func TestFuncSourceSink(t *testing.T) {
	s := &source.FuncSource{Fn: func() ([]byte, error) { return []byte{1, 2}, nil }}
	b, err := s.Next()
	if err != nil || !bytes.Equal(b, []byte{1, 2}) {
		t.Fatal(err, b)
	}
	var got []byte
	sink := &source.FuncSink{Fn: func(p []byte) error { got = append([]byte(nil), p...); return nil }}
	if err := sink.Write([]byte{3}); err != nil || !bytes.Equal(got, []byte{3}) {
		t.Fatal(err, got)
	}
}
