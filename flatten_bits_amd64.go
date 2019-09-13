//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
	"reflect"
)

//go:noescape
func _flatten_bits(base_ptr, pbase unsafe.Pointer, idx uint64 /* will be downconverted to uint32 in assembly */, bits uint64)

func flatten_bits(base *[]uint32, idx uint64, bits uint64) {

	sh := (*reflect.SliceHeader)(unsafe.Pointer(base))
	size := uint32(sh.Len)

	_flatten_bits(unsafe.Pointer(sh.Data), unsafe.Pointer(&size), idx, bits)

	if int(size) >= sh.Cap {
		panic("Memory corruption -- written beyond slice capacity -- expected capacity to be larger than max values written")
	}
	sh.Len = int(size)
}
