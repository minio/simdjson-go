//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
)

//go:noescape
func __finalize_structurals()

//go:noescape
func _finalize_structurals(structurals_in, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred unsafe.Pointer) (structurals uint64)

func finalize_structurals(structurals, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred *uint64) uint64 {
	return _finalize_structurals(structurals, whitespace, quote_mask, quote_bits, unsafe.Pointer(prev_iter_ends_pseudo_pred))
}
