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
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

const demo_ndjson = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":801,"Height":601,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":802,"Height":602,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

func verifyDemoNdjson(pj internalParsedJson, t *testing.T, object int) {

	const nul = '\000'

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
				{'r', 0x33},
				{'{', 0x32},
				{'"', 0x2},
				{nul, 0x5},
				{'{', 0x31},
				{'"', 0xb},
				{nul, 0x5},
				{'l', 0x0},
				{nul, 0x320},
				{'"', 0x17},
				{nul, 0x6},
				{'l', 0x0},
				{nul, 0x258},
				{'"', 0x24},
				{nul, 0x5},
				{'"', 0x2c},
				{nul, 0x14},
				{'"', 0x43},
				{nul, 0x9},
				{'{', 0x21},
				{'"', 0x50},
				{nul, 0x3},
				{'"', 0x56},
				{nul, 0x26},
				{'"', 0x7f},
				{nul, 0x6},
				{'l', 0x0},
				{nul, 0x7d},
				{'"', 0x8c},
				{nul, 0x5},
				{'l', 0x0},
				{nul, 0x64},
				{'}', 0x13},
				{'"', 0x99},
				{nul, 0x8},
				{'f', 0x0},
				{'"', 0xaa},
				{nul, 0x3},
				{'[', 0x30},
				{'l', 0x0},
				{nul, 0x74},
				{'l', 0x0},
				{nul, 0x3af},
				{'l', 0x0},
				{nul, 0xea},
				{'l', 0x0},
				{nul, 0x9789},
				{']', 0x26},
				{'}', 0x4},
				{'}', 0x1},
				{'r', 0x0},
				//
				// Second object
				{'r', 0x66},
				{'{', 0x65},
				{'"', 0xc7},
				{nul, 0x5},
				{'{', 0x64},
				{'"', 0xd0},
				{nul, 0x5},
				{'l', 0x0},
				{nul, 0x321},
				{'"', 0xdc},
				{nul, 0x6},
				{'l', 0x0},
				{nul, 0x259},
				{'"', 0xe9},
				{nul, 0x5},
				{'"', 0xf1},
				{nul, 0x14},
				{'"', 0x108},
				{nul, 0x9},
				{'{', 0x54},
				{'"', 0x115},
				{nul, 0x3},
				{'"', 0x11b},
				{nul, 0x26},
				{'"', 0x144},
				{nul, 0x6},
				{'l', 0x0},
				{nul, 0x7d},
				{'"', 0x151},
				{nul, 0x5},
				{'l', 0x0},
				{nul, 0x64},
				{'}', 0x46},
				{'"', 0x15e},
				{nul, 0x8},
				{'f', 0x0},
				{'"', 0x16f},
				{nul, 0x3},
				{'[', 0x63},
				{'l', 0x0},
				{nul, 0x74},
				{'l', 0x0},
				{nul, 0x3af},
				{'l', 0x0},
				{nul, 0xea},
				{'l', 0x0},
				{nul, 0x9789},
				{']', 0x59},
				{'}', 0x37},
				{'}', 0x34},
				{'r', 0x33},
				//
				// Third object
				{'r', 0x99},
				{'{', 0x98},
				{'"', 0x18c},
				{nul, 0x5},
				{'{', 0x97},
				{'"', 0x195},
				{nul, 0x5},
				{'l', 0x0},
				{nul, 0x322},
				{'"', 0x1a1},
				{nul, 0x6},
				{'l', 0x0},
				{nul, 0x25a},
				{'"', 0x1ae},
				{nul, 0x5},
				{'"', 0x1b6},
				{nul, 0x14},
				{'"', 0x1cd},
				{nul, 0x9},
				{'{', 0x87},
				{'"', 0x1da},
				{nul, 0x3},
				{'"', 0x1e0},
				{nul, 0x26},
				{'"', 0x209},
				{nul, 0x6},
				{'l', 0x0},
				{nul, 0x7d},
				{'"', 0x216},
				{nul, 0x5},
				{'l', 0x0},
				{nul, 0x64},
				{'}', 0x79},
				{'"', 0x223},
				{nul, 0x8},
				{'f', 0x0},
				{'"', 0x234},
				{nul, 0x3},
				{'[', 0x96},
				{'l', 0x0},
				{nul, 0x74},
				{'l', 0x0},
				{nul, 0x3af},
				{'l', 0x0},
				{nul, 0xea},
				{'l', 0x0},
				{nul, 0x9789},
				{']', 0x8c},
				{'}', 0x6a},
				{'}', 0x67},
				{'r', 0x66},
			},
		},
	}

	tc := testCases[0]

	//	For TestFindNewlineDelimiters, adjust the array that we are testing against
	if object == 1 {
		tc.expected = tc.expected[:51]
	} else if object == 2 || object == 3 {
		tc.expected = tc.expected[:51]

		adjustQoutes := []uint64{2, 5, 9, 13, 15, 17, 20, 22, 24, 28, 33, 36}
		for _, a := range adjustQoutes {
			tc.expected[a].val += 1
		}
		if object == 2 {
			tc.expected[8].val = 801
			tc.expected[12].val = 601
		} else if object == 3 {
			tc.expected[8].val = 802
			tc.expected[12].val = 602
		}
	}

	if len(pj.Tape) != len(tc.expected) {
		t.Errorf("verifyDemoNdjson: got: %d want: %d", len(pj.Tape), len(tc.expected))
	}
	for ii, tp := range pj.Tape {
		//c := "'" + string(byte(tp >> 56)) + "'"
		//if byte(tp >> 56) == 0 {
		//	c = "nul"
		//}
		//fmt.Printf("{%s, 0x%x},\n", c, tp&0xffffffffffffff)
		expected := tc.expected[ii].val | (uint64(tc.expected[ii].c) << 56)
		if tp != expected {
			t.Errorf("verifyDemoNdjson(%d): got: %016x want: %016x", ii, tp, expected)
		}
	}
}

