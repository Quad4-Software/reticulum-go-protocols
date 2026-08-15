//go:build cgo

package capi

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"quad4/reticulum-go-protocols/pkg/libmf"
)

var versionCString *C.char

func init() {
	versionCString = C.CString(libmf.Version())
}

//export mf_version
func mf_version() *C.char { return versionCString }

//export mf_last_error
func mf_last_error(buf *C.char, bufLen C.size_t, written *C.size_t) C.int {
	if written != nil {
		*written = 0
	}
	msg := libmf.LastError()
	if buf == nil || bufLen == 0 {
		if written != nil {
			*written = C.size_t(len(msg))
		}
		if len(msg) > 0 {
			return 8
		}
		return 0
	}
	n := copyCString(buf, bufLen, msg)
	if written != nil {
		*written = C.size_t(n)
	}
	if n < len(msg) {
		return 8
	}
	return 0
}

//export mf_pack
func mf_pack(sender *C.uint8_t, senderLen C.size_t, text *C.char, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	s, code := goBytes(sender, senderLen)
	if code != 0 {
		return C.int(code)
	}
	data, code := libmf.Pack(s, C.GoString(text))
	if code != libmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export mf_unpack
func mf_unpack(data *C.uint8_t, dataLen C.size_t,
	senderOut *C.uint8_t, senderOutLen C.size_t, senderWritten *C.size_t,
	textOut *C.char, textOutLen C.size_t, textWritten *C.size_t) C.int {
	raw, code := goBytes(data, dataLen)
	if code != 0 {
		return C.int(code)
	}
	sender, text, code := libmf.Unpack(raw)
	if code != libmf.OK {
		return C.int(code)
	}
	if c := writeBytes(senderOut, senderOutLen, senderWritten, sender); c != 0 {
		return C.int(c)
	}
	return C.int(writeString(textOut, textOutLen, textWritten, text))
}

func goBytes(ptr *C.uint8_t, n C.size_t) ([]byte, int) {
	if n == 0 {
		return nil, 0
	}
	if ptr == nil {
		return nil, libmf.ErrInvalidArg
	}
	return C.GoBytes(unsafe.Pointer(ptr), C.int(n)), 0
}

func writeBytes(out *C.uint8_t, outLen C.size_t, written *C.size_t, src []byte) int {
	if written != nil {
		*written = 0
	}
	if out == nil || outLen == 0 {
		if written != nil {
			*written = C.size_t(len(src))
		}
		if len(src) > 0 {
			return libmf.ErrTruncated
		}
		return libmf.OK
	}
	limit := int(outLen)
	n := len(src)
	if n > limit {
		n = limit
	}
	if n > 0 {
		C.memcpy(unsafe.Pointer(out), unsafe.Pointer(&src[0]), C.size_t(n))
	}
	if written != nil {
		*written = C.size_t(n)
	}
	if n < len(src) {
		return libmf.ErrTruncated
	}
	return libmf.OK
}

func writeString(buf *C.char, bufLen C.size_t, written *C.size_t, s string) int {
	if written != nil {
		*written = 0
	}
	if buf == nil || bufLen == 0 {
		if written != nil {
			*written = C.size_t(len(s))
		}
		if len(s) > 0 {
			return libmf.ErrTruncated
		}
		return libmf.OK
	}
	n := copyCString(buf, bufLen, s)
	if written != nil {
		*written = C.size_t(n)
	}
	if n < len(s) {
		return libmf.ErrTruncated
	}
	return libmf.OK
}

func copyCString(dst *C.char, capacity C.size_t, s string) int {
	limit := int(capacity)
	if limit == 0 {
		return 0
	}
	room := limit - 1
	n := len(s)
	if n > room {
		n = room
	}
	if n > 0 {
		C.memcpy(unsafe.Pointer(dst), unsafe.Pointer(unsafe.StringData(s)), C.size_t(n))
	}
	p := unsafe.Slice((*byte)(unsafe.Pointer(dst)), limit)
	p[n] = 0
	return n
}

var _ = sync.Mutex{}
