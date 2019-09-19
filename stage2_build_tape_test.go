package simdjson

import (
	"testing"
)

func TestStage2BuildTape(t *testing.T) {

	testCases := []struct {
		input    string
		expected []struct {
			c byte
			val uint64
		}
	}{
		{
			`{"a":"b","c":"d"}`,
			[]struct {
				c byte
				val uint64
			}{
				{'r', 0x0},
				{'{', 0x7},
				{'"', 0x0},
				{'"', 0x2},
				{'"', 0x4},
				{'"', 0x6},
				{'}', 0x1},
			},
		},
		{
			`{"a":"b","c":{"d":"e"}}`,
			[]struct {
				c byte
				val uint64
			}{
				{'r', 0x0},
				{'{', 0xa},
				{'"', 0x0},
				{'"', 0x2},
				{'"', 0x4},
				{'{', 0x9},
				{'"', 0x6},
				{'"', 0x8},
				{'}', 0x5},
				{'}', 0x1},
			},
		},
	}

	for i, tc := range testCases {

		pj := ParsedJson{}
		pj.structural_indexes = make([]uint32, 0, 1024)
		pj.tape = make([]uint64, 0, 1024)
		pj.containing_scope_offset = make([]uint64, 128)
		pj.ret_address = make([]byte, 1024)
		pj.strings = make([]byte, 1024)

		find_structural_bits([]byte(tc.input), &pj)
		unified_machine([]byte(tc.input), &pj)

		if len(pj.tape) != len(tc.expected) {
			t.Errorf("TestStage2BuildTape(%d): got: %d want: %d", i, len(pj.tape), len(tc.expected))
		}

		for ii, tp := range pj.tape {
			// fmt.Printf("{'%s', 0x%x},\n", string(byte((tp >> 56))), tp&0xffffffffffffff)
			expected := tc.expected[ii].val | (uint64(tc.expected[ii].c) << 56)
			if tp != expected {
				t.Errorf("TestStage2BuildTape(%d): got: %d want: %d", ii, tp, expected)
			}
		}
		// fmt.Println(pj.strings[:8])
	}
}
