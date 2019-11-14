package simdjson

import (
	"encoding/binary"
)

// Constants for "return address" modes
const RET_ADDRESS_SHIFT = 2
const RET_ADDRESS_START_CONST = 1
const RET_ADDRESS_OBJECT_CONST = 2
const RET_ADDRESS_ARRAY_CONST = 3

func updateChar(buf []byte, pj *internalParsedJson, idx_in uint64, indexesChan *indexChan) (done bool, idx uint64, c byte) {
	if (*indexesChan).index >= (*indexesChan).length {
		var ok bool
		*indexesChan, ok = <- pj.index_chan // Get next element from channel
		if !ok {
			done = true	// return done if channel closed
			return
		}
	}
	idx = idx_in + uint64((*indexesChan).indexes[(*indexesChan).index])
	(*indexesChan).index++
	c = buf[idx]
	return
}

func parse_string(buf []byte, pj *ParsedJson, depth int, offset uint64) bool {
	pj.write_tape(uint64(len(pj.Strings)), '"')
	parse_string_simd(buf[offset:], &pj.Strings)
	return true
}

func parse_number(buf []byte, pj *ParsedJson, idx uint64, neg bool) bool {
	succes, is_double, d, i := parse_number_simd(buf[idx:], neg)
	if !succes {
		return false
	}
	if is_double {
		pj.write_tape_double(d)
	} else {
		pj.write_tape_s64(i)
	}
	return true
}

func is_valid_true_atom(buf []byte) bool {
	tv := uint64(0x0000000065757274) // "true    "
	mask4 := uint64(0x00000000ffffffff)
	locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
	error := (locval & mask4) ^ tv
	error |= uint64(is_not_structural_or_whitespace(buf[4]))
	return error == 0
}

func is_valid_false_atom(buf []byte) bool {
	fv := uint64(0x00000065736c6166) // "false   "
	mask5 := uint64(0x000000ffffffffff)
	locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
	error := (locval & mask5) ^ fv
	error |= uint64(is_not_structural_or_whitespace(buf[5]))
	return error == 0
}

func is_valid_null_atom(buf []byte) bool {
	nv := uint64(0x000000006c6c756e) // "null    "
	mask4 := uint64(0x00000000ffffffff)
	locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
	error := (locval & mask4) ^ nv
	error |= uint64(is_not_structural_or_whitespace(buf[4]))
	return error == 0
}

