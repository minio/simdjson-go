//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
	"reflect"
)

//go:noescape
func _parse_string(src, dst, pcurrent_string_buf_loc unsafe.Pointer)

func parse_string_simd(buf []byte, stringbuf *[]byte) uintptr {

	sh := (*reflect.SliceHeader)(unsafe.Pointer(stringbuf))

	src := uintptr(unsafe.Pointer(&buf[0])) + 1 // const uint8_t *src = &buf[offset + 1];
	dst := unsafe.Pointer(sh.Data + 4)          // uint8_t *dst = pj.current_string_buf_loc + sizeof(uint32_t);
	string_buf_loc := unsafe.Pointer(sh.Data)

	_parse_string(unsafe.Pointer(src), dst, unsafe.Pointer(&string_buf_loc))

	//if int(size) >= sh.Cap {
	//	panic("Memory corruption -- written beyond slice capacity -- expected capacity to be larger than max values written")
	//}
	//sh.Len = int(size)

	return uintptr(string_buf_loc) - sh.Data
}
