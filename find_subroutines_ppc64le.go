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

func find_structural_bits_in_slice(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	structurals uint64,
	prev_iter_ends_pseudo_pred *uint64,
	indexes *[INDEX_SIZE]uint32, index *int, carried *uint64, position *uint64,
	ndjson uint64) (processed uint64) {
	return
}

func parse_string_simd_validate_only(buf []byte, maxStringSize, dst_length *uint64, need_copy *bool) bool {
	return false
}

func parse_string_simd(buf []byte, stringbuf *[]byte) bool {
	return false
}

func parse_number_simd(buf []byte, found_minus bool) (success, is_double bool, d float64, i int) {
	return
}

func find_odd_backslash_sequences(buf []byte, prev_iter_ends_odd_backslash *uint64) uint64 {
	return 0
}

func find_quote_mask_and_bits(buf []byte, odd_ends uint64, prev_iter_inside_quote, quote_bits, error_mask *uint64) (quote_mask uint64) {
	return
}

func find_whitespace_and_structurals(buf []byte, whitespace, structurals *uint64) {
}

func finalize_structurals(structurals, whitespace, quote_mask, quote_bits uint64, prev_iter_ends_pseudo_pred *uint64) uint64 {
	return 0
}
