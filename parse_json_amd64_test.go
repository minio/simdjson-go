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
	"fmt"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

func TestDemoNdjson(t *testing.T) {

	pj := internalParsedJson{}

	if err := pj.parseMessageNdjson([]byte(demo_ndjson)); err != nil {
		t.Errorf("TestDemoNdjson: got: %v want: nil", err)
	}

	verifyDemoNdjson(pj, t, 0)
}

func TestNdjsonEmptyLines(t *testing.T) {

	ndjson_emptylines := []string{`{"zero":"emptylines"}
{"c":"d"}`,
		`{"single":"emptyline"}

{"c":"d"}`,
		`{"dual":"emptylines"}


{"c":"d"}`,
		`{"triple":"emptylines"}



{"c":"d"}`}

	pj := internalParsedJson{}

	for _, json := range ndjson_emptylines {
		if err := pj.parseMessageNdjson([]byte(json)); err != nil {
			t.Errorf("TestNdjsonEmptyLine: got: %v want: nil", err)
		}
	}
}

func BenchmarkNdjsonStage2(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")
	pj := internalParsedJson{}

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := pj.parseMessageNdjson(ndjson)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkNdjsonStage1(b *testing.B) {

	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create new channel (large enough so we won't block)
		pj.index_chan = make(chan indexChan, 128*10240)
		find_structural_indices([]byte(ndjson), &pj)
	}
}

func BenchmarkNdjsonColdCountStar(b *testing.B) {

	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	// Allocate stuff
	pj := internalParsedJson{}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pj.parseMessageNdjson(ndjson)
		count_raw_tape(pj.Tape)
	}
}

func BenchmarkNdjsonColdCountStarWithWhere(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")
	const want = 110349
	runtime.GC()
	pj := internalParsedJson{}

	b.Run("raw", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			err := pj.parseMessage(ndjson)
			if err != nil {
				b.Fatal(err)
			}
			got := countRawTapeWhere("Make", "HOND", pj.ParsedJson)
			if got != want {
				b.Fatal(got, "!=", want)
			}
		}
	})
	b.Run("iter", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			err := pj.parseMessage(ndjson)
			if err != nil {
				b.Fatal(err)
			}
			got := countWhere("Make", "HOND", pj.ParsedJson)
			if got != want {
				b.Fatal(got, "!=", want)
			}
		}
	})
}

func TestParseNumber(t *testing.T) {

	if GOLANG_NUMBER_PARSING {
		t.Skip()
	}

	testCases := []struct {
		input     string
		is_double bool
		expectedD float64
		expectedI int
	}{
		{"1", false, 0.0, 1},
		{"-1", false, 0.0, -1},
		{"1.0", true, 1.0, 0},
		{"1234567890", false, 0.0, 1234567890},
		{"9876.543210", true, 9876.543210, 0},
		{"0.123456789e-12", true, 1.23456789e-13, 0},
		{"1.234567890E+34", true, 1.234567890e+34, 0},
		{"23456789012E66", true, 23456789012e66, 0},
		{"-9876.543210", true, -9876.543210, 0},
		// The number below parses to -65.61972000000004 for parse_number()
		// This extra inprecision is tolerated when GOLANG_NUMBER_PARSING = false
		{"-65.619720000000029", true, -65.61972000000003, 0},
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
				if GOLANG_NUMBER_PARSING {
					t.Errorf("TestParseNumber: got: %g want: %g", d, tc.expectedD)
				} else {
					if !closeEnoughLessPrecision(d, tc.expectedD) {
						t.Errorf("TestParseNumber: got: %g want: %g", d, tc.expectedD)
					}
				}
			}
		} else {
			if i != tc.expectedI {
				t.Errorf("TestParseNumber: got: %d want: %d", i, tc.expectedI)
			}
		}
	}
}

