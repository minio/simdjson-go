package simdjson

import (
	"encoding/binary"
)

func UPDATE_CHAR(buf []byte, pj *ParsedJson, i_in uint32) (i uint32, idx uint32, c byte) {
	idx = pj.structural_indexes[i_in]
	i = i_in + 1
	c = buf[idx]
	return
}

func parse_string(buf []byte, pj *ParsedJson, depth, offset uint32) bool {
	pj.write_tape(uint64(len(pj.strings)), '"')
	parse_string_simd(buf[offset:], &pj.strings)
	return true
}

func parse_number(buf []byte, pj *ParsedJson, idx uint32, neg bool) bool {
	succes, is_double, d, i := parse_number_simd(buf[idx:])
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
	tv :=  uint64(0x0000000065757274) // "true    "
	mask4 := uint64(0x00000000ffffffff)
	locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
	error := (locval & mask4) ^ tv
	error |= uint64(is_not_structural_or_whitespace(buf[4]))
	return error == 0
}

func is_valid_false_atom(buf []byte) bool {
	fv :=  uint64(0x00000065736c6166) // "false   "
	mask5 := uint64(0x000000ffffffffff)
	locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
	error := (locval & mask5) ^ fv
	error |= uint64(is_not_structural_or_whitespace(buf[5]))
	return error == 0
}

func is_valid_null_atom(buf []byte) bool {
	nv :=  uint64(0x000000006c6c756e) // "null    "
	mask4 := uint64(0x00000000ffffffff)
	locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
	error := (locval & mask4) ^ nv
	error |= uint64(is_not_structural_or_whitespace(buf[4]))
	return error == 0
}

