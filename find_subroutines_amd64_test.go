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
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/cpuid"
)

func TestFinalizeStructurals(t *testing.T) {

	testCases := []struct {
		structurals     uint64
		whitespace      uint64
		quote_mask      uint64
		quote_bits      uint64
		expected_strls  uint64
		expected_pseudo uint64
	}{
		{0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		{0x1, 0x0, 0x0, 0x0, 0x3, 0x0},
		{0x2, 0x0, 0x0, 0x0, 0x6, 0x0},
		// test to mask off anything inside quotes
		{0x2, 0x0, 0xf, 0x0, 0x0, 0x0},
		// test to add the real quote bits
		{0x8, 0x0, 0x0, 0x10, 0x28, 0x0},
		// whether the previous iteration ended on a whitespace
		{0x0, 0x8000000000000000, 0x0, 0x0, 0x0, 0x1},
		// whether the previous iteration ended on a structural character
		{0x8000000000000000, 0x0, 0x0, 0x0, 0x8000000000000000, 0x1},
		{0xf, 0xf0, 0xf00, 0xf000, 0x1000f, 0x0},
	}

	for i, tc := range testCases {
		prev_iter_ends_pseudo_pred := uint64(0)

		structurals := finalize_structurals(tc.structurals, tc.whitespace, tc.quote_mask, tc.quote_bits, &prev_iter_ends_pseudo_pred)

		if structurals != tc.expected_strls {
			t.Errorf("TestFinalizeStructurals(%d): got: 0x%x want: 0x%x", i, structurals, tc.expected_strls)
		}

		if prev_iter_ends_pseudo_pred != tc.expected_pseudo {
			t.Errorf("TestFinalizeStructurals(%d): got: 0x%x want: 0x%x", i, prev_iter_ends_pseudo_pred, tc.expected_pseudo)
		}
	}
}

func testFindNewlineDelimiters(t *testing.T, f func([]byte, uint64) uint64) {

	want := []uint64{
		0b0000000000000000000000000000000000000000000000000000000000000000,
		0b0000000000000000000000000000000000000000000000000000000000000000,
		0b0000000000000000000000000000000000000000000000000000000000000000,
		0b0000000000000000000000000000000000000000000000000000000000010000,
		0b0000000000000000000000000000000000000000000000000000000000000000,
		0b0000000000000000000000000000000000000000000000000000000000000000,
		0b0000000000000000000000000000000000000000000000000000001000000000,
		0b0000000000000000000000000000000000000000000000000000000000000000,
		0b0000000000000000000000000000000000000000000000000000000000000000,
	}

	for offset := 0; offset < len(demo_ndjson)-64; offset += 64 {
		mask := f([]byte(demo_ndjson)[offset:], 0)
		if mask != want[offset>>6] {
			t.Errorf("testFindNewlineDelimiters: got: %064b want: %064b", mask, want[offset>>6])
		}
	}
}

func TestFindNewlineDelimiters(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindNewlineDelimiters(t, _find_newline_delimiters)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindNewlineDelimiters(t, _find_newline_delimiters_avx512)
		})
	}
}

func testExcludeNewlineDelimitersWithinQuotes(t *testing.T, f func([]byte, uint64) uint64) {

	input := []byte(`  "-------------------------------------"                       `)
	input[10] = 0x0a // within quoted string, so should be ignored
	input[50] = 0x0a // outside quoted string, so should be found

	prev_iter_inside_quote, quote_bits, error_mask := uint64(0), uint64(0), uint64(0)

	odd_ends := uint64(0)
	quotemask := find_quote_mask_and_bits(input, odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)

	mask := f(input, quotemask)
	want := uint64(1 << 50)

	if mask != want {
		t.Errorf("testExcludeNewlineDelimitersWithinQuotes: got: %064b want: %064b", mask, want)
	}
}

func TestExcludeNewlineDelimitersWithinQuotes(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testExcludeNewlineDelimitersWithinQuotes(t, _find_newline_delimiters)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testExcludeNewlineDelimitersWithinQuotes(t, _find_newline_delimiters_avx512)
		})
	}

}

