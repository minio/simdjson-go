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
const retAddressShift = 2
const retAddressStartConst = 1
const retAddressObjectConst = 2
const retAddressArrayConst = 3

func updateChar(pj *internalParsedJson, idx_in uint64) (done bool, idx uint64) {
	if pj.indexesChan.index >= pj.indexesChan.length {
		var ok bool
		pj.indexesChan, ok = <-pj.indexChans // Get next element from channel
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
		pj.indexesChan, ok = <-pj.indexChans // Get next element from channel
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

func parseString(pj *ParsedJson, idx uint64, maxStringSize uint64) bool {
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
	if !parseStringSimdValidateOnly(buf, &maxStringSize, &size, &need_copy) {
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
		_ = parseStringSimd(buf, &pj.Strings) // We can safely ignore the result since we validate above
		pj.write_tape(uint64(STRINGBUFBIT+start), '"')
		size = uint64(len(pj.Strings) - start)
	}
	// put length onto the tape
	pj.Tape = append(pj.Tape, size)
	return true
}

func addNumber(buf []byte, pj *ParsedJson) bool {
	tag, val, flags := parseNumber(buf)
	if tag == TagEnd {
		return false
	}
	pj.writeTapeTagValFlags(tag, val, flags)
	return true
}

func isValidTrueAtom(buf []byte) bool {
	if len(buf) >= 8 { // fast path when there is enough space left in the buffer
		tv := uint64(0x0000000065757274) // "true    "
		mask4 := uint64(0x00000000ffffffff)
		locval := binary.LittleEndian.Uint64(buf)
		error := (locval & mask4) ^ tv
		error |= uint64(isNotStructuralOrWhitespace(buf[4]))
		return error == 0
	} else if len(buf) >= 5 {
		return bytes.Compare(buf[:4], []byte("true")) == 0 && isNotStructuralOrWhitespace(buf[4]) == 0
	}
	return false
}

func isValidFalseAtom(buf []byte) bool {
	if len(buf) >= 8 { // fast path when there is enough space left in the buffer
		fv := uint64(0x00000065736c6166) // "false   "
		mask5 := uint64(0x000000ffffffffff)
		locval := binary.LittleEndian.Uint64(buf)
		error := (locval & mask5) ^ fv
		error |= uint64(isNotStructuralOrWhitespace(buf[5]))
		return error == 0
	} else if len(buf) >= 6 {
		return bytes.Compare(buf[:5], []byte("false")) == 0 && isNotStructuralOrWhitespace(buf[5]) == 0
	}
	return false
}

func isValidNullAtom(buf []byte) bool {
	if len(buf) >= 8 { // fast path when there is enough space left in the buffer
		nv := uint64(0x000000006c6c756e) // "null    "
		mask4 := uint64(0x00000000ffffffff)
		locval := binary.LittleEndian.Uint64(buf) // we want to avoid unaligned 64-bit loads (undefined in C/C++)
		error := (locval & mask4) ^ nv
		error |= uint64(isNotStructuralOrWhitespace(buf[4]))
		return error == 0
	} else if len(buf) >= 5 {
		return bytes.Compare(buf[:4], []byte("null")) == 0 && isNotStructuralOrWhitespace(buf[4]) == 0
	}
	return false
}

func unifiedMachine(buf []byte, pj *internalParsedJson) bool {

	const addOneForRoot = 1

	done := false
	idx := ^uint64(0)   // location of the structural character in the input (buf)
	offset := uint64(0) // used to contain last element of containing_scope_offset

	////////////////////////////// START STATE /////////////////////////////
	pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressStartConst)

	pj.write_tape(0, 'r') // r for root, 0 is going to get overwritten
	// the root is used, if nothing else, to capture the size of the tape

	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
continueRoot:
	switch buf[idx] {
	case '{':
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressStartConst)
		pj.write_tape(0, buf[idx])
		goto object_begin
	case '[':
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressStartConst)
		pj.write_tape(0, buf[idx])
		goto arrayBegin
	default:
		goto fail
	}

startContinue:
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
		offset = pj.containingScopeOffset[len(pj.containingScopeOffset)-1]

		// drop last element
		pj.containingScopeOffset = pj.containingScopeOffset[:len(pj.containingScopeOffset)-1]

		pj.annotate_previousloc(offset>>retAddressShift, pj.get_current_loc()+addOneForRoot)
		pj.write_tape(offset>>retAddressShift, 'r') // r is root

		// And open a new root
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressStartConst)
		pj.write_tape(0, 'r') // r for root, 0 is going to get overwritten

		goto continueRoot
	}

	//////////////////////////////// OBJECT STATES /////////////////////////////

