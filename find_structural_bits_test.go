package simdjson

import (
	"fmt"
	"testing"
)

func TestFindStructuralBits(t *testing.T) {

	testCases := []struct {
		input              string
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
		structurals := find_structural_bits([]byte(tc.input), &prev_iter_ends_odd_backslash,
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

func TestFindStructuralBitsMultiple(t *testing.T) {
	_, _, msg := loadCompressed(t, "twitter")

	prev_iter_ends_odd_backslash := uint64(0)
	prev_iter_inside_quote := uint64(0) // either all zeros or all ones
	prev_iter_ends_pseudo_pred := uint64(1)
	error_mask := uint64(0) // for unescaped characters within strings (ASCII code points < 0x20)
	structurals := uint64(0)
	carried := 0

	// TODO: Deal with last batch of 64 bytes
	msg = msg[:len(msg) &^0x3f]

	for len(msg) > 0 {

		index := indexChan{}
		index.indexes = &[INDEX_SIZE]uint32{}

		processed := find_structural_bits_loop(msg, &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			structurals,
			&prev_iter_ends_pseudo_pred,
			index.indexes, &index.length, &carried)

		fmt.Println(index.length, "out of max =", INDEX_SIZE)
		msg = msg[processed:]
	}
}

func BenchmarkFindStructuralBits(b *testing.B) {

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
		find_structural_bits([]byte(msg), &prev_iter_ends_odd_backslash,
			&prev_iter_inside_quote, &error_mask,
			structurals,
			&prev_iter_ends_pseudo_pred)
	}
}

// find_structural_bits version that calls the individual assembly routines individually
func find_structural_bits_multiple_calls(buf []byte, prev_iter_ends_odd_backslash *uint64,
										 prev_iter_inside_quote, error_mask *uint64,
										 structurals uint64,
										 prev_iter_ends_pseudo_pred *uint64) (uint64) {
	quote_bits := uint64(0)
	whitespace_mask := uint64(0)

	odd_ends := find_odd_backslash_sequences(buf, prev_iter_ends_odd_backslash)

	// detect insides of quote pairs ("quote_mask") and also our quote_bits themselves
	quote_mask := find_quote_mask_and_bits(buf, odd_ends, prev_iter_inside_quote, &quote_bits, error_mask)

	find_whitespace_and_structurals(buf, &whitespace_mask, &structurals)

	// fixup structurals to reflect quotes and add pseudo-structural characters
	return finalize_structurals(structurals, whitespace_mask, quote_mask, quote_bits, prev_iter_ends_pseudo_pred)
}