func testFindOddBackslashSequences(t *testing.T, f func([]byte, *uint64) uint64) {

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
		mask := f([]byte(tc.input), &prev_iter_ends_odd_backslash)

		if mask != tc.expected {
			t.Errorf("testFindOddBackslashSequences(%d): got: 0x%x want: 0x%x", i, mask, tc.expected)
		}

		if prev_iter_ends_odd_backslash != tc.ends_odd_backslash {
			t.Errorf("testFindOddBackslashSequences(%d): got: %v want: %v", i, prev_iter_ends_odd_backslash, tc.ends_odd_backslash)
		}
	}

	// prepend test string with longer space, making sure shift to next 256-bit word is fine
	for i := uint(1); i <= 128; i++ {
		test := strings.Repeat(" ", int(i-1)) + `\"` + strings.Repeat(" ", 62+64)

		prev_iter_ends_odd_backslash := uint64(0)
		mask_lo := f([]byte(test), &prev_iter_ends_odd_backslash)
		mask_hi := f([]byte(test[64:]), &prev_iter_ends_odd_backslash)

		if i < 64 {
			if mask_lo != 1<<i || mask_hi != 0 {
				t.Errorf("testFindOddBackslashSequences(%d): got: lo = 0x%x; hi = 0x%x  want: 0x%x 0x0", i, mask_lo, mask_hi, 1<<i)
			}
		} else {
			if mask_lo != 0 || mask_hi != 1<<(i-64) {
				t.Errorf("testFindOddBackslashSequences(%d): got: lo = 0x%x; hi = 0x%x  want:  0x0 0x%x", i, mask_lo, mask_hi, 1<<(i-64))
			}
		}
	}
}

func TestFindOddBackslashSequences(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindOddBackslashSequences(t, find_odd_backslash_sequences)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindOddBackslashSequences(t, find_odd_backslash_sequences_avx512)
		})
	}
}

