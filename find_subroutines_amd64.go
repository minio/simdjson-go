//+build !noasm
//+build !appengine
//+build gc

/*
 * MinIO Cloud Storage, (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package simdjson

import (
	"unsafe"
)

//go:noescape
func __finalize_structurals()

//go:noescape
func __finalize_structurals_avx512()

//go:noescape
func _finalize_structurals(structurals_in, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred unsafe.Pointer) (structurals uint64)

func finalize_structurals(structurals, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred *uint64) uint64 {
	return _finalize_structurals(structurals, whitespace, quote_mask, quote_bits, unsafe.Pointer(prev_iter_ends_pseudo_pred))
}

//go:noescape
func _find_newline_delimiters(raw []byte, quoteMask uint64) (mask uint64)

//go:noescape
func __find_newline_delimiters()

//go:noescape
func _find_newline_delimiters_avx512(raw []byte, quoteMask uint64) (mask uint64)

//go:noescape
func __init_newline_delimiters_avx512()

//go:noescape
func __find_newline_delimiters_avx512()

//go:noescape
func __find_quote_mask_and_bits()

//go:noescape
func _find_quote_mask_and_bits(input unsafe.Pointer, odd_ends uint64, prev_iter_inside_quote, quote_bits, error_mask unsafe.Pointer) (quote_mask uint64)

func find_quote_mask_and_bits(buf []byte, odd_ends uint64, prev_iter_inside_quote, quote_bits, error_mask *uint64) (quote_mask uint64) {

	return _find_quote_mask_and_bits(unsafe.Pointer(&buf[0]), odd_ends, unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(quote_bits), unsafe.Pointer(error_mask))
}

//go:noescape
func __init_quote_mask_and_bits_avx512()

//go:noescape
func __find_quote_mask_and_bits_avx512()

//go:noescape
func _find_quote_mask_and_bits_avx512(input unsafe.Pointer, odd_ends uint64, prev_iter_inside_quote unsafe.Pointer) (error_mask, quote_bits, quote_mask uint64)

func find_quote_mask_and_bits_avx512(buf []byte, odd_ends uint64, prev_iter_inside_quote, quote_bits, error_mask *uint64) (quote_mask uint64) {

	*error_mask, *quote_bits, quote_mask = _find_quote_mask_and_bits_avx512(unsafe.Pointer(&buf[0]), odd_ends, unsafe.Pointer(prev_iter_inside_quote))
	return
}

//go:noescape
func __find_odd_backslash_sequences()

//go:noescape
func _find_odd_backslash_sequences(p1, p3 unsafe.Pointer) (result uint64)

func find_odd_backslash_sequences(buf []byte, prev_iter_ends_odd_backslash *uint64) uint64 {
	return _find_odd_backslash_sequences(unsafe.Pointer(&buf[0]), unsafe.Pointer(prev_iter_ends_odd_backslash))
}

//go:noescape
func __init_odd_backslash_sequences_avx512()

//go:noescape
func __find_odd_backslash_sequences_avx512()

//go:noescape
func _find_odd_backslash_sequences_avx512(p1, p3 unsafe.Pointer) (result uint64)

func find_odd_backslash_sequences_avx512(buf []byte, prev_iter_ends_odd_backslash *uint64) uint64 {
	return _find_odd_backslash_sequences_avx512(unsafe.Pointer(&buf[0]), unsafe.Pointer(prev_iter_ends_odd_backslash))
}

//go:noescape
func _find_structural_bits(p1, p3 unsafe.Pointer, /* for: find_odd_backslash_sequences() */
	prev_iter_inside_quote, quote_bits, error_mask unsafe.Pointer, /* for: find_quote_mask_and_bits() */
	whitespace, structurals_in unsafe.Pointer, /* for: find_whitespace_and_structurals() */
	prev_iter_ends_pseudo_pred unsafe.Pointer, /* for: finalize_structurals() */
) (structurals uint64)

func find_structural_bits(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	structurals uint64,
	prev_iter_ends_pseudo_pred *uint64) uint64 {

	quote_bits := uint64(0)
	whitespace := uint64(0)

	return _find_structural_bits(unsafe.Pointer(&buf[0]), unsafe.Pointer(prev_iter_ends_odd_backslash),
		unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(&quote_bits), unsafe.Pointer(error_mask),
		unsafe.Pointer(&whitespace), unsafe.Pointer(&structurals),
		unsafe.Pointer(prev_iter_ends_pseudo_pred))
}

//go:noescape
func _find_structural_bits_avx512(p1, p3 unsafe.Pointer, /* for: find_odd_backslash_sequences() */
	prev_iter_inside_quote, error_mask unsafe.Pointer, /* for: find_quote_mask_and_bits() */
	structurals_in unsafe.Pointer, /* for: find_whitespace_and_structurals() */
	prev_iter_ends_pseudo_pred unsafe.Pointer, /* for: finalize_structurals() */
) (structurals uint64)

