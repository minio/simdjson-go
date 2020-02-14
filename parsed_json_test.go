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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

type tester interface {
	Fatal(args ...interface{})
}

func loadCompressed(t tester, file string) (tape, sb, ref []byte) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	tap, err := ioutil.ReadFile(filepath.Join("testdata", file+".tape.zst"))
	if err != nil {
		t.Fatal(err)
	}
	tap, err = dec.DecodeAll(tap, nil)
	// Our end-of-root has been incremented by one (past last element) for quick skipping of ndjson
	// So correct the initial root element to point to one position higher
	binary.LittleEndian.PutUint64(tap, binary.LittleEndian.Uint64(tap)+1)
	if err != nil {
		t.Fatal(err)
	}
	sb, err = ioutil.ReadFile(filepath.Join("testdata", file+".stringbuf.zst"))
	if err != nil {
		t.Fatal(err)
	}
	sb, err = dec.DecodeAll(sb, nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err = ioutil.ReadFile(filepath.Join("testdata", file+".json.zst"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err = dec.DecodeAll(ref, nil)
	if err != nil {
		t.Fatal(err)
	}

	return tap, sb, ref
}

var testCases = []struct {
	name  string
	array bool
}{
	{
		name: "apache_builds",
	},
	{
		name: "canada",
	},
	{
		name: "citm_catalog",
	},
	{
		name:  "github_events",
		array: true,
	},
	{
		name: "gsoc-2018",
	},
	{
		name: "instruments",
	},
	{
		name:  "numbers",
		array: true,
	},
	{
		name: "marine_ik",
	},
	{
		name: "mesh",
	},
	{
		name: "mesh.pretty",
	},
	{
		name: "twitterescaped",
	},
	{
		name: "twitter",
	},
	{
		name: "random",
	},
	{
		name: "update-center",
	},
}

func bytesToUint64(buf []byte) []uint64 {

	tape := make([]uint64, len(buf)/8)
	for i := range tape {
		tape[i] = binary.LittleEndian.Uint64(buf[i*8:])
	}
	return tape
}

func testCTapeCtoGoTapeCompare(t *testing.T, ctape []uint64, csbuf []byte, pj internalParsedJson) {

	gotape := pj.Tape

	cindex, goindex := 0, 0
	for goindex < len(gotape) {
		if cindex == len(ctape) {
			t.Errorf("TestCTapeCtoGoTapeCompare: unexpected, ctape at end, but gotape not yet")
			break
		}
		cval, goval := ctape[cindex], gotape[goindex]

		// Make sure the type is the same between the C and Go version
		if cval>>56 != goval>>56 {
			t.Errorf("TestCTapeCtoGoTapeCompare: got: %02x want: %02x", goval>>56, cval>>56)
		}

		ntype := Tag(goval >> 56)
		switch ntype {
		case TagRoot, TagObjectStart, TagObjectEnd, TagArrayStart, TagArrayEnd:
			cindex++
			goindex++

		case TagString:
			cpayload := cval & JSONVALUEMASK
			cstrlen := binary.LittleEndian.Uint32(csbuf[cpayload : cpayload+4])
			cstr := string(csbuf[cpayload+4 : cpayload+4+uint64(cstrlen)])
			gostr, _ := pj.stringAt(goval&JSONVALUEMASK, gotape[goindex+1])
			if cstr != gostr {
				t.Errorf("TestCTapeCtoGoTapeCompare: got: %s want: %s", gostr, cstr)
			}
			cindex++
			goindex += 2

		case TagNull, TagBoolTrue, TagBoolFalse:
			cindex++
			goindex++

		case TagInteger, TagFloat:
			if ctape[cindex+1] != gotape[goindex+1] {
				if !(ntype == TagFloat && GOLANG_NUMBER_PARSING) {
					t.Errorf("TestCTapeCtoGoTapeCompare: got: %016x want: %016x", gotape[goindex+1], ctape[cindex+1])

				}
			}
			cindex += 2
			goindex += 2

		default:
			t.Errorf("TestCTapeCtoGoTapeCompare: unexpected token, got: %02x", ntype)
		}
	}

	if cindex != len(ctape) {
		t.Errorf("TestCTapeCtoGoTapeCompare: got: %d want: %d", cindex, len(ctape))
	}
}

func TestVerifyTape(t *testing.T) {

	for _, tt := range testCases {

		t.Run(tt.name, func(t *testing.T) {
			cbuf, csbuf, ref := loadCompressed(t, tt.name)

			pj := internalParsedJson{}
			if err := pj.parseMessage(ref); err != nil {
				t.Errorf("parseMessage failed: %v\n", err)
				return
			}

			ctape := bytesToUint64(cbuf)

			testCTapeCtoGoTapeCompare(t, ctape, csbuf, pj)
		})
	}
}

func BenchmarkIter_MarshalJSONBuffer(b *testing.B) {
	for _, tt := range testCases {
		b.Run(tt.name, func(b *testing.B) {
			tap, sb, _ := loadCompressed(b, tt.name)

			pj, err := loadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				b.Fatal(err)
			}
			iter := pj.Iter()
			cpy := iter
			output, err := cpy.MarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(output)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cpy := iter
				output, err = cpy.MarshalJSONBuffer(output[:0])
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGoMarshalJSON(b *testing.B) {
	for _, tt := range testCases {
		b.Run(tt.name, func(b *testing.B) {
			_, _, ref := loadCompressed(b, tt.name)
			var m interface{}
			m = map[string]interface{}{}
			if tt.array {
				m = []interface{}{}
			}
			err := json.Unmarshal(ref, &m)
			if err != nil {
				b.Fatal(err)
			}
			output, err := json.Marshal(m)
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(output)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output, err = json.Marshal(m)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestPrintJson(t *testing.T) {

	msg := []byte(demo_json)
	expected := `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

	pj := internalParsedJson{}

	if err := pj.parseMessage(msg); err != nil {
		t.Errorf("parseMessage failed\n")
	}

	iter := pj.Iter()
	out, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != expected {
		t.Errorf("TestPrintJson: got: %s want: %s", out, expected)
	}
}