func TestParseInt64(t *testing.T) {

	if GOLANG_NUMBER_PARSING {
		t.Skip()
	}

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

func TestParseFloat64(t *testing.T) {

	if GOLANG_NUMBER_PARSING {
		t.Skip()
	}

	for i := 0; i < len(atoftests); i++ {
		test := &atoftests[i]

		found_minus := false
		if test.in[0] == '-' {
			found_minus = true
		}
		succes, is_double, d, _ := parse_number_simd([]byte(fmt.Sprintf(`%s:`, test.in)), found_minus)
		if !succes {
			// Ignore intentionally bad syntactical errors
			if !reflect.DeepEqual(test.err, strconv.ErrSyntax) {
				t.Errorf("TestParseFloat64: got: %v want: %v", succes, true)
			}
			continue // skip testing the rest for this test case
		}
		if !is_double {
			t.Errorf("TestParseFloat64: got: %v want: %v", is_double, true)
		}
		outs := strconv.FormatFloat(d, 'g', -1, 64)
		if outs != test.out {
			t.Errorf("TestParseFloat64: got: %v want: %v", d, test.out)
		}
	}
}

// The following code is borrowed from Golang (https://golang.org/src/strconv/atoi_test.go)

type parseInt64Test struct {
	in  string
	out int
	err error
}

var parseInt64Tests = []parseInt64Test{
	//	{"", 0, strconv.ErrSyntax},                                  /* fails for simdjson */
	{"0", 0, nil},
	{"-0", 0, nil},
	{"1", 1, nil},
	{"-1", -1, nil},
	{"12345", 12345, nil},
	{"-12345", -12345, nil},
	//	{"012345", 12345, nil},                                      /* fails for simdjson */
	//	{"-012345", -12345, nil},                                    /* fails for simdjson */
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

	// zero (originate from atof tests below, but returned as int for simdjson)
	{"0e0", 0, nil},
	{"-0e0", 0, nil},
	{"0e-0", 0, nil},
	{"-0e-0", 0, nil},
	{"0e+0", 0, nil},
	{"-0e+0", 0, nil},
}

// The following code is borrowed from Golang (https://golang.org/src/strconv/atof_test.go)

type atofTest struct {
	in  string
	out string
	err error
}

var atoftests = []atofTest{
	//	{"", "0", strconv.ErrSyntax},                                /* fails for simdjson */
	//	{"1", "1", nil},                                             /* parsed as int for simdjson */
	//	{"+1", "1", nil},                                            /* parsed as int for simdjson */
	{"1x", "0", strconv.ErrSyntax},
	{"1.1.", "0", strconv.ErrSyntax},
	{"1e23", "1e+23", nil},
	{"1E23", "1e+23", nil},
	//	{"100000000000000000000000", "1e+23", nil},                  /* parsed as int for simdjson */
	{"1e-100", "1e-100", nil},
	//	{"123456700", "1.234567e+08", nil},                          /* parsed as int for simdjson */
	//	{"99999999999999974834176", "9.999999999999997e+22", nil},   /* parsed as int for simdjson */
	//	{"100000000000000000000001", "1.0000000000000001e+23", nil}, /* parsed as int for simdjson */
	//	{"100000000000000008388608", "1.0000000000000001e+23", nil}, /* parsed as int for simdjson */
	//	{"100000000000000016777215", "1.0000000000000001e+23", nil}, /* parsed as int for simdjson */
	//	{"100000000000000016777216", "1.0000000000000003e+23", nil}, /* parsed as int for simdjson */
	//	{"-1", "-1", nil},                                           /* parsed as int for simdjson */
	{"-0.1", "-0.1", nil},
	//	{"-0", "-0", nil},                                           /* parsed as int for simdjson */
	{"1e-20", "1e-20", nil},
	{"625e-3", "0.625", nil},

	// Hexadecimal floating-point.                               /* all fail for simdjson */

	// zeros (several test cases for zero have been moved up because they are detected as ints)
	//	{"+0e0", "0", nil},                                          /* fails for simdjson */
	//	{"+0e-0", "0", nil},                                         /* fails for simdjson */
	//	{"+0e+0", "0", nil},                                         /* fails for simdjson */
	//	{"0e+01234567890123456789", "0", nil},                       /* fails for simdjson */
	//	{"0.00e-01234567890123456789", "0", nil},                    /* fails for simdjson */
	//	{"-0e+01234567890123456789", "-0", nil},                     /* fails for simdjson */
	//	{"-0.00e-01234567890123456789", "-0", nil},                  /* fails for simdjson */

	{"0e291", "0", nil}, // issue 15364
	{"0e292", "0", nil}, // issue 15364
	{"0e347", "0", nil}, // issue 15364
	{"0e348", "0", nil}, // issue 15364
	//	{"-0e291", "-0", nil},                                       /* returns "0" */
	//	{"-0e292", "-0", nil},                                       /* returns "0" */
	//	{"-0e347", "-0", nil},                                       /* returns "0" */
	//	{"-0e348", "-0", nil},                                       /* returns "0" */

	// NaNs
	//	{"nan", "NaN", nil},                                         /* fails for simdjson */
	//	{"NaN", "NaN", nil},                                         /* fails for simdjson */
	//	{"NAN", "NaN", nil},                                         /* fails for simdjson */

	// Infs
	//	{"inf", "+Inf", nil},                                        /* fails for simdjson */
	//	{"-Inf", "-Inf", nil},                                       /* fails for simdjson */
	//	{"+INF", "+Inf", nil},                                       /* fails for simdjson */
	//	{"-Infinity", "-Inf", nil},                                  /* fails for simdjson */
	//	{"+INFINITY", "+Inf", nil},                                  /* fails for simdjson */
	//	{"Infinity", "+Inf", nil},                                   /* fails for simdjson */

	// largest float64
	{"1.7976931348623157e308", "1.7976931348623157e+308", nil},
	{"-1.7976931348623157e308", "-1.7976931348623157e+308", nil},

	// next float64 - too large
	{"1.7976931348623159e308", "+Inf", strconv.ErrRange},
	{"-1.7976931348623159e308", "-Inf", strconv.ErrRange},

	// the border is ...158079
	// borderline - okay
	//	{"1.7976931348623158e308", "1.7976931348623157e+308", nil},  /* returns "+Inf" */
	//	{"-1.7976931348623158e308", "-1.7976931348623157e+308", nil},/* returns "-Inf" */

	// borderline - too large
	{"1.797693134862315808e308", "+Inf", strconv.ErrRange},
	{"-1.797693134862315808e308", "-Inf", strconv.ErrRange},

	// a little too large
	{"1e308", "1e+308", nil},
	{"2e308", "+Inf", strconv.ErrRange},
	//	{"1e309", "+Inf", strconv.ErrRange},                         /* fails for simdjson */

	// way too large
	//	{"1e310", "+Inf", strconv.ErrRange},                         /* fails for simdjson */
	//	{"-1e310", "-Inf", strconv.ErrRange},                        /* fails for simdjson */
	//	{"1e400", "+Inf", strconv.ErrRange},                         /* fails for simdjson */
	//	{"-1e400", "-Inf", strconv.ErrRange},                        /* fails for simdjson */
	//	{"1e400000", "+Inf", strconv.ErrRange},                      /* fails for simdjson */
	//	{"-1e400000", "-Inf", strconv.ErrRange},                     /* fails for simdjson */

	// denormalized
	{"1e-305", "1e-305", nil},
	{"1e-306", "1e-306", nil},
	{"1e-307", "1e-307", nil},
	{"1e-308", "1e-308", nil},
	//	{"1e-309", "1e-309", nil},                                   /* fails for simdjson */
	//	{"1e-310", "1e-310", nil},                                   /* fails for simdjson */
	//	{"1e-322", "1e-322", nil},                                   /* fails for simdjson */
	// smallest denormal
	//	{"5e-324", "5e-324", nil},                                   /* fails for simdjson */
	//	{"4e-324", "5e-324", nil},                                   /* fails for simdjson */
	//	{"3e-324", "5e-324", nil},                                   /* fails for simdjson */
	// too small
	//	{"2e-324", "0", nil},                                        /* fails for simdjson */
	// way too small
	//	{"1e-350", "0", nil},                                        /* fails for simdjson */
	//	{"1e-400000", "0", nil},                                     /* fails for simdjson */

	// try to overflow exponent
	//	{"1e-4294967296", "0", nil},                                 /* fails for simdjson */
	//	{"1e+4294967296", "+Inf", strconv.ErrRange},                 /* fails for simdjson */
	//	{"1e-18446744073709551616", "0", nil},                       /* fails for simdjson */
	//	{"1e+18446744073709551616", "+Inf", strconv.ErrRange},       /* fails for simdjson */

	// Parse errors
	{"1e", "0", strconv.ErrSyntax},
	{"1e-", "0", strconv.ErrSyntax},
	{".e-1", "0", strconv.ErrSyntax},

	// https://www.exploringbinary.com/java-hangs-when-converting-2-2250738585072012e-308/
	//	{"2.2250738585072012e-308", "2.2250738585072014e-308", nil}, /* fails for simdjson */
	// https://www.exploringbinary.com/php-hangs-on-numeric-value-2-2250738585072011e-308/
	//	{"2.2250738585072011e-308", "2.225073858507201e-308", nil},  /* fails for simdjson */

	// A very large number (initially wrongly parsed by the fast algorithm).
	//	{"4.630813248087435e+307", "4.630813248087435e+307", nil},   /* fails for simdjson */

	// A different kind of very large number.
	//	{"22.222222222222222", "22.22222222222222", nil},            /* fails for simdjson */
	//	{"2." + strings.Repeat("2", 4000) + "e+1", "22.22222222222222", nil},

	// Exactly halfway between 1 and math.Nextafter(1, 2).
	// Round to even (down).
	//	{"1.00000000000000011102230246251565404236316680908203125", "1", nil}, /* fails for simdjson */
	// Slightly lower; still round down.
	//	{"1.00000000000000011102230246251565404236316680908203124", "1", nil}, /* fails for simdjson */
	// Slightly higher; round up.
	//	{"1.00000000000000011102230246251565404236316680908203126", "1.0000000000000002", nil}, /* fails for simdjson */
	// Slightly higher, but you have to read all the way to the end.
	//	{"1.00000000000000011102230246251565404236316680908203125" + strings.Repeat("0", 10000) + "1", "1.0000000000000002", nil},  /* fails for simdjson */

	// Halfway between x := math.Nextafter(1, 2) and math.Nextafter(x, 2)
	// Round to even (up).
	//	{"1.00000000000000033306690738754696212708950042724609375", "1.0000000000000004", nil}, /* fails for simdjson */

	// Underscores.
	//	{"1_23.50_0_0e+1_2", "1.235e+14", nil},                      /* fails for simdjson */
	{"-_123.5e+12", "0", strconv.ErrSyntax},
	{"+_123.5e+12", "0", strconv.ErrSyntax},
	{"_123.5e+12", "0", strconv.ErrSyntax},
	{"1__23.5e+12", "0", strconv.ErrSyntax},
	{"123_.5e+12", "0", strconv.ErrSyntax},
	{"123._5e+12", "0", strconv.ErrSyntax},
	{"123.5_e+12", "0", strconv.ErrSyntax},
	{"123.5__0e+12", "0", strconv.ErrSyntax},
	{"123.5e_+12", "0", strconv.ErrSyntax},
	{"123.5e+_12", "0", strconv.ErrSyntax},
	{"123.5e_-12", "0", strconv.ErrSyntax},
	{"123.5e-_12", "0", strconv.ErrSyntax},
	{"123.5e+1__2", "0", strconv.ErrSyntax},
	{"123.5e+12_", "0", strconv.ErrSyntax},
}

func TestParseString(t *testing.T) {

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// enclose test string in quotes (as validated by stage 1)
			buf := []byte(fmt.Sprintf(`"%s"`, tt.str))
			dest := make([]byte, 0, len(buf)+32 /* safety margin as parse_string writes full AVX2 words */)

			success := parse_string_simd(buf, &dest)

			if success != tt.success {
				t.Errorf("TestParseString() got = %v, want %v", success, tt.success)
			}
			if success {
				size := len(dest)
				if size != len(tt.want) {
					t.Errorf("TestParseString() got = %d, want %d", size, len(tt.want))
				}
				if bytes.Compare(dest[:size], tt.want) != 0 {
					t.Errorf("TestParseString() got = %v, want %v", dest[:size], tt.want)
				}
			}
		})
	}
}

