//+build !noasm
//+build !appengine

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
