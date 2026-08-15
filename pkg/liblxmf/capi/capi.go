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

	"quad4/reticulum-go-protocols/pkg/liblxmf"
)

var versionCString *C.char

func init() { versionCString = C.CString(liblxmf.Version()) }

//export lxmf_version
func lxmf_version() *C.char { return versionCString }

//export lxmf_last_error
func lxmf_last_error(buf *C.char, bufLen C.size_t, written *C.size_t) C.int {
	msg := liblxmf.LastError()
	if written != nil {
		*written = 0
	}
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

//export lxmf_identity_generate
func lxmf_identity_generate() C.uint64_t {
	id, _ := liblxmf.IdentityGenerate()
	return C.uint64_t(id)
}

//export lxmf_identity_destroy
func lxmf_identity_destroy(identity C.uint64_t) C.int {
	return C.int(liblxmf.IdentityDestroy(uint64(identity)))
}

//export lxmf_identity_hash
func lxmf_identity_hash(identity C.uint64_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.IdentityHash(uint64(identity))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxmf_identity_public_key
func lxmf_identity_public_key(identity C.uint64_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.IdentityPublicKey(uint64(identity))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxmf_identity_delivery_hash
func lxmf_identity_delivery_hash(identity C.uint64_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.IdentityDeliveryHash(uint64(identity))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxmf_identity_register_recall
func lxmf_identity_register_recall(identity C.uint64_t) C.int {
	return C.int(liblxmf.IdentityRegisterRecall(uint64(identity)))
}

//export lxmf_identity_register_recall_source
func lxmf_identity_register_recall_source(source *C.uint8_t, sourceLen C.size_t, publicKey *C.uint8_t, publicKeyLen C.size_t) C.int {
	src, c := goBytes(source, sourceLen)
	if c != 0 {
		return C.int(c)
	}
	pk, c := goBytes(publicKey, publicKeyLen)
	if c != 0 {
		return C.int(c)
	}
	return C.int(liblxmf.IdentityRegisterRecallSource(src, pk))
}

//export lxmf_message_create
func lxmf_message_create(dest *C.uint8_t, destLen C.size_t, source *C.uint8_t, sourceLen C.size_t, title, content *C.char) C.uint64_t {
	d, c := goBytes(dest, destLen)
	if c != 0 {
		return 0
	}
	s, c := goBytes(source, sourceLen)
	if c != 0 {
		return 0
	}
	id, _ := liblxmf.MessageCreate(d, s, C.GoString(title), C.GoString(content))
	return C.uint64_t(id)
}

//export lxmf_message_pack
func lxmf_message_pack(message, identity C.uint64_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.MessagePack(uint64(message), uint64(identity))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxmf_message_unpack
func lxmf_message_unpack(data *C.uint8_t, dataLen C.size_t) C.uint64_t {
	raw, c := goBytes(data, dataLen)
	if c != 0 {
		return 0
	}
	id, _ := liblxmf.MessageUnpack(raw)
	return C.uint64_t(id)
}

//export lxmf_message_get_dest
func lxmf_message_get_dest(message C.uint64_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.MessageGetDest(uint64(message))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxmf_message_get_source
func lxmf_message_get_source(message C.uint64_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.MessageGetSource(uint64(message))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxmf_message_get_title
func lxmf_message_get_title(message C.uint64_t, buf *C.char, bufLen C.size_t, written *C.size_t) C.int {
	s, code := liblxmf.MessageGetTitle(uint64(message))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeString(buf, bufLen, written, s))
}

//export lxmf_message_get_content
func lxmf_message_get_content(message C.uint64_t, buf *C.char, bufLen C.size_t, written *C.size_t) C.int {
	s, code := liblxmf.MessageGetContent(uint64(message))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeString(buf, bufLen, written, s))
}

//export lxmf_message_set_fields_json
func lxmf_message_set_fields_json(message C.uint64_t, json *C.char) C.int {
	if json == nil {
		return C.int(liblxmf.ErrInvalidArg)
	}
	return C.int(liblxmf.MessageSetFieldsJSON(uint64(message), []byte(C.GoString(json))))
}

//export lxmf_message_fields_json
func lxmf_message_fields_json(message C.uint64_t, buf *C.char, bufLen C.size_t, written *C.size_t) C.int {
	data, code := liblxmf.MessageFieldsJSON(uint64(message))
	if code != liblxmf.OK {
		return C.int(code)
	}
	return C.int(writeString(buf, bufLen, written, string(data)))
}

//export lxmf_message_field_count
func lxmf_message_field_count(message C.uint64_t, count *C.size_t) C.int {
	if count == nil {
		return C.int(liblxmf.ErrInvalidArg)
	}
	n, code := liblxmf.MessageFieldCount(uint64(message))
	if code != liblxmf.OK {
		return C.int(code)
	}
	*count = C.size_t(n)
	return C.int(liblxmf.OK)
}

//export lxmf_message_unpack_verified
func lxmf_message_unpack_verified(data *C.uint8_t, dataLen C.size_t, identity C.uint64_t) C.uint64_t {
	raw, c := goBytes(data, dataLen)
	if c != 0 {
		return 0
	}
	id, _ := liblxmf.MessageUnpackVerified(raw, uint64(identity))
	return C.uint64_t(id)
}

//export lxmf_message_destroy
func lxmf_message_destroy(message C.uint64_t) C.int {
	return C.int(liblxmf.MessageDestroy(uint64(message)))
}

func goBytes(ptr *C.uint8_t, n C.size_t) ([]byte, int) {
	if n == 0 {
		return nil, 0
	}
	if ptr == nil {
		return nil, liblxmf.ErrInvalidArg
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
			return liblxmf.ErrTruncated
		}
		return liblxmf.OK
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
		return liblxmf.ErrTruncated
	}
	return liblxmf.OK
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
			return liblxmf.ErrTruncated
		}
		return liblxmf.OK
	}
	n := copyCString(buf, bufLen, s)
	if written != nil {
		*written = C.size_t(n)
	}
	if n < len(s) {
		return liblxmf.ErrTruncated
	}
	return liblxmf.OK
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