func TestParseStringValidateOnly(t *testing.T) {

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// enclose test string in quotes (as validated by stage 1)
			buf := []byte(fmt.Sprintf(`"%s"`, tt.str))

			dst_length := uint64(0)
			need_copy := false
			l := uint64(len(buf))
			success := parse_string_simd_validate_only(buf, &l, &dst_length, &need_copy)

			if success != tt.success {
				t.Errorf("TestParseString() got = %v, want %v", success, tt.success)
			}
			if success && !need_copy {
				if dst_length != uint64(len(tt.want)) {
					t.Errorf("TestParseString() got = %d, want %d", dst_length, len(tt.want))
				}
			}
		})
	}
}

func TestParseStringValidateOnlyBeyondBuffer(t *testing.T) {

	t.Skip()

	buf := []byte(fmt.Sprintf(`"%s`, "   "))

	dst_length := uint64(0)
	need_copy := false
	l := uint64(len(buf)) + 32
	success := parse_string_simd_validate_only(buf, &l, &dst_length, &need_copy)
	if !success {
		t.Errorf("TestParseStringValidateOnlyBeyondBuffer() got = %v, want %v", success, false)
	}
}

// Benchmarking code for integers

func BenchmarkParseNumber(b *testing.B) {
	b.Run("Pos", func(b *testing.B) {
		benchmarkParseNumber(b, 1)
	})
	b.Run("Neg", func(b *testing.B) {
		benchmarkParseNumber(b, -1)
	})
}

