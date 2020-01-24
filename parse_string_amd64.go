//+build !noasm
//+build !appengine

package simdjson

import (
	"reflect"
	"unsafe"
)

//go:noescape
func _parse_string_validate_only(src, maxStringSize, str_length, dst_length unsafe.Pointer) (result uint64)

//go:noescape
func _parse_string(src, dst, pcurrent_string_buf_loc unsafe.Pointer) (res uint64)

func parse_string_simd_validate_only(buf []byte, maxStringSize, dst_length *uint64, need_copy *bool) bool {

	src := uintptr(unsafe.Pointer(&buf[0])) + 1 // Advance buffer by one in order to skip opening quote
	src_length := uint64(0)

	success := _parse_string_validate_only(unsafe.Pointer(src), unsafe.Pointer(&maxStringSize), unsafe.Pointer(&src_length), unsafe.Pointer(dst_length))

	*need_copy = src_length != *dst_length
	return success != 0
}

func parse_string_simd(buf []byte, stringbuf *[]byte) bool {

	sh := (*reflect.SliceHeader)(unsafe.Pointer(stringbuf))

	src := uintptr(unsafe.Pointer(&buf[0])) + 1 // Advance buffer by one in order to skip opening quote
	string_buf_loc := uintptr(unsafe.Pointer(sh.Data)) + uintptr(sh.Len)
	dst := string_buf_loc

	res := _parse_string(unsafe.Pointer(src), unsafe.Pointer(dst), unsafe.Pointer(&string_buf_loc))

	sh.Len += int(uintptr(string_buf_loc) - dst)

	return res != 0
}
