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

	indices := make([]uint32, 1024)

	rows := find_newline_delimiters([]byte(demo_ndjson), indices, 0x0a)

	//fmt.Println(indices[:10])
	//fmt.Println(rows)

	pj := internalParsedJson{}
	pj.initialize(1024)

	startIndex := uint32(0)
	for index := uint64(0); index < rows - 1; index++ {
		if err := pj.parseMessage([]byte(demo_ndjson)[startIndex:indices[index]]); err != nil {
			t.Errorf("TestNdjson: got: %v want: nil", err)
		}
		startIndex = indices[index]
	}

	if err := pj.parseMessage([]byte(demo_ndjson)[startIndex:len(demo_ndjson)]); err != nil {
		t.Errorf("TestNdjson: got: %v want: nil", err)
	}

	pj.dump_raw_tape()
}