func testFindQuoteMaskAndBits(t *testing.T, f func([]byte, uint64, *uint64, *uint64, *uint64) uint64) {

	testCases := []struct {
		inputOE      uint64 // odd_ends
		input        string
		expected     uint64
		expectedQB   uint64 // quote_bits
		expectedPIIQ uint64 // prev_iter_inside_quote
		expectedEM   uint64 // error_mask
	}{
		{0x0, `  ""                                                            `, 0x4, 0xc, 0 ,0},
		{0x0, `  "-"                                                           `, 0xc, 0x14, 0 ,0},
		{0x0, `  "--"                                                          `, 0x1c, 0x24, 0 ,0},
		{0x0, `  "---"                                                         `, 0x3c, 0x44, 0 ,0},
		{0x0, `  "-------------"                                               `, 0xfffc, 0x10004, 0 ,0},
		{0x0, `  "---------------------------------------"                     `, 0x3fffffffffc, 0x40000000004, 0 ,0},
		{0x0, `"--------------------------------------------------------------"`, 0x7fffffffffffffff, 0x8000000000000001, 0 ,0},

		// quote is not closed --> prev_iter_inside_quote should be set
		{0x0, `                                                            "---`, 0xf000000000000000, 0x1000000000000000, ^uint64(0) ,0},
		{0x0, `                                                            "", `, 0x1000000000000000, 0x3000000000000000, 0 ,0},
		{0x0, `                                                            "-",`, 0x3000000000000000, 0x5000000000000000, 0 ,0},
		{0x0, `                                                            "--"`, 0x7000000000000000, 0x9000000000000000, 0 ,0},
		{0x0, `                                                            "---`, 0xf000000000000000, 0x1000000000000000, ^uint64(0),0},

		// test previous mask ending in backslash
		{0x1, `"                                                               `, 0x0, 0x0, 0x0,0x0},
		{0x1, `"""                                                             `, 0x2, 0x6, 0x0 ,0x0},
		{0x0, `"                                                               `, 0xffffffffffffffff, 0x1, ^uint64(0),0x0},
		{0x0, `"""                                                             `, 0xfffffffffffffffd, 0x7, ^uint64(0), 0x0},

		// test invalid chars (< 0x20) that are enclosed in quotes
		{0x0, `"` + string([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}) + ` "                             `, 0x3ffffffff, 0x400000001, 0, 0x1fffffffe},
		{0x0, `"` + string([]byte{0, 32, 1, 32, 2, 32, 3, 32, 4, 32, 5, 32, 6, 32, 7, 32, 8, 32, 9, 32, 10, 32, 11, 32, 12, 32, 13, 32, 14, 32, 15, 32, 16, 32, 17, 32, 18, 32, 19, 32, 20, 32, 21, 32, 22, 32, 23, 32, 24, 32, 25, 32, 26, 32, 27, 32, 28, 32, 29, 32, 31}) + ` "`, 0x7fffffffffffffff, 0x8000000000000001, 0, 0x2aaaaaaaaaaaaaaa},
		{0x0, `" ` + string([]byte{0, 32, 1, 32, 2, 32, 3, 32, 4, 32, 5, 32, 6, 32, 7, 32, 8, 32, 9, 32, 10, 32, 11, 32, 12, 32, 13, 32, 14, 32, 15, 32, 16, 32, 17, 32, 18, 32, 19, 32, 20, 32, 21, 32, 22, 32, 23, 32, 24, 32, 25, 32, 26, 32, 27, 32, 28, 32, 29, 32, 31}) + `"`, 0x7fffffffffffffff, 0x8000000000000001, 0, 0x5555555555555554},
	}

	for i, tc := range testCases {

		prev_iter_inside_quote, quote_bits, error_mask := uint64(0), uint64(0), uint64(0)

		mask := f([]byte(tc.input), tc.inputOE, &prev_iter_inside_quote, &quote_bits, &error_mask)

		if mask != tc.expected {
			t.Errorf("testFindQuoteMaskAndBits(%d): got: 0x%x want: 0x%x", i, mask, tc.expected)
		}

		if quote_bits != tc.expectedQB {
			t.Errorf("testFindQuoteMaskAndBits(%d): got quote_bits: 0x%x want: 0x%x", i, quote_bits, tc.expectedQB)
		}

		if prev_iter_inside_quote != tc.expectedPIIQ {
			t.Errorf("testFindQuoteMaskAndBits(%d): got prev_iter_inside_quote: 0x%x want: 0x%x", i, prev_iter_inside_quote, tc.expectedPIIQ)
		}

		if error_mask != tc.expectedEM {
			t.Errorf("testFindQuoteMaskAndBits(%d): got error_mask: 0x%x want: 0x%x", i, error_mask, tc.expectedEM)
		}
	}

	testCasesPIIQ := []struct {
		inputPIIQ    uint64
		input        string
		expectedPIIQ uint64
	}{
		// prev_iter_inside_quote state remains unchanged
		{ uint64(0), `----------------------------------------------------------------`, uint64(0)},
		{ ^uint64(0), `----------------------------------------------------------------`, ^uint64(0)},

		// prev_iter_inside_quote state remains flips
		{ uint64(0), `---------------------------"------------------------------------`, ^uint64(0)},
		{ ^uint64(0), `---------------------------"------------------------------------`, uint64(0)},

		// prev_iter_inside_quote state remains flips twice (thus unchanged)
		{ uint64(0), `----------------"------------------------"----------------------`, uint64(0)},
		{ ^uint64(0), `----------------"------------------------"----------------------`, ^uint64(0)},
	}

	for i, tc := range testCasesPIIQ {

		prev_iter_inside_quote, quote_bits, error_mask := tc.inputPIIQ, uint64(0), uint64(0)

		f([]byte(tc.input), 0, &prev_iter_inside_quote, &quote_bits, &error_mask)

		if prev_iter_inside_quote != tc.expectedPIIQ {
			t.Errorf("testFindQuoteMaskAndBits(%d): got prev_iter_inside_quote: 0x%x want: 0x%x", i, prev_iter_inside_quote, tc.expectedPIIQ)
		}
	}
}

func TestFindQuoteMaskAndBits(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindQuoteMaskAndBits(t, find_quote_mask_and_bits)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindQuoteMaskAndBits(t, find_quote_mask_and_bits_avx512)
		})
	}
}

