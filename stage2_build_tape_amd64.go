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
	"bytes"
	"encoding/binary"
	"fmt"
)

// Constants for "return address" modes
const RET_ADDRESS_SHIFT = 2
const RET_ADDRESS_START_CONST = 1
const RET_ADDRESS_OBJECT_CONST = 2
const RET_ADDRESS_ARRAY_CONST = 3

func updateChar(pj *internalParsedJson, idx_in uint64) (done bool, idx uint64) {
	if pj.indexesChan.index >= pj.indexesChan.length {
		var ok bool
		pj.indexesChan, ok = <-pj.index_chan // Get next element from channel
		if !ok {
			done = true // return done if channel closed
			return
		}
	}
	idx = idx_in + uint64(pj.indexesChan.indexes[pj.indexesChan.index])
	pj.indexesChan.index++
	return
}

// Handy "debug" function to see where Stage 2 fails (rename to `updateChar`)
func updateCharDebug(pj *internalParsedJson, idx_in uint64) (done bool, idx uint64) {
	if pj.indexesChan.index >= pj.indexesChan.length {
		var ok bool
		pj.indexesChan, ok = <-pj.index_chan // Get next element from channel
		if !ok {
			done = true // return done if channel closed
			return
		}
	}
	idx = idx_in + uint64(pj.indexesChan.indexes[pj.indexesChan.index])
	fmt.Printf("At 0x%x char: %s\n", idx, string(pj.Message[idx]))
	pj.indexesChan.index++
	return
}

func peekSize(pj *internalParsedJson) uint64 {
	if pj.indexesChan.index >= pj.indexesChan.length {
		//panic("cannot peek the size") // should never happen since last string element should be saved for next buffer
		// let's return 0 for the sake of safety (could lead to a string being to short)
		return 0
	}
	return uint64(pj.indexesChan.indexes[pj.indexesChan.index])
}

func parse_string(pj *ParsedJson, idx uint64, maxStringSize uint64) bool {
	size := uint64(0)
	need_copy := false
	buf := pj.Message[idx:]
	// Make sure that we have at least one full YMM word available after maxStringSize into the buffer
	if len(buf)-int(maxStringSize) < 64 {
		if len(buf) > 512-64 { // only allocated if needed
			paddedBuf := make([]byte, len(buf)+64)
			copy(paddedBuf, buf)
			buf = paddedBuf
		} else {
			paddedBuf := [512]byte{}
			copy(paddedBuf[:], buf)
			buf = paddedBuf[:]
		}
	}
	if !parse_string_simd_validate_only(buf, &maxStringSize, &size, &need_copy) {
		return false
	}
	if !need_copy {
		pj.write_tape(idx+1, '"')
	} else {
		// Make sure we account for at least 32 bytes additional space due to
		requiredLen := uint64(len(pj.Strings)) + size + 32
		if requiredLen >= uint64(cap(pj.Strings)) {
			newSize := uint64(cap(pj.Strings) * 2)
			if newSize < requiredLen {
				newSize = requiredLen + size // add size once more to account for further space
			}
			strs := make([]byte, len(pj.Strings), newSize)
			copy(strs, pj.Strings)
			pj.Strings = strs
		}
		start := len(pj.Strings)
		_ = parse_string_simd(buf, &pj.Strings) // We can safely ignore the result since we validate above
		pj.write_tape(uint64(STRINGBUFBIT+start), '"')
		size = uint64(len(pj.Strings) - start)
	}
	// put length onto the tape
	pj.Tape = append(pj.Tape, size)
	return true
}

func parse_number(buf []byte, pj *ParsedJson, neg bool) bool {
	if len(buf) < 64 { // if we have less than 2 YMM words left, make sure there is enough space
		paddedBuf := [128]byte{}
		copy(paddedBuf[:], buf)
		buf = paddedBuf[:]
	}
	succes, is_double, d, i := parse_number_simd(buf, neg)
	if !succes {
		return false
	}
	if is_double {
		pj.write_tape_double(d)
	} else {
		pj.write_tape_s64(int64(i))
	}
	return true
}

func is_valid_true_atom(buf []byte) bool {
	if len(buf) >= 8 { // fast path when there is enough space left in the buffer
		tv := uint64(0x0000000065757274) // "true    "
		mask4 := uint64(0x00000000ffffffff)
		locval := binary.LittleEndian.Uint64(buf)
		error := (locval & mask4) ^ tv
		error |= uint64(is_not_structural_or_whitespace(buf[4]))
		return error == 0
	} else if len(buf) >= 5 {
		return bytes.Compare(buf[:4], []byte("true")) == 0 && is_not_structural_or_whitespace(buf[4]) == 0
	}
	return false
}

func is_valid_false_atom(buf []byte) bool {
	if len(buf) >= 8 { // fast path when there is enough space left in the buffer
		fv := uint64(0x00000065736c6166) // "false   "
		mask5 := uint64(0x000000ffffffffff)
		locval := binary.LittleEndian.Uint64(buf)
		error := (locval & mask5) ^ fv
		error |= uint64(is_not_structural_or_whitespace(buf[5]))
		return error == 0
	} else if len(buf) >= 6 {
		return bytes.Compare(buf[:5], []byte("false")) == 0 && is_not_structural_or_whitespace(buf[5]) == 0
	}
	return false
}

func is_valid_null_atom(buf []byte) bool {
	if len(buf) >= 8 { // fast path when there is enough space left in the buffer
		nv := uint64(0x000000006c6c756e) // "null    "
		mask4 := uint64(0x00000000ffffffff)
		locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
		error := (locval & mask4) ^ nv
		error |= uint64(is_not_structural_or_whitespace(buf[4]))
		return error == 0
	} else if len(buf) >= 5 {
		return bytes.Compare(buf[:4], []byte("null")) == 0 && is_not_structural_or_whitespace(buf[4]) == 0
	}
	return false
}

