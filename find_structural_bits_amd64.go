//+build !noasm
//+build !appengine

package simdjson

import (
	"unsafe"
)

//go:noescape
func _find_structural_bits(p1, p3 unsafe.Pointer,                                         /* for: find_odd_backslash_sequences()    */
						   prev_iter_inside_quote, quote_bits, error_mask unsafe.Pointer, /* for: find_quoute_mask_and_bits()       */
						   whitespace, structurals_in unsafe.Pointer,                     /* for: find_whitespace_and_structurals() */
						   prev_iter_ends_pseudo_pred unsafe.Pointer,                     /* for: finalize_structurals()            */
						   ) (structurals uint64)

func find_structural_bits(buf []byte, prev_iter_ends_odd_backslash *uint64,
						  prev_iter_inside_quote, error_mask *uint64,
						  structurals uint64,
						  prev_iter_ends_pseudo_pred *uint64) (uint64) {

	quote_bits := uint64(0)
	whitespace := uint64(0)

	return _find_structural_bits(unsafe.Pointer(&buf[0]), unsafe.Pointer(prev_iter_ends_odd_backslash),
		 						 unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(&quote_bits), unsafe.Pointer(error_mask),
								 unsafe.Pointer(&whitespace), unsafe.Pointer(&structurals),
		                         unsafe.Pointer(prev_iter_ends_pseudo_pred))
}
