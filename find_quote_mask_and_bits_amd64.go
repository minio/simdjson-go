//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
)

//go:noescape
func __find_quote_mask_and_bits()

//go:noescape
func _find_quote_mask_and_bits(input unsafe.Pointer, odd_ends uint64, prev_iter_inside_quote, quote_bits, error_mask unsafe.Pointer) (quote_mask uint64)

func find_quote_mask_and_bits(buf []byte, odd_ends uint64, prev_iter_inside_quote, quote_bits, error_mask *uint64) (quote_mask uint64) {

	return _find_quote_mask_and_bits(unsafe.Pointer(&buf[0]), odd_ends, unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(quote_bits), unsafe.Pointer(error_mask))
}
