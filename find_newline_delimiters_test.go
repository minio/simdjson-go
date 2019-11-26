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
}
