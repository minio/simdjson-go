package simdjson

import (
	"testing"
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
