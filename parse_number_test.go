package simdjson

import (
	"fmt"
	"math"
	"testing"
	"strconv"
	"reflect"
)

func closeEnough(d1, d2 float64) (ce bool) {
	return math.Abs(d1 - d2) / (0.5*(d1 + d2)) < 1e-12
}

func TestParseNumber(t *testing.T) {

	testCases := []struct {
		input     string
		is_double bool
		expectedD float64
		expectedI int64
	}{
		{"1", false, 0.0, 1},
		{"-1", false, 0.0, -1},
		{"1.0", true, 1.0, 0},
		{"1234567890", false, 0.0, 1234567890},
		{"9876.543210", true, 9876.543210, 0},
		{ "0.123456789e-12", true, 1.23456789e-13, 0},
		{ "1.234567890E+34", true, 1.234567890E+34, 0},
		{ "23456789012E66", true, 23456789012E66, 0},
		{"-9876.543210", true, -9876.543210, 0}, // fails
	}

	for _, tc := range testCases {
		found_minus := false
		if tc.input[0] == '-' {
			found_minus = true
		}
		succes, is_double, d, i := parse_number_simd([]byte(fmt.Sprintf(`%s:`, tc.input)), found_minus)
		if !succes {
			t.Errorf("TestParseNumber: got: %v want: %v", succes, true)
		}
		if is_double != tc.is_double {
			t.Errorf("TestParseNumber: got: %v want: %v", is_double, tc.is_double)
		}
		if is_double {
			if !closeEnough(d, tc.expectedD) {
				t.Errorf("TestParseNumber: got: %g want: %g", d, tc.expectedD)
			}
		} else {
			if i != tc.expectedI {
				t.Errorf("TestParseNumber: got: %d want: %d", i, tc.expectedI)
			}
		}
	}
}

func TestParseInt64(t *testing.T) {
	for i := range parseInt64Tests {
		test := &parseInt64Tests[i]

		found_minus := false
		if test.in[0] == '-' {
			found_minus = true
		}
		succes, is_double, _, i := parse_number_simd([]byte(fmt.Sprintf(`%s:`, test.in)), found_minus)
		if !succes {
			// Ignore intentionally bad syntactical errors
			if !reflect.DeepEqual(test.err, strconv.ErrSyntax) {
				t.Errorf("TestParseInt64: got: %v want: %v", succes, true)
			}
			continue // skip testing the rest for this test case
		}
		if is_double {
			t.Errorf("TestParseInt64: got: %v want: %v", is_double, false)
		}
		if i != test.out {
			// Ignore intentionally wrong conversions
			if !reflect.DeepEqual(test.err, strconv.ErrRange) {
				t.Errorf("TestParseInt64: got: %v want: %v", i, test.out)
			}
		}
	}
}

// The following code is borrowed from Golang (https://golang.org/src/strconv/atoi_test.go)

type parseInt64Test struct {
	in  string
	out int64
	err error
}

var parseInt64Tests = []parseInt64Test{
//	{"", 0, strconv.ErrSyntax}, /* fails for simdjson */
	{"0", 0, nil},
	{"-0", 0, nil},
	{"1", 1, nil},
	{"-1", -1, nil},
	{"12345", 12345, nil},
	{"-12345", -12345, nil},
//	{"012345", 12345, nil},   /* fails for simdjson */
//	{"-012345", -12345, nil}, /* fails for simdjson */
	{"98765432100", 98765432100, nil},
	{"-98765432100", -98765432100, nil},
	{"9223372036854775807", 1<<63 - 1, nil},
	{"-9223372036854775807", -(1<<63 - 1), nil},
	{"9223372036854775808", 1<<63 - 1, strconv.ErrRange},
	{"-9223372036854775808", -1 << 63, nil},
	{"9223372036854775809", 1<<63 - 1, strconv.ErrRange},
	{"-9223372036854775809", -1 << 63, strconv.ErrRange},
	{"-1_2_3_4_5", 0, strconv.ErrSyntax}, // base=10 so no underscores allowed
	{"-_12345", 0, strconv.ErrSyntax},
	{"_12345", 0, strconv.ErrSyntax},
	{"1__2345", 0, strconv.ErrSyntax},
	{"12345_", 0, strconv.ErrSyntax},
}