func unified_machine(buf []byte, pj *internalParsedJson) bool {

	const addOneForRoot = 1

	done := false
	idx := ^uint64(0)   // location of the structural character in the input (buf)
	offset := uint64(0) // used to contain last element of containing_scope_offset

	////////////////////////////// START STATE /////////////////////////////
	pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)

	pj.write_tape(0, 'r') // r for root, 0 is going to get overwritten
	// the root is used, if nothing else, to capture the size of the tape

	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
continue_root:
	switch buf[idx] {
	case '{':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)
		pj.write_tape(0, buf[idx])
		goto object_begin
	case '[':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)
		pj.write_tape(0, buf[idx])
		goto array_begin
	default:
		goto fail
	}

start_continue:
	// We are back at the top, read the next char and we should be done
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	} else {
		// For an ndjson object, wrap up current object, start new root and check for minimum of 1 newline
		if buf[idx] != '\n' {
			goto fail
		}

		// Eat any empty lines
		for buf[idx] == '\n' {
			if done, idx = updateChar(pj, idx); done {
				goto succeed
			}
		}

		// Otherwise close current root
		offset = pj.containing_scope_offset[len(pj.containing_scope_offset)-1]

		// drop last element
		pj.containing_scope_offset = pj.containing_scope_offset[:len(pj.containing_scope_offset)-1]

		pj.annotate_previousloc(offset>>RET_ADDRESS_SHIFT, pj.get_current_loc()+addOneForRoot)
		pj.write_tape(offset>>RET_ADDRESS_SHIFT, 'r') // r is root

		// And open a new root
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_START_CONST)
		pj.write_tape(0, 'r') // r for root, 0 is going to get overwritten

		goto continue_root
	}

	//////////////////////////////// OBJECT STATES /////////////////////////////

object_begin:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	switch buf[idx] {
	case '"':
		if !parse_string(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}
		goto object_key_state
	case '}':
		goto scope_end // could also go to object_continue
	default:
		goto fail
	}

object_key_state:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	if buf[idx] != ':' {
		goto fail
	}
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	switch buf[idx] {
	case '"':
		if !parse_string(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}

	case 't':
		if !is_valid_true_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'f':
		if !is_valid_false_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'n':
		if !is_valid_null_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if !parse_number(buf[idx:], &pj.ParsedJson, false) {
			goto fail
		}

	case '-':
		if !parse_number(buf[idx:], &pj.ParsedJson, true) {
			goto fail
		}

	case '{':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_OBJECT_CONST)
		pj.write_tape(0, buf[idx])
		// we have not yet encountered } so we need to come back for it
		goto object_begin

	case '[':
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_OBJECT_CONST)
		pj.write_tape(0, buf[idx])
		// we have not yet encountered } so we need to come back for it
		goto array_begin

	default:
		goto fail
	}

object_continue:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	switch buf[idx] {
	case ',':
		if done, idx = updateChar(pj, idx); done {
			goto succeed
		}
		if buf[idx] != '"' {
			goto fail
		}
		if !parse_string(&pj.ParsedJson, idx, peekSize(pj)) {
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

	pj.write_tape(offset>>RET_ADDRESS_SHIFT, buf[idx])
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
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	if buf[idx] == ']' {
		goto scope_end // could also go to array_continue
	}

main_array_switch:
	// we call update char on all paths in, so we can peek at c on the
	// on paths that can accept a close square brace (post-, and at start)
	switch buf[idx] {
	case '"':
		if !parse_string(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}
	case 't':
		if !is_valid_true_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'f':
		if !is_valid_false_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'n':
		if !is_valid_null_atom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])
		/* goto array_continue */

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if !parse_number(buf[idx:], &pj.ParsedJson, false) {
			goto fail
		}

	case '-':
		if !parse_number(buf[idx:], &pj.ParsedJson, true) {
			goto fail
		}
		/* goto array_continue */

	case '{':
		// we have not yet encountered ] so we need to come back for it
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_ARRAY_CONST)
		pj.write_tape(0, buf[idx]) //  here the compilers knows what c is so this gets optimized
		goto object_begin

	case '[':
		// we have not yet encountered ] so we need to come back for it
		pj.containing_scope_offset = append(pj.containing_scope_offset, (pj.get_current_loc()<<RET_ADDRESS_SHIFT)|RET_ADDRESS_ARRAY_CONST)
		pj.write_tape(0, buf[idx]) // here the compilers knows what c is so this gets optimized
		goto array_begin

	default:
		goto fail
	}

array_continue:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	switch buf[idx] {
	case ',':
		if done, idx = updateChar(pj, idx); done {
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

	pj.annotate_previousloc(offset>>RET_ADDRESS_SHIFT, pj.get_current_loc()+addOneForRoot)
	pj.write_tape(offset>>RET_ADDRESS_SHIFT, 'r') // r is root

	pj.isvalid = true
	return true

fail:
	return false
}

// structural chars here are
// they are { 0x7b } 0x7d : 0x3a [ 0x5b ] 0x5d , 0x2c (and NULL)
// we are also interested in the four whitespace characters
// space 0x20, linefeed 0x0a, horizontal tab 0x09 and carriage return 0x0d

// these are the chars that can follow a true/false/null or number atom
// and nothing else
var structural_or_whitespace_negated = [256]byte{
	0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1,

	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1,

	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,

	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

// return non-zero if not a structural or whitespace char
// zero otherwise
func is_not_structural_or_whitespace(c byte) byte {
	return structural_or_whitespace_negated[c]
}
