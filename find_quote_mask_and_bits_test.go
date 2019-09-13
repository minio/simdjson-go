package simdjson

import (
	_ "fmt"
	"testing"
)

func TestFindQuoteMaskAndBits(t *testing.T) {

	testCases := []struct {
		input    string
		expected uint64
	}{
		{`  ""                                                              `, 0x4},
		{`  "-"                                                             `, 0xc},
		{`  "--"                                                            `, 0x1c},
		{`  "---"                                                           `, 0x3c},
		{`  "-------------"                                                 `, 0xfffc},
		{`  "---------------------------------------"                       `, 0x3fffffffffc},
		{`"----------------------------------------------------------------"`, 0xffffffffffffffff},
	}

	for i, tc := range testCases {

		odd_ends := uint64(0)
		prev_iter_inside_quote, quote_bits, error_mask := uint64(0), uint64(0), uint64(0)

		mask := find_quote_mask_and_bits([]byte(tc.input), odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)

		if mask != tc.expected {
			t.Errorf("TestFindOddBackslashSequences(%d): got: 0x%x want: 0x%x", i, mask, tc.expected)
		}
	}
}