func find_structural_bits_avx512(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	structurals uint64,
	prev_iter_ends_pseudo_pred *uint64) uint64 {

	return _find_structural_bits_avx512(unsafe.Pointer(&buf[0]), unsafe.Pointer(prev_iter_ends_odd_backslash),
		unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(error_mask),
		unsafe.Pointer(&structurals),
		unsafe.Pointer(prev_iter_ends_pseudo_pred))
}

//go:noescape
func _find_structural_bits_in_slice(buf unsafe.Pointer, len uint64, p3 unsafe.Pointer, /* for: find_odd_backslash_sequences() */
	prev_iter_inside_quote, quote_bits, error_mask unsafe.Pointer, /* for: find_quote_mask_and_bits() */
	whitespace, structurals_in unsafe.Pointer, /* for: find_whitespace_and_structurals() */
	prev_iter_ends_pseudo_pred unsafe.Pointer, /* for: finalize_structurals()  */
	indexes, index unsafe.Pointer, indexes_len uint64,
	carried unsafe.Pointer, position unsafe.Pointer,
	ndjson uint64) (processed uint64)

func find_structural_bits_in_slice(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	prev_iter_ends_pseudo_pred *uint64,
	indexes *[indexSize]uint32, index *int, carried *uint64, position *uint64,
	ndjson uint64) (processed uint64) {

	if len(buf) == 0 {
		return 0
	}

	structurals := uint64(0)
	quote_bits := uint64(0)
	whitespace := uint64(0)

	return _find_structural_bits_in_slice(unsafe.Pointer(&buf[0]), uint64(len(buf)), unsafe.Pointer(prev_iter_ends_odd_backslash),
		unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(&quote_bits), unsafe.Pointer(error_mask),
		unsafe.Pointer(&whitespace), unsafe.Pointer(&structurals),
		unsafe.Pointer(prev_iter_ends_pseudo_pred),
		unsafe.Pointer(&(*indexes)[0]), unsafe.Pointer(index), indexSizeWithSafetyBuffer,
		unsafe.Pointer(carried), unsafe.Pointer(position),
		ndjson)
}

//go:noescape
func _find_structural_bits_in_slice_avx512(buf unsafe.Pointer, len uint64, p3 unsafe.Pointer, /* for: find_odd_backslash_sequences() */
	prev_iter_inside_quote, error_mask unsafe.Pointer, /* for: find_quote_mask_and_bits() */
	prev_iter_ends_pseudo_pred unsafe.Pointer, /* for: finalize_structurals()  */
	indexes, index unsafe.Pointer, indexes_len uint64,
	carried unsafe.Pointer, position unsafe.Pointer,
	ndjson uint64) (processed uint64)

func find_structural_bits_in_slice_avx512(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	prev_iter_ends_pseudo_pred *uint64,
	indexes *[indexSize]uint32, index *int, carried *uint64, position *uint64,
	ndjson uint64) (processed uint64) {

	if len(buf) == 0 {
		return 0
	}

	return _find_structural_bits_in_slice_avx512(unsafe.Pointer(&buf[0]), uint64(len(buf)), unsafe.Pointer(prev_iter_ends_odd_backslash),
		unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(error_mask),
		unsafe.Pointer(prev_iter_ends_pseudo_pred),
		unsafe.Pointer(&(*indexes)[0]), unsafe.Pointer(index), indexSizeWithSafetyBuffer,
		unsafe.Pointer(carried), unsafe.Pointer(position),
		ndjson)
}

//go:noescape
func __find_whitespace_and_structurals()

//go:noescape
func _find_whitespace_and_structurals(input, whitespace, structurals unsafe.Pointer)

func find_whitespace_and_structurals(buf []byte, whitespace, structurals *uint64) {
	_find_whitespace_and_structurals(unsafe.Pointer(&buf[0]), unsafe.Pointer(whitespace), unsafe.Pointer(structurals))
}

//go:noescape
func __init_whitespace_and_structurals_avx512()

//go:noescape
func __find_whitespace_and_structurals_avx512()

//go:noescape
func _find_whitespace_and_structurals_avx512(input unsafe.Pointer) (whitespace, structurals uint64)

func find_whitespace_and_structurals_avx512(buf []byte, whitespace, structurals *uint64) {
	*whitespace, *structurals = _find_whitespace_and_structurals_avx512(unsafe.Pointer(&buf[0]))
}

//go:noescape
func __flatten_bits_incremental()

//go:noescape
func _flatten_bits_incremental(base_ptr, pbase unsafe.Pointer, mask uint64, carried unsafe.Pointer, position unsafe.Pointer)

func flatten_bits_incremental(base *[indexSize]uint32, base_index *int, mask uint64, carried *int, position *uint64) {
	_flatten_bits_incremental(unsafe.Pointer(&(*base)[0]), unsafe.Pointer(base_index), mask, unsafe.Pointer(carried), unsafe.Pointer(position))
}
