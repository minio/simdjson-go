//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
)

//go:noescape
func _parse_string_validate_only(src, str_length, dst_length unsafe.Pointer) (result uint64)

func parse_string_simd_validate_only(buf []byte, dst_length *uint64, need_copy *bool) bool {

	src := uintptr(unsafe.Pointer(&buf[0])) + 1 // const uint8_t *src = &buf[offset + 1];
	src_length := uint64(0)

	success := _parse_string_validate_only(unsafe.Pointer(src), unsafe.Pointer(&src_length), unsafe.Pointer(dst_length))

	*need_copy = src_length != *dst_length
	return success != 0
}