func testFindStructuralBits(t *testing.T, f func([]byte, *uint64, *uint64, *uint64, uint64, *uint64) uint64) {

	testCases := []struct {
		input string
	}{
		{`{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor`},
		{`","Thumbnail":{"Url":"http://www.example.com/image/481989943","H`},
		{`eight":125,"Width":100},"Animated":false,"IDs":[116,943,234,3879`},
	}

	prev_iter_ends_odd_backslash := uint64(0)
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred := uint64(1)
	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	structurals := uint64(0)

	// Declare same variables for 'multiple_calls' version
	prev_iter_ends_odd_backslash_MC := uint64(0)
	prev_iter_inside_quote_MC := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred_MC := uint64(1)
	error_mask_MC := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	structurals_MC := uint64(0)

	for i, tc := range testCases {

		// Call assembly routines as a single method
		structurals := f([]byte(tc.input), &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			structurals,
			&prev_iter_ends_pseudo_pred)

		// Call assembly routines individually
		structurals_MC := find_structural_bits_multiple_calls([]byte(tc.input), &prev_iter_ends_odd_backslash_MC,
			&prev_iter_inside_quote_MC, &error_mask_MC,
			structurals_MC,
			&prev_iter_ends_pseudo_pred_MC)

		// And compare the results
		if structurals != structurals_MC {
			t.Errorf("TestFindStructuralBits(%d): got: 0x%x want: 0x%x", i, structurals, structurals_MC)
		}
	}
}

func TestFindStructuralBits(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindStructuralBits(t, find_structural_bits)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindStructuralBits(t, find_structural_bits_avx512)
		})
	}
}

func testFindStructuralBitsWhitespacePadding(t *testing.T, f func([]byte, *uint64, *uint64, *uint64, *uint64, *[INDEX_SIZE]uint32, *int, *uint64, *uint64, uint64) uint64) {

	// Test whitespace padding (for partial load of last 64 bytes) with
	// string full of structural characters
	msg := `::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::`

	for l := len(msg); l >= 0; l-- {

		prev_iter_ends_odd_backslash := uint64(0)
		prev_iter_inside_quote := uint64(0) // either all zeros or all ones
		prev_iter_ends_pseudo_pred := uint64(1)
		error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
		carried := ^uint64(0)
		position := ^uint64(0)

		index := indexChan{}
		index.indexes = &[INDEX_SIZE]uint32{}

		processed := find_structural_bits_in_slice([]byte(msg[:l]), &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			&prev_iter_ends_pseudo_pred,
			index.indexes, &index.length, &carried, &position, 0)

		if processed != uint64(l) {
			t.Errorf("testFindStructuralBitsWhitespacePadding(%d): got: %d want: %d", l, processed, l)
		}
		if index.length != l {
			t.Errorf("testFindStructuralBitsWhitespacePadding(%d): got: %d want: %d", l, index.length, l)
		}

		// Compute offset of last (structural) character and verify it points to the end of the message
		lastChar := uint64(0)
		for i := 0; i < index.length; i++ {
			lastChar += uint64(index.indexes[i])
		}
		if l > 0 {
			if lastChar != uint64(l-1) {
				t.Errorf("testFindStructuralBitsWhitespacePadding(%d): got: %d want: %d", l, lastChar, uint64(l-1))
			}
		} else {
			if lastChar != uint64(l-1)-carried {
				t.Errorf("testFindStructuralBitsWhitespacePadding(%d): got: %d want: %d", l, lastChar, uint64(l-1)-carried)
			}
		}
	}
}

func TestFindStructuralBitsWhitespacePadding(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindStructuralBitsWhitespacePadding(t, find_structural_bits_in_slice)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindStructuralBitsWhitespacePadding(t, find_structural_bits_in_slice_avx512)
		})
	}
}

