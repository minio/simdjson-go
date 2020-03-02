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
	"strconv"
	"unicode"
	"unsafe"
)

//go:noescape
func _parse_number(buf unsafe.Pointer, offset, found_minus uint64, is_double, resultDouble, resultInt64 unsafe.Pointer) (success uint64)

func parse_number_simd(buf []byte, found_minus bool) (success, is_double bool, d float64, i int) {

	if GOLANG_NUMBER_PARSING {

		pos := 0
		for ; pos < len(buf) && (unicode.IsDigit(rune(buf[pos])) || buf[pos] == '.' || buf[pos] == '+' || buf[pos] == '-' || buf[pos] == 'e' || buf[pos] == 'E'); pos++ {
		}

		var err error
		i, err = strconv.Atoi(string(buf[:pos]))
		if err == nil {
			success = true
			return
		}
		d, err = strconv.ParseFloat(string(buf[:pos]), 64)
		if err == nil {
			success, is_double = true, true
		}
		return
	}

	src := uintptr(unsafe.Pointer(&buf[0]))

	fm := uint64(0)
	if found_minus {
		fm = 1
	}

	success = _parse_number(unsafe.Pointer(src), 0, fm, unsafe.Pointer(&is_double), unsafe.Pointer(&d), unsafe.Pointer(&i)) != 0

	return
}
