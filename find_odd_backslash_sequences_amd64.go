//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
)

//go:noescape
func __find_odd_backslash_sequences()

//go:noescape
func _find_odd_backslash_sequences(p1, p2, p3 unsafe.Pointer) (result uint64)

func find_odd_backslash_sequences(buf []byte, prev_iter_ends_odd_backslash *uint64) uint64 {
	return _find_odd_backslash_sequences(unsafe.Pointer(&buf[0]), unsafe.Pointer(&buf[32]), unsafe.Pointer(prev_iter_ends_odd_backslash))
}