func unified_machine(buf []byte, pj *internalParsedJson) bool {

	done := false
	idx := uint64(0)    // location of the structural character in the input (buf)
	c := byte(0)        // used to track the (structural) character we are looking at
	offset := uint64(0) // used to contain last element of containing_scope_offset
	var indexCh indexChan

	////////////////////////////// START STATE /////////////////////////////
	pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)

	pj.write_tape(0, 'r') // r for root, 0 is going to get overwritten
	// the root is used, if nothing else, to capture the size of the tape

	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	switch c {
	case '{':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)
		pj.write_tape(0, c) // strangely, moving this to object_begin slows things down
		goto object_begin
	case '[':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)
		pj.write_tape(0, c)
		goto array_begin

		// A JSON text is a serialized value.  Note that certain previous
		// specifications of JSON constrained a JSON text to be an object or an
		// array. Implementations that generate only objects or arrays where a
		// JSON text is called for will be interoperable in the sense that all
		// implementations will accept these as conforming JSON texts.
		// https://tools.ietf.org/html/rfc8259

		// #ifdef SIMDJSON_ALLOWANYTHINGINROOT
		// case '"': {
		//     if (!parse_string(buf, len, pj, len(pj.containing_scope_offset), idx)) {
		//         goto fail;
		//     }
		// break;
		// }
		// case 't': {
		// // we need to make a copy to make sure that the string is NULL terminated.
		// // this only applies to the JSON document made solely of the true value.
		// // this will almost never be called in practice
		// char * copy = static_cast<char *>(malloc(len + SIMDJSON_PADDING));
		//     if(copy == nullptr) { goto fail;
		//     }
		// memcpy(copy, buf, len);
		// copy[len] = '\0';
		//     if (!is_valid_true_atom(reinterpret_cast<const uint8_t *>(copy) + idx)) {
		//         free(copy);
		//         goto fail;
		//     }
		// free(copy);
		// pj.write_tape(0, c);
		// break;
		// }
		// case 'f': {
		// // we need to make a copy to make sure that the string is NULL terminated.
		// // this only applies to the JSON document made solely of the false value.
		// // this will almost never be called in practice
		// char * copy = static_cast<char *>(malloc(len + SIMDJSON_PADDING));
		// if(copy == nullptr) { goto fail;
		// }
		// memcpy(copy, buf, len);
		// copy[len] = '\0';
		// if (!is_valid_false_atom(reinterpret_cast<const uint8_t *>(copy) + idx)) {
		//     free(copy);
		//     goto fail;
		// }
		// free(copy);
		// pj.write_tape(0, c);
		// break;
		// }
		// case 'n': {
		// // we need to make a copy to make sure that the string is NULL terminated.
		// // this only applies to the JSON document made solely of the null value.
		// // this will almost never be called in practice
		// char * copy = static_cast<char *>(malloc(len + SIMDJSON_PADDING));
		// if(copy == nullptr) { goto fail;
		// }
		// memcpy(copy, buf, len);
		// copy[len] = '\0';
		// if (!is_valid_null_atom(reinterpret_cast<const uint8_t *>(copy) + idx)) {
		//     free(copy);
		//     goto fail;
		// }
		// free(copy);
		// pj.write_tape(0, c);
		// break;
		// }
		// case '0':
		// case '1':
		// case '2':
		// case '3':
		// case '4':
		// case '5':
		// case '6':
		// case '7':
		// case '8':
		// case '9': {
		// // we need to make a copy to make sure that the string is NULL terminated.
		// // this is done only for JSON documents made of a sole number
		// // this will almost never be called in practice
		// char * copy = static_cast<char *>(malloc(len + SIMDJSON_PADDING));
		// if(copy == nullptr) { goto fail;
		// }
		// memcpy(copy, buf, len);
		// copy[len] = '\0';
		// if (!parse_number(reinterpret_cast<const uint8_t *>(copy), pj, idx, false)) {
		// free(copy);
		// goto fail;
		// }
		// free(copy);
		// break;
		// }
		// case '-': {
		// // we need to make a copy to make sure that the string is NULL terminated.
		// // this is done only for JSON documents made of a sole number
		// // this will almost never be called in practice
		// char * copy = static_cast<char *>(malloc(len + SIMDJSON_PADDING));
		// if(copy == nullptr) { goto fail;
		// }
		// memcpy(copy, buf, len);
		// copy[len] = '\0';
		// if (!parse_number(reinterpret_cast<const uint8_t *>(copy), pj, idx, true)) {
		// free(copy);
		// goto fail;
		// }
		// free(copy);
		// break;
		// }
		// #endif // ALLOWANYTHINGINROOT
	default:
		goto fail
	}

start_continue:
	// We are back at the top, read the next char and we should be done
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	} else {
		goto fail
	}

	//////////////////////////////// OBJECT STATES /////////////////////////////

object_begin:
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	switch c {
	case '"':
		if !parse_string(buf, &pj.ParsedJson, len(pj.containing_scope_offset), idx) {
			goto fail
		}
		goto object_key_state
	case '}':
		goto scope_end // could also go to object_continue
	default:
		goto fail
	}

object_key_state:
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	if c != ':' {
		goto fail
	}
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	switch c {
	case '"':
		if !parse_string(buf, &pj.ParsedJson, len(pj.containing_scope_offset), idx) {
			goto fail
		}

	case 't':
		if !is_valid_true_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, c)

	case 'f':
		if !is_valid_false_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, c)

	case 'n':
		if !is_valid_null_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, c)

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if !parse_number(buf, &pj.ParsedJson, idx, false) {
			goto fail
		}

	case '-':
		if !parse_number(buf, &pj.ParsedJson, idx, true) {
			goto fail
		}

	case '{':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_OBJECT_CONST)
		pj.write_tape(0, c) // here the compilers knows what c is so this gets optimized
		// we have not yet encountered } so we need to come back for it
		goto object_begin

	case '[':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_OBJECT_CONST)
		pj.write_tape(0, c) // here the compilers knows what c is so this gets optimized
		// we have not yet encountered } so we need to come back for it
		goto array_begin

	default:
		goto fail
	}

