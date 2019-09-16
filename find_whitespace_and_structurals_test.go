package simdjson

import (
	"testing"
)

func TestFindWhitespaceAndStructurals(t *testing.T) {

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

		find_whitespace_and_structurals([]byte(tc.input), &whitespace, &structurals)

		if whitespace != tc.expected_ws {
			t.Errorf("TestFindWhitespaceAndStructurals(%d): got: 0x%x want: 0x%x", i, whitespace, tc.expected_ws)
		}

		if structurals != tc.expected_strls {
			t.Errorf("TestFindWhitespaceAndStructurals(%d): got: 0x%x want: 0x%x", i, structurals, tc.expected_strls)
		}
	}
}
