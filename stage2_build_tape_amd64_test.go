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
	"testing"
)

func TestStage2BuildTape(t *testing.T) {

	floatHexRepresentation1 := uint64(0x69066666666667)
	floatHexRepresentation2 := uint64(0x79066666666667)

	if GOLANG_NUMBER_PARSING {
		floatHexRepresentation1 = 0x69066666666666
		floatHexRepresentation2 = 0x79066666666666
	}

	const nul = '\000'

	testCases := []struct {
		input    string
		expected []struct {
			c   byte
			val uint64
		}
	}{
		{
			`{"a":"b","c":"dd"}`,
			[]struct {
				c   byte
				val uint64
			}{
				{'r', 0xc},
				{'{', 0xb},
				{'"', 0x2},
				{nul, 0x1},
				{'"', 0x6},
				{nul, 0x1},
				{'"', 0xa},
				{nul, 0x1},
				{'"', 0xe},
				{nul, 0x2},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":"b","c":{"d":"e"}}`,
			[]struct {
				c   byte
				val uint64
			}{
				{'r', 0x10},
				{'{', 0xf},
				{'"', 0x2},
				{nul, 0x1},
				{'"', 0x6},
				{nul, 0x1},
				{'"', 0xa},
				{nul, 0x1},
				{'{', 0xe},
				{'"', 0xf},
				{nul, 0x1},
				{'"', 0x13},
				{nul, 0x1},
				{'}', 0x8},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":"b","c":[{"d":"e"},{"f":"g"}]}`,
			[]struct {
				c   byte
				val uint64
			}{
				{'r', 0x18},
				{'{', 0x17},
				{'"', 0x2},
				{nul, 0x1},
				{'"', 0x6},
				{nul, 0x1},
				{'"', 0xa},
				{nul, 0x1},
				{'[', 0x16},
				{'{', 0xf},
				{'"', 0x10},
				{nul, 0x1},
				{'"', 0x14},
				{nul, 0x1},
				{'}', 0x9},
				{'{', 0x15},
				{'"', 0x1a},
				{nul, 0x1},
				{'"', 0x1e},
				{nul, 0x1},
				{'}', 0xf},
				{']', 0x8},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":true,"b":false,"c":null}   `, // without additional spaces, is_valid_null_atom reads beyond buffer capacity
			[]struct {
				c   byte
				val uint64
			}{
				{'r', 0xd},
				{'{', 0xc},
				{'"', 0x2},
				{nul, 0x1},
				{'t', 0x0},
				{'"', 0xb},
				{nul, 0x1},
				{'f', 0x0},
				{'"', 0x15},
				{nul, 0x1},
				{'n', 0x0},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":100,"b":200.2,"c":300,"d":400.4}`,
			[]struct {
				c   byte
				val uint64
			}{
				{'r', 0x14},
				{'{', 0x13},
				{'"', 0x2},
				{nul, 0x1},
				{'l', 0x0},
				{nul, 0x64}, // 100
				{'"', 0xa},
				{nul, 0x1},
				{'d', 0x0},
				{'@', floatHexRepresentation1}, // 200.2
				{'"', 0x14},
				{nul, 0x1},
				{'l', 0x0},
				{nul, 0x12c}, // 300
				{'"', 0x1c},
				{nul, 0x1},
				{'d', 0x0},
				{'@', floatHexRepresentation2}, // 400.4
				{'}', 0x1},
				{'r', 0x0},
			},
		},
	}

	for i, tc := range testCases {

		pj := internalParsedJson{}

		if err := pj.parseMessage([]byte(tc.input)); err != nil {
			t.Errorf("TestStage2BuildTape(%d): got: %v want: nil", i, err)
		}

		if len(pj.Tape) != len(tc.expected) {
			t.Errorf("TestStage2BuildTape(%d): got: %d want: %d", i, len(pj.Tape), len(tc.expected))
		}

		for ii, tp := range pj.Tape {
			//c := "'" + string(byte(tp >> 56)) + "'"
			//if byte(tp >> 56) == 0 {
			//	c = "nul"
			//}
			//fmt.Printf("{%s, 0x%x},\n", c, tp&0xffffffffffffff)
			expected := tc.expected[ii].val | (uint64(tc.expected[ii].c) << 56)
			if tp != expected {
				t.Errorf("TestStage2BuildTape(%d): got: %d want: %d", ii, tp, expected)
			}
		}
	}
}

func TestIsValidTrueAtom(t *testing.T) {

	testCases := []struct {
		input    string
		expected bool
	}{
		{"true    ", true},
		{"true,   ", true},
		{"true}   ", true},
		{"true]   ", true},
		{"treu    ", false}, // French for true, so perhaps should be true
		{"true1   ", false},
		{"truea   ", false},
	}

	for _, tc := range testCases {
		same := is_valid_true_atom([]byte(tc.input))
		if same != tc.expected {
			t.Errorf("TestIsValidTrueAtom: got: %v want: %v", same, tc.expected)
		}
	}
}

func TestIsValidFalseAtom(t *testing.T) {

	testCases := []struct {
		input    string
		expected bool
	}{
		{"false   ", true},
		{"false,  ", true},
		{"false}  ", true},
		{"false]  ", true},
		{"flase   ", false},
		{"false1  ", false},
		{"falsea  ", false},
	}

	for _, tc := range testCases {
		same := is_valid_false_atom([]byte(tc.input))
		if same != tc.expected {
			t.Errorf("TestIsValidFalseAtom: got: %v want: %v", same, tc.expected)
		}
	}
}

func TestIsValidNullAtom(t *testing.T) {

	testCases := []struct {
		input    string
		expected bool
	}{
		{"null    ", true},
		{"null,   ", true},
		{"null}   ", true},
		{"null]   ", true},
		{"nul     ", false},
		{"null1   ", false},
		{"nulla   ", false},
	}

	for _, tc := range testCases {
		same := is_valid_null_atom([]byte(tc.input))
		if same != tc.expected {
			t.Errorf("TestIsValidNullAtom: got: %v want: %v", same, tc.expected)
		}
	}
}