func testFindStructuralBitsLoop(t *testing.T, f func([]byte, *uint64, *uint64, *uint64, *uint64, *[INDEX_SIZE]uint32, *int, *uint64, *uint64, uint64) uint64) {
	msg := loadCompressed(t, "twitter")

	prev_iter_ends_odd_backslash := uint64(0)
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred := uint64(1)
	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	carried := ^uint64(0)
	position := ^uint64(0)

	indexes := make([]uint32, 0)

	for processed := uint64(0); processed < uint64(len(msg)); {
		index := indexChan{}
		index.indexes = &[INDEX_SIZE]uint32{}

		processed += f(msg[processed:], &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			&prev_iter_ends_pseudo_pred,
			index.indexes, &index.length, &carried, &position, 0)

		indexes = append(indexes, (*index.indexes)[:index.length]...)
	}

	// Last 5 expected structural (in reverse order)
	const expectedStructuralsReversed = `}}":"`
	const expectedLength = 55263

	if len(indexes) != expectedLength {
		t.Errorf("TestFindStructuralBitsLoop: got: %d want: %d", len(indexes), expectedLength)
	}

	pos, j := len(msg)-1, 0
	for i := len(indexes) - 1; i >= len(indexes)-len(expectedStructuralsReversed); i-- {

		if msg[pos] != expectedStructuralsReversed[j] {
			t.Errorf("TestFindStructuralBitsLoop: got: %c want: %c", msg[pos], expectedStructuralsReversed[j])
		}

		pos -= int(indexes[i])
		j++
	}
}

func TestFindStructuralBitsLoop(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindStructuralBitsLoop(t, find_structural_bits_in_slice)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindStructuralBitsLoop(t, find_structural_bits_in_slice_avx512)
		})
	}
}

func benchmarkFindStructuralBits(b *testing.B, f func([]byte, *uint64, *uint64, *uint64, uint64, *uint64) uint64) {

	const msg = "                                                                "
	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	prev_iter_ends_odd_backslash := uint64(0)
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred := uint64(1)
	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	structurals := uint64(0)

	for i := 0; i < b.N; i++ {
		f([]byte(msg), &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			structurals,
			&prev_iter_ends_pseudo_pred)
	}
}

func BenchmarkFindStructuralBits(b *testing.B) {
	b.Run("avx2", func(b *testing.B) {
		benchmarkFindStructuralBits(b, find_structural_bits)
	})
	if cpuid.CPU.AVX512F() {
		b.Run("avx512", func(b *testing.B) {
			benchmarkFindStructuralBits(b, find_structural_bits_avx512)
		})
	}
}

func benchmarkFindStructuralBitsLoop(b *testing.B, f func([]byte, *uint64, *uint64, *uint64, *uint64, *[INDEX_SIZE]uint32, *int, *uint64, *uint64, uint64) uint64) {

	msg := loadCompressed(b, "twitter")

	prev_iter_ends_odd_backslash := uint64(0)
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred := uint64(1)
	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	carried := ^uint64(0)
	position := ^uint64(0)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		for processed := uint64(0); processed < uint64(len(msg)); {
			index := indexChan{}
			index.indexes = &[INDEX_SIZE]uint32{}

			processed += f(msg[processed:], &prev_iter_ends_odd_backslash,
				&prev_iter_inside_quote, &error_mask,
				&prev_iter_ends_pseudo_pred,
				index.indexes, &index.length, &carried, &position, 0)
		}
	}
}

func BenchmarkFindStructuralBitsLoop(b *testing.B) {
	b.Run("avx2", func(b *testing.B) {
		benchmarkFindStructuralBitsLoop(b, find_structural_bits_in_slice)
	})
	if cpuid.CPU.AVX512F() {
		b.Run("avx512", func(b *testing.B) {
			benchmarkFindStructuralBitsLoop(b, find_structural_bits_in_slice_avx512)
		})
	}
}

