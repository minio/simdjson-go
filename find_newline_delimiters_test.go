package simdjson

import (
	_ "fmt"
	"testing"
)

func TestFindNewlineDelimiters(t *testing.T) {

	const demo_ndjson =
	`{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":801,"Height":601,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":802,"Height":602,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

	testCases := []struct {
		expected []struct {
			c   byte
			val uint64
		}
	}{
		{
			[]struct {
				c   byte
				val uint64
			}{
				// First object
				{'r', 0x27},
				{'{', 0x26},
				{'"', 0x0},
				{'{', 0x25},
				{'"', 0xa},
				{'l', 0x0},
				{0, 0x320},
				{'"', 0x14},
				{'l', 0x0},
				{0, 0x258},
				{'"', 0x1f},
				{'"', 0x29},
				{'"', 0x42},
				{'{', 0x17},
				{'"', 0x50},
				{'"', 0x58},
				{'"', 0x83},
				{'l', 0x0},
				{0, 0x7d},
				{'"', 0x8e},
				{'l', 0x0},
				{0, 0x64},
				{'}', 0xd},
				{'"', 0x98},
				{'f', 0x0},
				{'"', 0xa5},
				{'[', 0x24},
				{'l', 0x0},
				{0, 0x74},
				{'l', 0x0},
				{0, 0x3af},
				{'l', 0x0},
				{0, 0xea},
				{'l', 0x0},
				{0, 0x9789},
				{']', 0x1a},
				{'}', 0x3},
				{'}', 0x1},
				{'r', 0x0},
				//
				// Second object
				{'r', 0x4e},
				{'{', 0x4d},
				{'"', 0xad},
				{'{', 0x4c},
				{'"', 0xb7},
				{'l', 0x0},
				{0, 0x321},
				{'"', 0xc1},
				{'l', 0x0},
				{0, 0x259},
				{'"', 0xcc},
				{'"', 0xd6},
				{'"', 0xef},
				{'{', 0x3e},
				{'"', 0xfd},
				{'"', 0x105},
				{'"', 0x130},
				{'l', 0x0},
				{0, 0x7d},
				{'"', 0x13b},
				{'l', 0x0},
				{0, 0x64},
				{'}', 0x34},
				{'"', 0x145},
				{'f', 0x0},
				{'"', 0x152},
				{'[', 0x4b},
				{'l', 0x0},
				{0, 0x74},
				{'l', 0x0},
				{0, 0x3af},
				{'l', 0x0},
				{0, 0xea},
				{'l', 0x0},
				{0, 0x9789},
				{']', 0x41},
				{'}', 0x2a},
				{'}', 0x28},
				{'r', 0x27},
				//
				// Third object
				{'r', 0x75},
				{'{', 0x74},
				{'"', 0x15a},
				{'{', 0x73},
				{'"', 0x164},
				{'l', 0x0},
				{0, 0x322},
				{'"', 0x16e},
				{'l', 0x0},
				{0, 0x25a},
				{'"', 0x179},
				{'"', 0x183},
				{'"', 0x19c},
				{'{', 0x65},
				{'"', 0x1aa},
				{'"', 0x1b2},
				{'"', 0x1dd},
				{'l', 0x0},
				{0, 0x7d},
				{'"', 0x1e8},
				{'l', 0x0},
				{0, 0x64},
				{'}', 0x5b},
				{'"', 0x1f2},
				{'f', 0x0},
				{'"', 0x1ff},
				{'[', 0x72},
				{'l', 0x0},
				{0, 0x74},
				{'l', 0x0},
				{0, 0x3af},
				{'l', 0x0},
				{0, 0xea},
				{'l', 0x0},
				{0, 0x9789},
				{']', 0x68},
				{'}', 0x51},
				{'}', 0x4f},
				{'r', 0x4e},
			},
		},
	}

	indices := make([]uint32, 16)

	rows := find_newline_delimiters([]byte(demo_ndjson), indices, 0x0a)

	if rows != 3 {
		t.Errorf("TestFindNewlineDelimiters: got: %d want: 3", rows)
	}
	if indices[0] != 196 {
		t.Errorf("TestFindNewlineDelimiters: got: %d want: 196", indices[0])
	}
	if indices[1] != 393 {
		t.Errorf("TestFindNewlineDelimiters: got: %d want: 393", indices[1])
	}

	pj := internalParsedJson{}
	pj.initialize(1024)

	startIndex := uint32(0)
	for index := uint64(0); index < rows; index++ {
		end := len(demo_ndjson)
		if index < rows - 1 {
			end = int(indices[index])
		}
		if err := pj.parseMessage([]byte(demo_ndjson)[startIndex:end]); err != nil {
			t.Errorf("TestNdjson: got: %v want: nil", err)
		}
		startIndex = indices[index]
	}

	tc := testCases[0]

	if len(pj.Tape) != len(tc.expected) {
		t.Errorf("TestFindNewlineDelimiters: got: %d want: %d", len(pj.Tape), len(tc.expected))
	}

	for ii, tp := range pj.Tape {
		// fmt.Printf("{'%s', 0x%x},\n", string(byte((tp >> 56))), tp&0xffffffffffffff)
		expected := tc.expected[ii].val | (uint64(tc.expected[ii].c) << 56)
		if tp != expected {
			t.Errorf("TestFindNewlineDelimiters(%d): got: %d want: %d", ii, tp, expected)
		}
	}
}
