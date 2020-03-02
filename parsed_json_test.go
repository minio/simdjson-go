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
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

const demo_json = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

type tester interface {
	Fatal(args ...interface{})
}

func loadCompressed(t tester, file string) (ref []byte) {
	dec, err := zstd.NewReader(nil)
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

	return ref
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

func BenchmarkIter_MarshalJSONBuffer(b *testing.B) {
	if !SupportedCPU() {
		b.SkipNow()
	}
	for _, tt := range testCases {
		b.Run(tt.name, func(b *testing.B) {
			ref := loadCompressed(b, tt.name)
			pj, err := Parse(ref, nil)
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
			ref := loadCompressed(b, tt.name)
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
	if !SupportedCPU() {
		t.SkipNow()
	}
	msg := []byte(demo_json)
	expected := `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

	pj, err := Parse(msg, nil)

	if err != nil {
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