func TestNdjsonCountWhere(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	if testing.Short() {
		t.Skip("skipping... too long")
	}
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")
	pj, err := ParseND(ndjson, nil)
	if err != nil {
		t.Fatal(err)
	}

	const want = 110349
	if result := countWhere("Make", "HOND", *pj); result != want {
		t.Errorf("TestNdjsonCountWhere: got: %d want: %d", result, want)
	}
}

func TestNdjsonCountWhere2(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	if testing.Short() {
		t.Skip("skipping... too long")
	}
	ndjson := loadFile("testdata/RC_2009-01.json.zst")
	// Test trimming
	b := make([]byte, 0, len(ndjson)+4)
	b = append(b, '\n', '\n')
	b = append(b, ndjson...)
	b = append(b, '\n', '\n')
	pj, err := ParseND(ndjson, nil)
	if err != nil {
		t.Fatal(err)
	}
	const want = 170315
	if result := countWhere("subreddit", "reddit.com", *pj); result != want {
		t.Errorf("TestNdjsonCountWhere: got: %d want: %d", result, want)
	}
}

func loadFile(filename string) []byte {
	if !strings.HasSuffix(filename, ".zst") {
		ndjson, err := ioutil.ReadFile(filename)
		if err != nil {
			panic("Failed to load file")
		}
		return ndjson
	}
	var f *os.File
	var err error
	for {
		f, err = os.Open(filename)
		if err == nil {
			defer f.Close()
			break
		}
		if os.IsNotExist(err) {
			fmt.Println("downloading file" + filename)
			resp, err := http.DefaultClient.Get("https://files.klauspost.com/compress/" + filepath.Base(filename))
			if err == nil && resp.StatusCode == http.StatusOK {
				b, err := ioutil.ReadAll(resp.Body)
				if err == nil {
					err = ioutil.WriteFile(filename, b, os.ModePerm)
					if err == nil {
						continue
					}
				}
			}
		}
		panic("Failed to (down)load file:" + err.Error())
	}
	dec, err := zstd.NewReader(f)
	if err != nil {
		panic("Failed to create decompressor")
	}
	defer dec.Close()
	ndjson, err := ioutil.ReadAll(dec)
	if err != nil {
		panic("Failed to load file")
	}
	return ndjson
}

