package simdjson

import (
	"testing"
	"bytes"
	"io/ioutil"
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

func getPatchedNdjson(filename string) []byte {
	ndjson, err := ioutil.ReadFile(filename)
	if err != nil {
		panic("Failed to load file")
	}
	return bytes.ReplaceAll([]byte(ndjson), []byte("\n"), []byte("{"))
}

func BenchmarkNdjsonStage1(b *testing.B) {

	ndjson := getPatchedNdjson("parking-citations-1M.json")

	pj := internalParsedJson{}

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create new channel (large enough so we won't block)
		pj.index_chan = make(chan indexChan, 128*10240)
		find_structural_indices([]byte(ndjson), &pj)
	}
}

func BenchmarkNdjsonStage2(b *testing.B) {

	ndjson := getPatchedNdjson("parking-citations-1M.json")

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pj := internalParsedJson{}
		pj.initialize(len(ndjson)*3/2)
		pj.parseMessage(ndjson)
	}
}
