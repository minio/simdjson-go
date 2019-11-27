package simdjson

import (
	"bytes"
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
	if !strings.HasSuffix(filename, ".zst") {
		ndjson, err := ioutil.ReadFile(filename)
		if err != nil {
			panic("Failed to load file")
		}
		return bytes.ReplaceAll(ndjson, []byte("\n"), []byte("{"))
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
	return bytes.ReplaceAll(ndjson, []byte("\n"), []byte("{"))

}

func BenchmarkNdjsonStage1(b *testing.B) {

	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

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
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")
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

func BenchmarkNdjsonStage2CountStar(b *testing.B) {

	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

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

		switch typ {
		case TypeNone:
			if len(stack) == 0 {
				return
			}
			stack = stack[:len(stack)-1]
		case TypeRoot:
			var err error
			tmp, err = iter.Root(tmp)
			if err != nil {
				log.Fatal(err)
			}
			stack = append(stack, tmp)
		case TypeObject:
			if len(stack) > 2 {
				break
			}
			var err error
			obj, err = iter.Object(obj)
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

func BenchmarkNdjsonStage2CountStarWithWhere(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")
	const want = 110349
	runtime.GC()
	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)

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
}

func BenchmarkNdjsonCountStarWarm(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

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

func BenchmarkNdjsonCountStarWithWhereWarm(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	b.Run("iter", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			countWhere("Make", "HOND", pj.ParsedJson)
		}
	})
	b.Run("raw", func(b *testing.B) {
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			countRawTapeWhere("Make", "HOND", pj.ParsedJson)
		}
	})

}

func BenchmarkNdjsonIterWarm(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")
	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		it := pj.Iter()
		count := 0
		for it.Advance() != TypeNone {
			count++
		}
	}
}

func TestNdjsonIterWhere(t *testing.T) {
	const carmake = "HOND"
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	t.Log(countWhere("Make", carmake, pj.ParsedJson))
}

func BenchmarkNdjsonIterWhereWarm(b *testing.B) {

	const carmake = "HOND"

	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	b.SetBytes(int64(len(ndjson)))
	b.ReportAllocs()
	b.ResetTimer()

	count := 0
	for i := 0; i < b.N; i++ {
		it := pj.Iter()
		count = 0
		var obj *Iter
		var object *Object
		for it.Advance() != TypeNone {
			if it.Type() != TypeRoot {
				panic("not root:" + it.Type().String())
			}
			var err error
			obj, err = it.Root(obj)
			if err != nil {
				b.Fatal(err)
			}
			if obj.Advance() != TypeObject {
				if obj.Type() == TypeNone {
					// Last record is an empty root element.
					break
				}
				panic("not object:" + obj.Type().String())
			}
			object, err = obj.Object(object)
			if err != nil {
				b.Fatal(err)
			}
			var tmp Iter
			for {
				name, t, err := object.NextElementBytes(&tmp)
				if err != nil {
					return
				}
				if t == TypeNone {
					// Done
					break
				}
				if string(name) == "Make" {
					val, err := tmp.StringBytes()
					if err != nil {
						return
					}
					if string(val) == carmake {
						count++
					}
					break
				}
			}
		}
	}
	b.Log(count)
}
