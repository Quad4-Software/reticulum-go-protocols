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

	"quad4/reticulum-go-protocols/pkg/liblxst"
)

var versionCString *C.char

func init() {
	versionCString = C.CString(liblxst.Version())
}

//export lxst_version
func lxst_version() *C.char { return versionCString }

//export lxst_last_error
func lxst_last_error(buf *C.char, bufLen C.size_t, written *C.size_t) C.int {
	msg := liblxst.LastError()
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

//export lxst_pack_signalling
func lxst_pack_signalling(signals *C.int, signalCount C.size_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	if signalCount == 0 || signals == nil {
		_, code := liblxst.PackSignalling(nil)
		return C.int(code)
	}
	hdr := unsafe.Slice((*C.int)(unsafe.Pointer(signals)), int(signalCount))
	sigs := make([]int, len(hdr))
	for i, v := range hdr {
		sigs[i] = int(v)
	}
	data, code := liblxst.PackSignalling(sigs)
	if code != liblxst.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxst_pack_frame
func lxst_pack_frame(codec C.uint8_t, payload *C.uint8_t, payloadLen C.size_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	raw, code := goBytes(payload, payloadLen)
	if code != 0 {
		return C.int(code)
	}
	data, code := liblxst.PackFrame(byte(codec), raw)
	if code != liblxst.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxst_unpack
func lxst_unpack(data *C.uint8_t, dataLen C.size_t) C.uint64_t {
	raw, code := goBytes(data, dataLen)
	if code != 0 {
		return 0
	}
	handle, code := liblxst.Unpack(raw)
	if code != liblxst.OK {
		return 0
	}
	return C.uint64_t(handle)
}

//export lxst_packet_destroy
func lxst_packet_destroy(packet C.uint64_t) C.int {
	return C.int(liblxst.PacketDestroy(uint64(packet)))
}

//export lxst_packet_signal_count
func lxst_packet_signal_count(packet C.uint64_t, count *C.size_t) C.int {
	n, code := liblxst.PacketSignalCount(uint64(packet))
	if code != liblxst.OK {
		return C.int(code)
	}
	if count != nil {
		*count = C.size_t(n)
	}
	return 0
}

//export lxst_packet_signal_at
func lxst_packet_signal_at(packet C.uint64_t, index C.size_t, signal *C.int) C.int {
	v, code := liblxst.PacketSignalAt(uint64(packet), int(index))
	if code != liblxst.OK {
		return C.int(code)
	}
	if signal != nil {
		*signal = C.int(v)
	}
	return 0
}

//export lxst_packet_frame_count
func lxst_packet_frame_count(packet C.uint64_t, count *C.size_t) C.int {
	n, code := liblxst.PacketFrameCount(uint64(packet))
	if code != liblxst.OK {
		return C.int(code)
	}
	if count != nil {
		*count = C.size_t(n)
	}
	return 0
}

//export lxst_packet_frame_at
func lxst_packet_frame_at(packet C.uint64_t, index C.size_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	data, code := liblxst.PacketFrameAt(uint64(packet), int(index))
	if code != liblxst.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxst_split_frame
func lxst_split_frame(frame *C.uint8_t, frameLen C.size_t, codecOut *C.uint8_t,
	payloadOut *C.uint8_t, payloadOutLen C.size_t, payloadWritten *C.size_t) C.int {
	raw, code := goBytes(frame, frameLen)
	if code != 0 {
		return C.int(code)
	}
	codec, payload, code := liblxst.SplitFrame(raw)
	if code != liblxst.OK {
		return C.int(code)
	}
	if codecOut != nil {
		*codecOut = C.uint8_t(codec)
	}
	return C.int(writeBytes(payloadOut, payloadOutLen, payloadWritten, payload))
}

//export lxst_telephony_hash
func lxst_telephony_hash(identityHash *C.uint8_t, identityLen C.size_t, out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	raw, code := goBytes(identityHash, identityLen)
	if code != 0 {
		return C.int(code)
	}
	data, code := liblxst.TelephonyHash(raw)
	if code != liblxst.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxst_dest_hash
func lxst_dest_hash(identityHash *C.uint8_t, identityLen C.size_t, appName, aspect *C.char,
	out *C.uint8_t, outLen C.size_t, written *C.size_t) C.int {
	raw, code := goBytes(identityHash, identityLen)
	if code != 0 {
		return C.int(code)
	}
	data, code := liblxst.DestHash(raw, C.GoString(appName), C.GoString(aspect))
	if code != liblxst.OK {
		return C.int(code)
	}
	return C.int(writeBytes(out, outLen, written, data))
}

//export lxst_signal_preferred_mode
func lxst_signal_preferred_mode(mode C.int) C.int {
	return C.int(liblxst.SignalPreferredMode(int(mode)))
}

//export lxst_signal_preferred_profile
func lxst_signal_preferred_profile(profile C.int) C.int {
	return C.int(liblxst.SignalPreferredProfile(int(profile)))
}

//export lxst_profile_from_name
func lxst_profile_from_name(name *C.char) C.int {
	return C.int(liblxst.ProfileFromName(C.GoString(name)))
}

func goBytes(ptr *C.uint8_t, n C.size_t) ([]byte, int) {
	if n == 0 {
		return nil, 0
	}
	if ptr == nil {
		return nil, liblxst.ErrInvalidArg
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
			return liblxst.ErrTruncated
		}
		return liblxst.OK
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
		return liblxst.ErrTruncated
	}
	return liblxst.OK
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