func benchmarkFindStructuralBitsParallelLoop(b *testing.B, f func([]byte, *uint64, *uint64, *uint64, *uint64, *[INDEX_SIZE]uint32, *int, *uint64, *uint64, uint64) uint64) {

	msg := loadCompressed(b, "twitter")
	cpus := runtime.NumCPU()

	b.SetBytes(int64(len(msg) * cpus))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(cpus)
		for cpu := 0; cpu < cpus; cpu++ {
			go func() {
				prev_iter_ends_odd_backslash := uint64(0)
				prev_iter_inside_quote := uint64(0) // either all zeros or all ones
				prev_iter_ends_pseudo_pred := uint64(1)
				error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
				carried := ^uint64(0)
				position := ^uint64(0)

				for processed := uint64(0); processed < uint64(len(msg)); {
					index := indexChan{}
					index.indexes = &[INDEX_SIZE]uint32{}

					processed += f(msg[processed:], &prev_iter_ends_odd_backslash,
						&prev_iter_inside_quote, &error_mask,
						&prev_iter_ends_pseudo_pred,
						index.indexes, &index.length, &carried, &position, 0)
				}
				defer wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkFindStructuralBitsParallelLoop(b *testing.B) {
	b.Run("avx2", func(b *testing.B) {
		benchmarkFindStructuralBitsParallelLoop(b, find_structural_bits_in_slice)
	})
	if cpuid.CPU.AVX512F() {
		b.Run("avx512", func(b *testing.B) {
			benchmarkFindStructuralBitsParallelLoop(b, find_structural_bits_in_slice_avx512)
		})
	}
}

// find_structural_bits version that calls the individual assembly routines individually
func find_structural_bits_multiple_calls(buf []byte, prev_iter_ends_odd_backslash *uint64,
	prev_iter_inside_quote, error_mask *uint64,
	structurals uint64,
	prev_iter_ends_pseudo_pred *uint64) uint64 {
	quote_bits := uint64(0)
	whitespace_mask := uint64(0)

	odd_ends := find_odd_backslash_sequences(buf, prev_iter_ends_odd_backslash)

	// detect insides of quote pairs ("quote_mask") and also our quote_bits themselves
	quote_mask := find_quote_mask_and_bits(buf, odd_ends, prev_iter_inside_quote, &quote_bits, error_mask)

	find_whitespace_and_structurals(buf, &whitespace_mask, &structurals)

	// fixup structurals to reflect quotes and add pseudo-structural characters
	return finalize_structurals(structurals, whitespace_mask, quote_mask, quote_bits, prev_iter_ends_pseudo_pred)
}

func testFindWhitespaceAndStructurals(t *testing.T, f func([]byte, *uint64, *uint64)) {

	testCases := []struct {
		input          string
		expected_ws    uint64
		expected_strls uint64
	}{
		{`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, 0x0, 0x0},
		{` aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, 0x1, 0x0},
		{`:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, 0x0, 0x1},
		{` :aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, 0x1, 0x2},
		{`: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, 0x2, 0x1},
		{`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa `, 0x8000000000000000, 0x0},
		{`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:`, 0x0, 0x8000000000000000},
		{`a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a `, 0xaaaaaaaaaaaaaaaa, 0x0},
		{` a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a a`, 0x5555555555555555, 0x0},
		{`a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:`, 0x0, 0xaaaaaaaaaaaaaaaa},
		{`:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a:a`, 0x0, 0x5555555555555555},
		{`                                                                `, 0xffffffffffffffff, 0x0},
		{`{                                                               `, 0xfffffffffffffffe, 0x1},
		{`}                                                               `, 0xfffffffffffffffe, 0x1},
		{`"                                                               `, 0xfffffffffffffffe, 0x0},
		{`::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::::`, 0x0, 0xffffffffffffffff},
		{`{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{{`, 0x0, 0xffffffffffffffff},
		{`}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}}`, 0x0, 0xffffffffffffffff},
		{`  :                                                             `, 0xfffffffffffffffb, 0x4},
		{`    :                                                           `, 0xffffffffffffffef, 0x10},
		{`      :     :      :          :             :                  :`, 0x7fffefffbff7efbf, 0x8000100040081040},
		{demo_json, 0x421000000000000, 0x40440220301},
	}

	for i, tc := range testCases {
		whitespace := uint64(0)
		structurals := uint64(0)

		f([]byte(tc.input), &whitespace, &structurals)

		if whitespace != tc.expected_ws {
			t.Errorf("testFindWhitespaceAndStructurals(%d): got: 0x%x want: 0x%x", i, whitespace, tc.expected_ws)
		}

		if structurals != tc.expected_strls {
			t.Errorf("testFindWhitespaceAndStructurals(%d): got: 0x%x want: 0x%x", i, structurals, tc.expected_strls)
		}
	}
}

func TestFindWhitespaceAndStructurals(t *testing.T) {
	t.Run("avx2", func(t *testing.T) {
		testFindWhitespaceAndStructurals(t, find_whitespace_and_structurals)
	})
	if cpuid.CPU.AVX512F() {
		t.Run("avx512", func(t *testing.T) {
			testFindWhitespaceAndStructurals(t, find_whitespace_and_structurals_avx512)
		})
	}
}

func TestFlattenBitsIncremental(t *testing.T) {

	testCases := []struct {
		masks    []uint64
		expected []uint32
	}{
		// Single mask
		{[]uint64{0x11}, []uint32{0x1, 0x4}},
		{[]uint64{0x100100100100}, []uint32{0x9, 0xc, 0xc, 0xc}},
		{[]uint64{0x100100100300}, []uint32{0x9, 0x1, 0xb, 0xc, 0xc}},
		{[]uint64{0x8101010101010101}, []uint32{0x1, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x7}},
		{[]uint64{0x4000000000000000}, []uint32{0x3f}},
		{[]uint64{0x8000000000000000}, []uint32{0x40}},
		{[]uint64{0xf000000000000000}, []uint32{0x3d, 0x1, 0x1, 0x1}},
		{[]uint64{0xffffffffffffffff}, []uint32{
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
		}},
		////
		//// Multiple masks
		{[]uint64{0x1, 0x1000}, []uint32{0x1, 0x4c}},
		{[]uint64{0x1, 0x4000000000000000}, []uint32{0x1, 0x7e}},
		{[]uint64{0x1, 0x8000000000000000}, []uint32{0x1, 0x7f}},
		{[]uint64{0x1, 0x0, 0x8000000000000000}, []uint32{0x1, 0xbf}},
		{[]uint64{0x1, 0x0, 0x0, 0x8000000000000000}, []uint32{0x1, 0xff}},
		{[]uint64{0x100100100100100, 0x100100100100100}, []uint32{0x9, 0xc, 0xc, 0xc, 0xc, 0x10, 0xc, 0xc, 0xc, 0xc}},
		{[]uint64{0xffffffffffffffff, 0xffffffffffffffff}, []uint32{
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
			0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1, 0x1,
		}},
	}

	for i, tc := range testCases {

		index := indexChan{}
		index.indexes = &[INDEX_SIZE]uint32{}
		carried := 0
		position := ^uint64(0)

		for _, mask := range tc.masks {
			flatten_bits_incremental(index.indexes, &index.length, mask, &carried, &position)
		}

		if index.length != len(tc.expected) {
			t.Errorf("TestFlattenBitsIncremental(%d): got: %d want: %d", i, index.length, len(tc.expected))
		}

		compare := make([]uint32, 0, 1024)
		for idx := 0; idx < index.length; idx++ {
			compare = append(compare, index.indexes[idx])
		}

		if !reflect.DeepEqual(compare, tc.expected) {
			t.Errorf("TestFlattenBitsIncremental(%d): got: %v want: %v", i, compare, tc.expected)
		}
	}
}

func BenchmarkFlattenBits(b *testing.B) {

	msg := loadCompressed(b, "twitter")

	prev_iter_ends_odd_backslash := uint64(0)
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred := uint64(1)
	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	structurals := uint64(0)

	structuralsArray := make([]uint64, 0, len(msg)>>6)

	// Collect all structurals into array
	for i := 0; i < len(msg)-64; i += 64 {
		find_structural_bits([]byte(msg)[i:], &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			structurals,
			&prev_iter_ends_pseudo_pred)

		structuralsArray = append(structuralsArray, structurals)
	}

	b.SetBytes(int64(len(structuralsArray) * 8))
	b.ReportAllocs()
	b.ResetTimer()

	index := indexChan{}
	index.indexes = &[INDEX_SIZE]uint32{}
	carried := 0
	position := ^uint64(0)

	for i := 0; i < b.N; i++ {
		for _, structurals := range structuralsArray {
			index.length = 0 // reset length to prevent overflow
			flatten_bits_incremental(index.indexes, &index.length, structurals, &carried, &position)
		}
	}
}