object_begin:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	switch buf[idx] {
	case '"':
		if !parseString(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}
		goto object_key_state
	case '}':
		goto scopeEnd // could also go to object_continue
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
		if !parseString(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}

	case 't':
		if !isValidTrueAtom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'f':
		if !isValidFalseAtom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'n':
		if !isValidNullAtom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if !addNumber(buf[idx:], &pj.ParsedJson) {
			goto fail
		}

	case '-':
		if !addNumber(buf[idx:], &pj.ParsedJson) {
			goto fail
		}

	case '{':
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressObjectConst)
		pj.write_tape(0, buf[idx])
		// we have not yet encountered } so we need to come back for it
		goto object_begin

	case '[':
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressObjectConst)
		pj.write_tape(0, buf[idx])
		// we have not yet encountered } so we need to come back for it
		goto arrayBegin

	default:
		goto fail
	}

objectContinue:
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
		if !parseString(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}
		goto object_key_state

	case '}':
		goto scopeEnd

	default:
		goto fail
	}

	////////////////////////////// COMMON STATE /////////////////////////////
scopeEnd:
	// write our tape location to the header scope
	offset = pj.containingScopeOffset[len(pj.containingScopeOffset)-1]
	// drop last element
	pj.containingScopeOffset = pj.containingScopeOffset[:len(pj.containingScopeOffset)-1]

	pj.write_tape(offset>>retAddressShift, buf[idx])
	pj.annotate_previousloc(offset>>retAddressShift, pj.get_current_loc())

	/* goto saved_state*/
	switch offset & ((1 << retAddressShift) - 1) {
	case retAddressArrayConst:
		goto arrayContinue
	case retAddressObjectConst:
		goto objectContinue
	default:
		goto startContinue
	}

	////////////////////////////// ARRAY STATES /////////////////////////////
arrayBegin:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	if buf[idx] == ']' {
		goto scopeEnd // could also go to array_continue
	}

mainArraySwitch:
	// we call update char on all paths in, so we can peek at c on the
	// on paths that can accept a close square brace (post-, and at start)
	switch buf[idx] {
	case '"':
		if !parseString(&pj.ParsedJson, idx, peekSize(pj)) {
			goto fail
		}
	case 't':
		if !isValidTrueAtom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'f':
		if !isValidFalseAtom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])

	case 'n':
		if !isValidNullAtom(buf[idx:]) {
			goto fail
		}
		pj.write_tape(0, buf[idx])
		/* goto array_continue */

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-':
		if !addNumber(buf[idx:], &pj.ParsedJson) {
			goto fail
		}

	case '{':
		// we have not yet encountered ] so we need to come back for it
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressArrayConst)
		pj.write_tape(0, buf[idx]) //  here the compilers knows what c is so this gets optimized
		goto object_begin

	case '[':
		// we have not yet encountered ] so we need to come back for it
		pj.containingScopeOffset = append(pj.containingScopeOffset, (pj.get_current_loc()<<retAddressShift)|retAddressArrayConst)
		pj.write_tape(0, buf[idx]) // here the compilers knows what c is so this gets optimized
		goto arrayBegin

	default:
		goto fail
	}

arrayContinue:
	if done, idx = updateChar(pj, idx); done {
		goto succeed
	}
	switch buf[idx] {
	case ',':
		if done, idx = updateChar(pj, idx); done {
			goto succeed
		}
		goto mainArraySwitch

	case ']':
		goto scopeEnd

	default:
		goto fail
	}

	////////////////////////////// FINAL STATES /////////////////////////////
succeed:
	offset = pj.containingScopeOffset[len(pj.containingScopeOffset)-1]
	// drop last element
	pj.containingScopeOffset = pj.containingScopeOffset[:len(pj.containingScopeOffset)-1]

	// Sanity checks
	if len(pj.containingScopeOffset) != 0 {
		return false
	}

	pj.annotate_previousloc(offset>>retAddressShift, pj.get_current_loc()+addOneForRoot)
	pj.write_tape(offset>>retAddressShift, 'r') // r is root

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
var structuralOrWhitespaceNegated = [256]byte{
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
func isNotStructuralOrWhitespace(c byte) byte {
	return structuralOrWhitespaceNegated[c]
}