func unified_machine(buf []byte, pj *ParsedJson) bool {

	// TODO: Figure out why we may have a trailing zero as the last structural element
	if pj.structural_indexes[len(pj.structural_indexes)-1] == 0 {
		pj.structural_indexes = pj.structural_indexes[:len(pj.structural_indexes)-1]
	}

	i := uint32(0)     // index of the structural character (0,1,2,3...)
	idx := uint32(0)   // location of the structural character in the input (buf)
	c := byte(0)       // used to track the (structural) character we are looking at
	depth := uint32(0) // could have an arbitrary starting depth

	//pj.init();

	//if(pj.bytecapacity < len) {
	//return simdjson::CAPACITY;
	//}

	////////////////////////////// START STATE /////////////////////////////

	pj.ret_address[depth] = 's'

	pj.containing_scope_offset[depth] = pj.get_current_loc()

	pj.write_tape(0, 'r') // r for root, 0 is going to get overwritten
	// the root is used, if nothing else, to capture the size of the tape
	depth++ // everything starts at depth = 1, depth = 0 is just for the root, the root may contain an object, an array or something else.

	i, idx, c = UPDATE_CHAR(buf, pj, i)
	switch c {
	case '{':
		pj.containing_scope_offset[depth] = pj.get_current_loc()
		pj.ret_address[depth] = 's'
		depth++
		pj.write_tape(0, c) // strangely, moving this to object_begin slows things down
		goto object_begin
	case '[':
		pj.containing_scope_offset[depth] = pj.get_current_loc()
		pj.ret_address[depth] = 's'
		depth++
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
		//     if (!parse_string(buf, len, pj, depth, idx)) {
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
	// the string might not be NULL terminated.
	if i + 1 == uint32(len(pj.structural_indexes)) {
		goto succeed
	} else {
		goto fail
	}

	//////////////////////////////// OBJECT STATES /////////////////////////////

object_begin:
	i, idx, c = UPDATE_CHAR(buf, pj, i)
	switch (c) {
	case '"':
		if (!parse_string(buf, pj, depth, idx)) {
			goto fail
		}
		goto object_key_state
	case '}':
		goto scope_end // could also go to object_continue
	default:
		goto fail
	}

object_key_state:
	i, idx, c = UPDATE_CHAR(buf, pj, i)
	if c != ':' {
		goto fail
	}
	i, idx, c = UPDATE_CHAR(buf, pj, i)
	switch c {
	case '"':
		if !parse_string(buf, pj, depth, idx) {
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
		if !parse_number(buf, pj, idx, false) {
			goto fail
		}

	case '-':
		if !parse_number(buf, pj, idx, true) {
			goto fail
		}

	case '{':
		pj.containing_scope_offset[depth] = pj.get_current_loc()
		pj.write_tape(0, c) // here the compilers knows what c is so this gets optimized
		// we have not yet encountered } so we need to come back for it
		pj.ret_address[depth] = 'o'
		// we found an object inside an object, so we need to increment the depth
		depth++
		goto object_begin

	case '[':
		pj.containing_scope_offset[depth] = pj.get_current_loc()
		pj.write_tape(0, c) // here the compilers knows what c is so this gets optimized
		// we have not yet encountered } so we need to come back for it
		pj.ret_address[depth] = 'o'
		// we found an array inside an object, so we need to increment the depth
		depth++
		goto array_begin

	default:
		goto fail
	}

object_continue:
	i, idx, c = UPDATE_CHAR(buf, pj, i)
	switch c {
	case ',':
		i, idx, c = UPDATE_CHAR(buf, pj, i)
		if c != '"' {
			goto fail
		}
		if (!parse_string(buf, pj, depth, idx)) {
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
	depth--
	pj.write_tape(pj.containing_scope_offset[depth], c)
	pj.annotate_previousloc(pj.containing_scope_offset[depth], pj.get_current_loc())

	/* goto saved_state*/
	if pj.ret_address[depth] == 'a' {
	    goto array_continue
	} else if pj.ret_address[depth] == 'o' {
	    goto object_continue
	}
    goto start_continue

	////////////////////////////// ARRAY STATES /////////////////////////////
array_begin:
	i, idx, c = UPDATE_CHAR(buf, pj, i)
	if c == ']' {
		goto scope_end // could also go to array_continue
	}

main_array_switch:
	// we call update char on all paths in, so we can peek at c on the
	// on paths that can accept a close square brace (post-, and at start)
	switch c {
	case '"':
		if !parse_string(buf, pj, depth, idx) {
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
		if !parse_number(buf, pj, idx, false) {
			goto fail
		}

	case '-':
		if !parse_number(buf, pj, idx, true) {
			goto fail
		}
		/* goto array_continue */

	case '{':
		// we have not yet encountered ] so we need to come back for it
		pj.containing_scope_offset[depth] = pj.get_current_loc()
		pj.write_tape(0, c) //  here the compilers knows what c is so this gets optimized
		pj.ret_address[depth] = 'a'
		// we found an object inside an array, so we need to increment the depth
		depth++
		goto object_begin

	case '[':
		// we have not yet encountered ] so we need to come back for it
		pj.containing_scope_offset[depth] = pj.get_current_loc()
		pj.write_tape(0, c) // here the compilers knows what c is so this gets optimized
		pj.ret_address[depth] = 'a'
		// we found an array inside an array, so we need to increment the depth
		depth++
		goto array_begin

	default:
		goto fail
	}

array_continue:
	i, idx, c = UPDATE_CHAR(buf, pj, i)
	switch c {
	case ',':
		i, idx, c = UPDATE_CHAR(buf, pj, i)
		goto main_array_switch

	case ']':
		goto scope_end

	default:
		goto fail
	}

	////////////////////////////// FINAL STATES /////////////////////////////
succeed:
	depth--
	if depth != 0 {
		panic("internal bug\n")
	}

	if pj.containing_scope_offset[depth] != 0 {
		panic("internal bug\n")
	}

	pj.annotate_previousloc(pj.containing_scope_offset[depth], pj.get_current_loc())
	pj.write_tape(pj.containing_scope_offset[depth], 'r') // r is root

	pj.isvalid  = true
	return true // simdjson::SUCCESS

fail:
	return false // simdjson::TAPE_ERROR
}

