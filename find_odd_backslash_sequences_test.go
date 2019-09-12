package simdjson

import (
	"strings"
	"testing"
)

func TestFindOddBackslashSequences(t *testing.T) {

	testCases := []struct {
		prev_ends_odd      uint64
		input              string
		expected           uint64
		ends_odd_backslash uint64
	}{
		{0, `                                                                `, 0x0, 0},
		{0, `\"                                                              `, 0x2, 0},
		{0, `  \"                                                            `, 0x8, 0},
		{0, `        \"                                                      `, 0x200, 0},
		{0, `                           \"                                   `, 0x10000000, 0},
		{0, `                               \"                               `, 0x100000000, 0},
		{0, `                                                              \"`, 0x8000000000000000, 0},
		{0, `                                                               \`, 0x0, 1},
		{0, `\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"`, 0xaaaaaaaaaaaaaaaa, 0},
		{0, `"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\`, 0x5555555555555554, 1},
		{1, `                                                                `, 0x1, 0},
		{1, `\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"`, 0xaaaaaaaaaaaaaaa8, 0},
		{1, `"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\"\`, 0x5555555555555555, 1},
	}

	for i, tc := range testCases {
		prev_iter_ends_odd_backslash := tc.prev_ends_odd
		mask := find_odd_backslash_sequences([]byte(tc.input), &prev_iter_ends_odd_backslash)

		if mask != tc.expected {
			t.Errorf("TestFindOddBackslashSequences(%d): got: 0x%x want: 0x%x", i, mask, tc.expected)
		}

		if prev_iter_ends_odd_backslash != tc.ends_odd_backslash {
			t.Errorf("TestFindOddBackslashSequences(%d): got: %v want: %v", i, prev_iter_ends_odd_backslash, tc.ends_odd_backslash)
		}
	}

	// prepend test string with longer space, making sure shift to next 256-bit word is fine
	for i := uint(1); i <= 128; i++ {
		test := strings.Repeat(" ", int(i-1)) + `\"` + strings.Repeat(" ", 62+64)

		prev_iter_ends_odd_backslash := uint64(0)
		mask_lo := find_odd_backslash_sequences([]byte(test), &prev_iter_ends_odd_backslash)
		mask_hi := find_odd_backslash_sequences([]byte(test[64:]), &prev_iter_ends_odd_backslash)

		if i < 64 {
			if mask_lo != 1<<i || mask_hi != 0 {
				t.Errorf("TestFindOddBackslashSequences(%d): got: lo = 0x%x; hi = 0x%x  want: 0x%x 0x0", i, mask_lo, mask_hi, 1<<i)
			}
		} else {
			if mask_lo != 0 || mask_hi != 1<<(i-64) {
				t.Errorf("TestFindOddBackslashSequences(%d): got: lo = 0x%x; hi = 0x%x  want:  0x0 0x%x", i, mask_lo, mask_hi, 1<<(i-64))
			}
		}
	}
}