object_continue:
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	switch c {
	case ',':
		if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
			goto succeed
		}
		if c != '"' {
			goto fail
		}
		if !parse_string(buf, &pj.ParsedJson, len(pj.containing_scope_offset), idx) {
			goto fail
		}
		goto object_key_state

	case '}':
		goto scope_end

	default:
		goto fail
	}

	////////////////////////////// COMMON STATE /////////////////////////////
scope_end:
	// write our tape location to the header scope
	offset = pj.containing_scope_offset[len(pj.containing_scope_offset)-1]
	// drop last element
	pj.containing_scope_offset = pj.containing_scope_offset[:len(pj.containing_scope_offset)-1]

	pj.write_tape(offset>>RET_ADDRESS_SHIFT, c)
	pj.annotate_previousloc(offset>>RET_ADDRESS_SHIFT, pj.get_current_loc())

	/* goto saved_state*/
	switch offset & ((1 << RET_ADDRESS_SHIFT) - 1) {
	case RET_ADDRESS_ARRAY_CONST:
		goto array_continue
	case RET_ADDRESS_OBJECT_CONST:
		goto object_continue
	default:
		goto start_continue
	}

	////////////////////////////// ARRAY STATES /////////////////////////////
array_begin:
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	if c == ']' {
		goto scope_end // could also go to array_continue
	}

main_array_switch:
	// we call update char on all paths in, so we can peek at c on the
	// on paths that can accept a close square brace (post-, and at start)
	switch c {
	case '"':
		if !parse_string(buf, &pj.ParsedJson, len(pj.containing_scope_offset), idx) {
			goto fail
		}
	case 't':
		if !is_valid_true_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, c)

	case 'f':
		if !is_valid_false_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, c)

	case 'n':
		if !is_valid_null_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, c)
		/* goto array_continue */

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if !parse_number(buf, &pj.ParsedJson, idx, false) {
			goto fail
		}

	case '-':
		if !parse_number(buf, &pj.ParsedJson, idx, true) {
			goto fail
		}
		/* goto array_continue */

	case '{':
		// we have not yet encountered ] so we need to come back for it
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_ARRAY_CONST)
		pj.write_tape(0, c) //  here the compilers knows what c is so this gets optimized
		goto object_begin

	case '[':
		// we have not yet encountered ] so we need to come back for it
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_ARRAY_CONST)
		pj.write_tape(0, c) // here the compilers knows what c is so this gets optimized
		goto array_begin

	default:
		goto fail
	}

array_continue:
	if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
		goto succeed
	}
	switch c {
	case ',':
		if done, idx, c = updateChar(buf, pj, idx, &indexCh); done {
			goto succeed
		}
		goto main_array_switch

	case ']':
		goto scope_end

	default:
		goto fail
	}

	////////////////////////////// FINAL STATES /////////////////////////////
succeed:
	offset = pj.containing_scope_offset[len(pj.containing_scope_offset)-1]
	// drop last element
	pj.containing_scope_offset = pj.containing_scope_offset[:len(pj.containing_scope_offset)-1]

	// Sanity checks
	if len(pj.containing_scope_offset) != 0 {
		return false
	}

	const addOneForRoot = 1
	pj.annotate_previousloc(offset>>RET_ADDRESS_SHIFT, pj.get_current_loc() + addOneForRoot)
	pj.write_tape(offset>>RET_ADDRESS_SHIFT, 'r') // r is root

	pj.isvalid = true
	return true // simdjson::SUCCESS

fail:
	return false // simdjson::TAPE_ERROR
}
