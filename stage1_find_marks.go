package simdjson

import (
	"sync/atomic"
)

func matching_structurals(opening, ending byte) bool {
	return opening == '{' && ending == '}'
}

func find_structural_indices(buf []byte, pj *internalParsedJson) bool {

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

	// structurals are persistent state across loop as we flatten them on the
	// subsequent iteration into our array.
	// This is harmless on the first iteration as structurals == 0
	// and is done for performance reasons; we can hide some of the latency of the
	// expensive carryless multiply in the previous step with this work
	structurals := uint64(0)

	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)

	indexTotal := 0

	// empty bits that are carried over to the next call to flatten_bits_incremental
	carried := uint64(0)

	// keep the opening structural char so that we can verify it with the closing char
	opening_struct_char := ^uint64(0)

	for len(buf) > 0 {

		index := indexChan{}
		offset := atomic.AddUint64(&pj.buffers_offset, 1)
		index.indexes = &pj.buffers[offset%INDEX_SLOTS]

		var processed uint64
		if len(buf) > 64 {
			processed = find_structural_bits_in_slice(buf[:len(buf) & ^63], &prev_iter_ends_odd_backslash,
				&prev_iter_inside_quote, &error_mask,
				structurals,
				&prev_iter_ends_pseudo_pred,
				index.indexes, &index.length, &carried, pj.ndjson)
		} else {
			// Process last 64 bytes in larger buffer (to safeguard against reading beyond the end of the buffer)
			paddedBuf := [128]byte{}
			copy(paddedBuf[:], buf)
			processed = find_structural_bits_in_slice(paddedBuf[:len(buf)], &prev_iter_ends_odd_backslash,
				&prev_iter_inside_quote, &error_mask,
				structurals,
				&prev_iter_ends_pseudo_pred,
				index.indexes, &index.length, &carried, pj.ndjson)
		}

		if opening_struct_char == ^uint64(0) && index.length > 0 {
			opening_struct_char = uint64(buf[opening_struct_char+uint64(index.indexes[0])])
		}

		if uint64(len(buf)) == processed { // message processing completed?
			offset := ^uint64(0)
			for i := 0; i < index.length; i++ {
				offset += uint64(index.indexes[i])
			}
			// break out if either
			// - no structural chars have been found
			// - is there an unmatched quote at the end
			// - the ending structural char does not match the opening char
			if index.length == 0 ||
				prev_iter_inside_quote != 0 ||
				(offset != ^uint64(0) && offset < uint64(len(buf)) && !matching_structurals(byte(opening_struct_char), buf[offset])) {
				error_mask = ^uint64(0)
				break
			}
		}

		pj.index_chan <- index
		indexTotal += index.length

		buf = buf[processed:]
	}
	close(pj.index_chan)

	// a valid JSON file cannot have zero structural indexes - we should have found something
	return error_mask == 0 && indexTotal > 0
}
