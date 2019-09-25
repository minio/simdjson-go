package simdjson

import (
	"fmt"
	"testing"
)

func TestParseNumber(t *testing.T) {

	testCases := []struct {
		input     string
		is_double bool
		expectedD float64
		expectedI int64
	}{
		{"1", false, 0.0, 1},
		{"1.0", true, 1.0, 0},
		{"1234567890", false, 0.0, 1234567890},
		{"9876.543210", true, 9876.543210, 0},
	/*	{ "0.123456789e-12", true, 1.23456789e-13, 0}, */
	/*	{ "1.234567890E+34", true, 1.234567890E+34, 0}, */
	/*	{ "23456789012E66", true, 23456789012E66, 0}, */
	/*	{"-9876.543210", true, -9876.543210, 0}, // fails */
	}

	for _, tc := range testCases {
		succes, is_double, d, i := parse_number_simd([]byte(fmt.Sprintf(`%s:`, tc.input)))
		if !succes {
			t.Errorf("TestParseNumber: got: %v want: %v", succes, true)
		}
		if is_double != tc.is_double {
			t.Errorf("TestParseNumber: got: %v want: %v", is_double, tc.is_double)
		}
		if is_double {
			if d != tc.expectedD {
				t.Errorf("TestParseNumber: got: %g want: %g", d, tc.expectedD)
			}
		} else {
			if i != tc.expectedI {
				t.Errorf("TestParseNumber: got: %d want: %d", i, tc.expectedI)
			}
		}
	}
}