func benchmarkParseNumber(b *testing.B, neg int) {
	cases := []benchCase{
		{"63bit", 1<<63 - 1},
	}
	for _, cs := range cases {
		b.Run(cs.name, func(b *testing.B) {
			s := fmt.Sprintf("%d", cs.num*int64(neg))
			s = fmt.Sprintf(`%s:`, s) // append delimiter
			found_minus := false
			if neg != 0 {
				found_minus = true
			}
			for i := 0; i < b.N; i++ {
				_, _, _, i := parse_number_simd([]byte(s), found_minus)
				BenchSink += int(i)
			}
		})
	}
}

func BenchmarkParseNumberFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parse_number_simd([]byte("339.7784:"), false)
	}
}

func BenchmarkParseAtof64FloatGolang(b *testing.B) {
	for i := 0; i < b.N; i++ {
		strconv.ParseFloat("339.7784", 64)
	}
}

func BenchmarkParseNumberFloatExp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parse_number_simd([]byte("-5.09e75:"), false)
	}
}

func BenchmarkParseNumberBig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parse_number_simd([]byte("123456789123456789123456789:"), false)
	}
}

func BenchmarkParseNumberRandomBits(b *testing.B) {
	initAtof()
	for i := 0; i < b.N; i++ {
		parse_number_simd([]byte(benchmarksRandomBitsSimd[i%1024]), false)
	}
}

func BenchmarkParseNumberRandomFloats(b *testing.B) {
	initAtof()
	for i := 0; i < b.N; i++ {
		parse_number_simd([]byte(benchmarksRandomNormalSimd[i%1024]), false)
	}
}

func TestVerifyTape(t *testing.T) {
	// FIXME: Does not have tapes any more.
	for _, tt := range testCases {

		t.Run(tt.name, func(t *testing.T) {
			ref := loadCompressed(t, tt.name)

			pj := internalParsedJson{}
			if err := pj.parseMessage(ref); err != nil {
				t.Errorf("parseMessage failed: %v\n", err)
				return
			}

			//ctape := bytesToUint64(cbuf)

			//testCTapeCtoGoTapeCompare(t, ctape, csbuf, pj)
		})
	}
}
