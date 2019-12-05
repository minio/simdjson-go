package simdjson

import "sync/atomic"

func find_structural_indices(buf []byte, pj *internalParsedJson) bool {

	//  #ifdef SIMDJSON_UTF8VALIDATE
	//      __m256i has_error = _mm256_setzero_si256();
	//      struct avx_processed_utf_bytes previous {};
	//	    previous.rawbytes = _mm256_setzero_si256();
	//	    previous.high_nibbles = _mm256_setzero_si256();
	//	    previous.carried_continuations = _mm256_setzero_si256();
	//	#endif

	// we have padded the input out to 64 byte multiple with the remainder being zeros

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

	for len(buf) > 0 {

		// #ifdef SIMDJSON_UTF8VALIDATE
		// check_utf8(input_lo, input_hi, has_error, previous);
		// #endif

		index := indexChan{}
		offset := atomic.AddUint64(&pj.buffers_offset, 1)
		index.indexes = &pj.buffers[offset%INDEX_SLOTS]

		processed := find_structural_bits_loop(buf, &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			structurals,
			&prev_iter_ends_pseudo_pred,
			index.indexes, &index.length, &carried)

		buf = buf[processed:]

		// Is there an unmatched quote at the end? If so do not forward the
		// indices onto the channel as this may cause a read beyond the slice
		// boundary in stage 2
		unmatched_quote_at_end := prev_iter_inside_quote != 0 && len(buf) == 0
		if !unmatched_quote_at_end {
			pj.index_chan <- index
			indexTotal += index.length
		}
	}
	close(pj.index_chan)

	// Did we end with an unmatched quote? If so fail the stage
	if prev_iter_inside_quote != 0 {
		return false
	}

	// a valid JSON file cannot have zero structural indexes - we should have found something
	if indexTotal == 0 {
		return false
	}

	if error_mask != 0 {
		return false
	}

	// #ifdef SIMDJSON_UTF8VALIDATE
	// return _mm256_testz_si256(has_error, has_error) != 0;
	// #endif

	return true
}
