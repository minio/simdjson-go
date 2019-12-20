package simdjson

import (
	"testing"
)

func TestFindNewlineDelimiters(t *testing.T) {

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

	for offset := 0; offset < len(demo_ndjson) - 64; offset += 64 {
		mask := _find_newline_delimiters([]byte(demo_ndjson)[offset:])
		if mask != want[offset >> 6] {
			t.Errorf("TestFindNewlineDelimiters: got: %064b want: %064b", mask, want[offset >> 6])
		}
	}
}

func TestExcludeNewlineDelimitersWithinQuotes(t *testing.T) {

	input := []byte(`  "-------------------------------------"                       `)
	input[10] = 0x0a // within quoted string, so should be ignored
	input[50] = 0x0a // outside quoted string, so should be found

	prev_iter_inside_quote, quote_bits, error_mask := uint64(0), uint64(0), uint64(0)

	odd_ends := uint64(0)
	quotemask := find_quote_mask_and_bits(input, odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)

	mask := _find_newline_delimiters(input) & ^quotemask
	want := uint64(1 << 50)

	if mask != want {
		t.Errorf("TestExcludeNewlineDelimitersWithinQuotes: got: %064b want: %064b", mask, want)
	}
}
