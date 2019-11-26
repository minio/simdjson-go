package simdjson

import (
	"testing"
)

func TestNdjson(t *testing.T) {

	const demo_ndjson = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":801,"Height":601,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":802,"Height":602,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

	pj := internalParsedJson{}
	pj.initialize(len(demo_ndjson))

	if err := pj.parseMessageNdjson([]byte(demo_ndjson)); err != nil {
		t.Errorf("TestFindNewlineDelimitersHack: got: %v want: nil", err)
	}

	pj.dump_raw_tape()
}
