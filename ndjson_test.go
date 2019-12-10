package simdjson

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

const demo_ndjson = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":801,"Height":601,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}
{"Image":{"Width":802,"Height":602,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

func verifyDemoNdjson(pj internalParsedJson, t *testing.T) {

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

	tc := testCases[0]

	if len(pj.Tape) != len(tc.expected) {
		t.Errorf("verifyDemoNdjson: got: %d want: %d", len(pj.Tape), len(tc.expected))
	}
	for ii, tp := range pj.Tape {
		// fmt.Printf("{'%s', 0x%x},\n", string(byte((tp >> 56))), tp&0xffffffffffffff)
		expected := tc.expected[ii].val | (uint64(tc.expected[ii].c) << 56)
		if tp != expected {
			t.Errorf("verifyDemoNdjson(%d): got: %d want: %d", ii, tp, expected)
		}
	}
}

func TestDemoNdjson(t *testing.T) {

	pj := internalParsedJson{}
	pj.initialize(len(demo_ndjson))

	if err := pj.parseMessageNdjson([]byte(demo_ndjson)); err != nil {
		t.Errorf("TestDemoNdjson: got: %v want: nil", err)
	}

	verifyDemoNdjson(pj, t)
}

func TestNdjsonCountWhere(t *testing.T) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessageNdjson(ndjson)

	const want = 110349
	if result := countWhere("Make", "HOND", pj.ParsedJson); result != want {
		t.Errorf("TestNdjsonCountWhere: got: %d want: %d", result, want)
	}
}

func TestNdjsonCountWhere2(t *testing.T) {
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

func BenchmarkNdjsonStage1(b *testing.B) {

	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

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
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")
	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pj.initialize(len(ndjson) * 3 / 2)
		pj.parseMessage(ndjson)
	}
}

func count_raw_tape(tape []uint64) (count int) {

	for tapeidx := uint64(0); tapeidx < uint64(len(tape)); count++ {
		tape_val := tape[tapeidx]
		tapeidx = tape_val & JSONVALUEMASK
	}

	return
}

func BenchmarkNdjsonColdCountStar(b *testing.B) {

	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	// Allocate stuff
	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pj.initialize(len(ndjson) * 3 / 2)
		pj.parseMessage(ndjson)
		count_raw_tape(pj.Tape)
	}
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

func BenchmarkNdjsonColdCountStarWithWhere(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")
	const want = 110349
	runtime.GC()
	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)

	b.Run("raw", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pj.initialize(len(ndjson) * 3 / 2)
			err := pj.parseMessage(ndjson)
			if err != nil {
				b.Fatal(err)
			}
			got := countRawTapeWhere("Make", "HOND", pj.ParsedJson)
			if got != want {
				b.Fatal(got, "!=", want)
			}
		}
	})
	b.Run("iter", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			pj.initialize(len(ndjson) * 3 / 2)
			err := pj.parseMessage(ndjson)
			if err != nil {
				b.Fatal(err)
			}
			got := countWhere("Make", "HOND", pj.ParsedJson)
			if got != want {
				b.Fatal(got, "!=", want)
			}
		}
	})
}

func BenchmarkNdjsonWarmCountStar(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		countObjects(pj.ParsedJson)
	}
}

func BenchmarkNdjsonWarmCountStarWithWhere(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	b.Run("raw", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			countRawTapeWhere("Make", "HOND", pj.ParsedJson)
		}
	})
	b.Run("iter", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			countWhere("Make", "HOND", pj.ParsedJson)
		}
	})

}
