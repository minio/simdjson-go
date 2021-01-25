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
	"sync/atomic"

	"github.com/klauspost/cpuid/v2"
)

var jsonMarkupTable = [256]bool{
	'{': true,
	'}': true,
	'[': true,
	']': true,
	',': true,
	':': true,
}

func jsonMarkup(b byte) bool {
	return jsonMarkupTable[b]
}

func findStructuralIndices(buf []byte, pj *internalParsedJson) bool {

	f := find_structural_bits_in_slice
	if cpuid.CPU.Has(cpuid.AVX512F) {
		f = find_structural_bits_in_slice_avx512
	}

	// persistent state across loop
	// does the last iteration end with an odd-length sequence of backslashes?
	// either 0 or 1, but a 64-bit value
	prev_iter_ends_odd_backslash := uint64(0)

	// does the previous iteration end inside a double-quote pair?
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones

	// does the previous iteration end on something that is a predecessor of a
	// pseudo-structural character - i.e. whitespace or a structural character
	// effectively the very first char is considered to follow "whitespace" for the
	// purposes of pseudo-structural character detection so we initialize to 1
	prev_iter_ends_pseudo_pred := uint64(1)

	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)

	indexTotal := 0

	// empty bits that are carried over to the next call to flatten_bits_incremental
	carried := uint64(0)

	// absolute position into message buffer
	position := ^uint64(0)
	stripped_index := ^uint64(0)

	for len(buf) > 0 {

		index := indexChan{}
		offset := atomic.AddUint64(&pj.buffersOffset, 1)
		index.indexes = &pj.buffers[offset%indexSlots]

		// In case last index during previous round was stripped back, put it back
		if stripped_index != ^uint64(0) {
			position += stripped_index
			index.indexes[0] = uint32(stripped_index)
			index.length = 1
			stripped_index = ^uint64(0)
		}

		processed := f(buf[:len(buf) & ^63], &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			&prev_iter_ends_pseudo_pred,
			index.indexes, &index.length, &carried, &position, pj.ndjson)

		// Check if we have at most a single iteration of 64 bytes left, tag on to previous invocation
		if uint64(len(buf))-processed <= 64 {
			// Process last 64 bytes in larger buffer (to safeguard against reading beyond the end of the buffer)
			paddedBuf := [128]byte{}
			copy(paddedBuf[:], buf[processed:])
			paddedBytes := uint64(len(buf)) - processed
			processed += f(paddedBuf[:paddedBytes], &prev_iter_ends_odd_backslash,
				&prev_iter_inside_quote, &error_mask,
				&prev_iter_ends_pseudo_pred,
				index.indexes, &index.length, &carried, &position, pj.ndjson)
		}

		if index.length == 0 { // No structural chars found, so error out
			error_mask = ^uint64(0)
			break
		}

		if uint64(len(buf)) == processed { // message processing completed?
			// break out if either
			// - is there an unmatched quote at the end
			// - the ending structural char is not either a '}' (normal json) or a ']' (array style)
			if prev_iter_inside_quote != 0 ||
				position >= uint64(len(buf)) ||
				!(buf[position] == '}' || buf[position] == ']') {
				error_mask = ^uint64(0)
				break
			}
		} else if !jsonMarkup(buf[position]) {
			// There may be a dangling quote at the end of the index buffer
			// Strip it from current index buffer and save for next round
			stripped_index = uint64(index.indexes[index.length-1])
			position -= stripped_index
			index.length -= 1
		}

		pj.indexChans <- index
		indexTotal += index.length

		buf = buf[processed:]
		position -= processed
	}
	close(pj.indexChans)

	// a valid JSON file cannot have zero structural indexes - we should have found something
	return error_mask == 0 && indexTotal > 0
}
