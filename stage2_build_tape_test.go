package simdjson

import (
	"fmt"
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"testing"
	"path/filepath"
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
				{'r', 0x7},
				{'{', 0x7},
				{'"', 0x0},
				{'"', 0x6},
				{'"', 0xc},
				{'"', 0x12},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":"b","c":{"d":"e"}}`,
			[]struct {
				c byte
				val uint64
			}{
				{'r', 0xa},
				{'{', 0xa},
				{'"', 0x0},
				{'"', 0x6},
				{'"', 0xc},
				{'{', 0x9},
				{'"', 0x12},
				{'"', 0x18},
				{'}', 0x5},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":"b","c":[{"d":"e"},{"f":"g"}]}`,
			[]struct {
				c byte
				val uint64
			}{
				{'r', 0x10},
				{'{', 0x10},
				{'"', 0x0},
				{'"', 0x6},
				{'"', 0xc},
				{'[', 0xf},
				{'{', 0xa},
				{'"', 0x12},
				{'"', 0x18},
				{'}', 0x6},
				{'{', 0xe},
				{'"', 0x1e},
				{'"', 0x24},
				{'}', 0xa},
				{']', 0x5},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":true,"b":false,"c":null}   `, // without additional spaces, is_valid_null_atom reads beyond buffer capacity
			[]struct {
				c byte
				val uint64
			}{
				{'r', 0x9},
				{'{', 0x9},
				{'"', 0x0},
				{'t', 0x0},
				{'"', 0x6},
				{'f', 0x0},
				{'"', 0xc},
				{'n', 0x0},
				{'}', 0x1},
				{'r', 0x0},
			},
		},
		{
			`{"a":100,"b":200.2,"c":300,"d":400.4}`,
			[]struct {
				c byte
				val uint64
			}{
				{'r', 0xf},
				{'{', 0xf},
				{'"', 0x0},
				{'l', 0x0},
				{'\000', 0x64},          // 100
				{'"', 0x6},
				{'d', 0x0},
				{'@', 0x69066666666667}, // 200.2
				{'"', 0xc},
				{'l', 0x0},
				{'\000', 0x12c},         // 300
				{'"', 0x12},
				{'d', 0x0},
				{'@', 0x79066666666667}, // 400.4
				{'}', 0x1},
				{'r', 0x0},
			},
		},
	}

	for i, tc := range testCases {

		pj := ParsedJson{}
		pj.initialize(1024)

		find_structural_indices([]byte(tc.input), &pj)
		success := unified_machine([]byte(tc.input), &pj)
		if !success {
			t.Errorf("TestStage2BuildTape(%d): got: %v want: true", i, success)
		}

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
	}
}

func TestIsValidTrueAtom(t *testing.T) {

	testCases := []struct {
		input     string
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
		input     string
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
		input     string
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

func testStage2VerifyTape(t *testing.T, filename string) {

	msg, err := ioutil.ReadFile(filepath.Join("testdata", filename + ".json"))
	if err != nil {
		panic("failed to read file")
	}

	pj := ParsedJson{}
	pj.initialize(len(msg)*2)

	find_structural_indices(msg, &pj)
	success := unified_machine(msg, &pj)
	if !success {
		fmt.Errorf("Stage2 failed\n")
	}

	tape := make([]byte, len(pj.tape)*8)
	for i, t := range pj.tape {
		binary.LittleEndian.PutUint64(tape[i*8:], t)
	}
	expected, err := ioutil.ReadFile(filepath.Join("testdata", filename + ".tape"))
	if err != nil {
		panic("failed to read file")
	}

	if bytes.Compare(tape, expected) != 0 {
		t.Errorf("TestStage2VerifyTape (%s): got: %v want: %v", filename, tape, expected)
	}

	expectedStringBuf, err := ioutil.ReadFile(filepath.Join("testdata", filename + ".stringbuf"))
	if err != nil {
		panic("failed to read file")
	}

	if bytes.Compare(pj.strings, expectedStringBuf) != 0 {
		t.Errorf("TestStage2VerifyTape (%s): got: %v want: %v", filename, pj.strings, expectedStringBuf)
	}

}

func TestStage2VerifyApache_builds(t *testing.T) { testStage2VerifyTape(t, "apache_builds") }
func TestStage2VerifyCitm_catalog(t *testing.T) { testStage2VerifyTape(t, "citm_catalog") }
func TestStage2VerifyGithub_events(t *testing.T) { testStage2VerifyTape(t, "github_events") }
func TestStage2VerifyGsoc_2018(t *testing.T) { testStage2VerifyTape(t, "gsoc-2018") }
func TestStage2VerifyInstruments(t *testing.T) { testStage2VerifyTape(t, "instruments") }
func TestStage2VerifyNumbers(t *testing.T) { testStage2VerifyTape(t, "numbers") }
func TestStage2VerifyRandom(t *testing.T) { testStage2VerifyTape(t, "random") }
func TestStage2VerifyUpdate_center(t *testing.T) { testStage2VerifyTape(t, "update-center") }
