package simdjson

type ParsedJson struct {
	structural_indexes []uint32
}

const paddingSpaces64 = "                                                                "

func find_structural_bits(buf []byte, pj *ParsedJson) bool {

	//if (len > pj.bytecapacity) {
	//	cerr << "Your ParsedJson object only supports documents up to "
	//	<< pj.bytecapacity << " bytes but you are trying to process " << len
	//	<< " bytes\n";
	//	return false;
	//}

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

	lenminus64 := uint64(0)
	if len(buf) >= 64 {
		lenminus64 = uint64(len(buf)) - 64
	}

	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)

	idx := uint64(0)
	for ; idx < lenminus64; idx += 64 {

		// __builtin_prefetch(buf + idx + 128);

		// #ifdef SIMDJSON_UTF8VALIDATE
		// check_utf8(input_lo, input_hi, has_error, previous);
		// #endif

		// detect odd sequences of backslashes
		odd_ends := find_odd_backslash_sequences(buf[idx:], &prev_iter_ends_odd_backslash)

		// detect insides of quote pairs ("quote_mask") and also our quote_bits themselves
		quote_bits := uint64(0)
		quote_mask := find_quote_mask_and_bits(buf[idx:], odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)

		// take the previous iterations structural bits, not our current iteration, and flatten
		flatten_bits(&pj.structural_indexes, uint64(idx), structurals)

		whitespace_mask := uint64(0)
		find_whitespace_and_structurals(buf[idx:], &whitespace_mask, &structurals)

		// fixup structurals to reflect quotes and add pseudo-structural characters
		structurals = finalize_structurals(structurals, whitespace_mask, quote_mask, quote_bits, &prev_iter_ends_pseudo_pred)
	}

	////////////////
	/// we use a giant copy-paste which is ugly.
	/// but otherwise the string needs to be properly padded or else we
	/// risk invalidating the UTF-8 checks.
	////////////
	if idx < uint64(len(buf)) {
		tmpbuf := [64]byte{}

		remain := uint64(len(buf))-idx
		copy(tmpbuf[:], buf[idx:])
		copy(tmpbuf[remain:], []byte(paddingSpaces64)[:64-remain])

		// #ifdef SIMDJSON_UTF8VALIDATE
		// check_utf8(input_lo, input_hi, has_error, previous);
		// #endif

		// detect odd sequences of backslashes
		odd_ends := find_odd_backslash_sequences(tmpbuf[:], &prev_iter_ends_odd_backslash)

		// detect insides of quote pairs ("quote_mask") and also our quote_bits themselves
		quote_bits := uint64(0)
		quote_mask := find_quote_mask_and_bits(tmpbuf[:], odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)

		// take the previous iterations structural bits, not our current iteration, and flatten
		flatten_bits(&pj.structural_indexes, uint64(idx), structurals)

		whitespace_mask := uint64(0)
		find_whitespace_and_structurals(tmpbuf[:], &whitespace_mask, &structurals)

		// fixup structurals to reflect quotes and add pseudo-structural characters
		structurals = finalize_structurals(structurals, whitespace_mask, quote_mask, quote_bits, &prev_iter_ends_pseudo_pred)

		idx += 64
	}

	// finally, flatten out the remaining structurals from the last iteration
	flatten_bits(&pj.structural_indexes, uint64(idx), structurals)

	// a valid JSON file cannot have zero structural indexes - we should have found something
	if len(pj.structural_indexes) == 0 {
		return false
	}

	if uint32(len(buf)) != pj.structural_indexes[len(pj.structural_indexes) - 1] {
		// the string might not be NULL terminated, but we add a virtual NULL ending character.
		pj.structural_indexes = append(pj.structural_indexes, uint32(len(buf)))
	}

	// make it safe to dereference one beyond this array
	pj.structural_indexes = append(pj.structural_indexes, 0)

	if error_mask != 0 {
		return false
	}

	// #ifdef SIMDJSON_UTF8VALIDATE
	// return _mm256_testz_si256(has_error, has_error) != 0;
	// #endif

	return true
}

