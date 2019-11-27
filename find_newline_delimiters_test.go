package simdjson

import (
	_ "fmt"
	"testing"
)

func TestFindNewlineDelimiters(t *testing.T) {

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

	verifyDemoNdjson(pj, t)
}