func count_raw_tape(tape []uint64) (count int) {

	for tapeidx := uint64(0); tapeidx < uint64(len(tape)); count++ {
		tape_val := tape[tapeidx]
		tapeidx = tape_val & JSONVALUEMASK
	}

	return
}

func countWhere(key, value string, data ParsedJson) (count int) {
	tmpi := data.Iter()
	stack := []*Iter{&tmpi}
	var obj *Object
	var tmp *Iter
	var elem Element

	for len(stack) > 0 {
		iter := stack[len(stack)-1]
		typ := iter.Advance()

	typeswitch:
		switch typ {
		case TypeNone:
			if len(stack) == 0 {
				return
			}
			stack = stack[:len(stack)-1]
		case TypeRoot:
			var err error
			typ, tmp, err = iter.Root(tmp)
			if err != nil {
				log.Fatal(err)
			}
			switch typ {
			case TypeNone:
				break typeswitch
			case TypeObject:
			default:
				log.Fatalf("expected object inside root, got %v", typ)
			}
			if len(stack) > 2 {
				break
			}
			obj, err = tmp.Object(obj)
			if err != nil {
				log.Fatal(err)
			}
			e := obj.FindKey(key, &elem)
			if e != nil && elem.Type == TypeString {
				v, _ := elem.Iter.StringBytes()
				if string(v) == value {
					count++
				}
			}
		default:
		}
	}

	return
}

func countRawTapeWhere(key, value string, data ParsedJson) (count int) {
	tape := data.Tape
	strbuf := data.Strings
	for tapeidx := uint64(0); tapeidx < uint64(len(tape)); tapeidx++ {
		howmany := uint64(0)
		tape_val := tape[tapeidx]
		ntype := tape_val >> 56

		if ntype == 'r' {
			howmany = tape_val & JSONVALUEMASK
		} else {
			return 0
		}

		// Decrement howmany (since we're adding one now for the ndjson support)
		howmany -= 1

		tapeidx++
		for ; tapeidx < howmany; tapeidx++ {
			tape_val = tape[tapeidx]
			ntype := Tag(tape_val >> 56)
			payload := tape_val & JSONVALUEMASK
			switch ntype {
			case TagString: // we have a string
				string_length := uint64(binary.LittleEndian.Uint32(strbuf[payload : payload+4]))
				if string(strbuf[payload+4:payload+4+string_length]) == key {
					tape_val_next := tape[tapeidx+1]
					ntype_next := Tag(tape_val_next >> 56)
					if ntype_next == TagString {
						payload_next := tape_val_next & JSONVALUEMASK
						string_length_next := uint64(binary.LittleEndian.Uint32(strbuf[payload_next : payload_next+4]))
						if string(strbuf[payload_next+4:payload_next+4+string_length_next]) == value {
							count++
						}
					}
				}

			case TagInteger: // we have a long int
				tapeidx++

			case TagFloat: // we have a double
				tapeidx++

			case TagNull: // we have a null
			case TagBoolTrue: // we have a true
			case TagBoolFalse: // we have a false
			case TagObjectStart: // we have an object
			case TagObjectEnd: // we end an object
			case TagArrayStart: // we start an array
			case TagArrayEnd: // we end an array
			case TagRoot: // we start and end with the root node
				return 0

			default:
				return 0
			}
		}
	}

	return
}

func countObjects(data ParsedJson) (count int) {
	iter := data.Iter()
	for {
		typ := iter.Advance()
		switch typ {
		case TypeNone:
			return
		case TypeRoot:
			count++
		default:
			panic(typ)
		}
	}
}

func BenchmarkNdjsonWarmCountStar(b *testing.B) {
	if !SupportedCPU() {
		b.SkipNow()
	}

	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj, err := ParseND(ndjson, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		countObjects(*pj)
	}
}

func BenchmarkNdjsonWarmCountStarWithWhere(b *testing.B) {
	if !SupportedCPU() {
		b.SkipNow()
	}

	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj, err := ParseND(ndjson, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("iter", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			countWhere("Make", "HOND", *pj)
		}
	})
}
