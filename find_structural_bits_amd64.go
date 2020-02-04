//+build !noasm
//+build !appengine

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
func _find_structural_bits_in_slice(buf unsafe.Pointer, len uint64, p3 unsafe.Pointer, /* for: find_odd_backslash_sequences() */
	prev_iter_inside_quote, quote_bits, error_mask unsafe.Pointer, /* for: find_quote_mask_and_bits() */
	whitespace, structurals_in unsafe.Pointer, /* for: find_whitespace_and_structurals() */
	prev_iter_ends_pseudo_pred unsafe.Pointer, /* for: finalize_structurals()  */
	indexes, index unsafe.Pointer, indexes_len uint64,
	carried unsafe.Pointer, position unsafe.Pointer,
	ndjson uint64) (processed uint64)

func find_structural_bits_in_slice(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	structurals uint64,
	prev_iter_ends_pseudo_pred *uint64,
	indexes *[INDEX_SIZE]uint32, index *int, carried *uint64, position *uint64,
	ndjson uint64) (processed uint64) {

	if len(buf) == 0 {
		return 0
	}

	quote_bits := uint64(0)
	whitespace := uint64(0)

	return _find_structural_bits_in_slice(unsafe.Pointer(&buf[0]), uint64(len(buf)), unsafe.Pointer(prev_iter_ends_odd_backslash),
		unsafe.Pointer(prev_iter_inside_quote), unsafe.Pointer(&quote_bits), unsafe.Pointer(error_mask),
		unsafe.Pointer(&whitespace), unsafe.Pointer(&structurals),
		unsafe.Pointer(prev_iter_ends_pseudo_pred),
		unsafe.Pointer(&(*indexes)[0]), unsafe.Pointer(index), INDEX_SIZE_WITH_SAFETY_BUFFER,
		unsafe.Pointer(carried), unsafe.Pointer(position),
		ndjson)
}
