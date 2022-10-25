//go:build go1.18
// +build go1.18

/*
 * MinIO Cloud Storage, (C) 2022 MinIO, Inc.
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
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

func FuzzParse(f *testing.F) {
	if !SupportedCPU() {
		f.SkipNow()
	}
	addBytesFromTarZst(f, "testdata/fuzz/corpus.tar.zst", testing.Short())
	addBytesFromTarZst(f, "testdata/fuzz/go-corpus.tar.zst", testing.Short())
	f.Fuzz(func(t *testing.T, data []byte) {
		var dst map[string]interface{}
		var dstA []interface{}
		pj, err := Parse(data, nil)
		jErr := json.Unmarshal(data, &dst)
		if err != nil {
			if jErr == nil && dst != nil {
				t.Logf("got error %v, but json.Unmarshal could unmarshal", err)
			}
			// Don't continue
			t.Skip()
			return
		}
		if jErr != nil {
			if strings.Contains(jErr.Error(), "cannot unmarshal array into") {
				jErr2 := json.Unmarshal(data, &dstA)
				if jErr2 != nil {
					t.Logf("no error reported, but json.Unmarshal (Array) reported: %v", jErr2)
				}
			} else {
				t.Logf("no error reported, but json.Unmarshal reported: %v", jErr)
			}
		}
		// Check if we can convert back
		i := pj.Iter()
		if i.PeekNextTag() != TagEnd {
			_, err = i.MarshalJSON()
			if err != nil {
				switch {
				// This is ok.
				case strings.Contains(err.Error(), "INF or NaN number found"):
				default:
					t.Error(err)
				}
			}
		}
		// Do simple ND test.
		d2 := append(make([]byte, 0, len(data)*3+2), data...)
		d2 = append(d2, '\n')
		d2 = append(d2, data...)
		d2 = append(d2, '\n')
		d2 = append(d2, data...)
		_, _ = ParseND(data, nil)
		return
	})
}

// FuzzCorrect will check for correctness and compare output to stdlib.
func FuzzCorrect(f *testing.F) {
	if !SupportedCPU() {
		f.SkipNow()
	}
	const (
		// fail if simdjson doesn't report error, but json.Unmarshal does
		failOnMissingError = true
		// Run input through json.Unmarshal/json.Marshal first
		filterRaw = true
	)
	addBytesFromTarZst(f, "testdata/fuzz/corpus.tar.zst", testing.Short())
	addBytesFromTarZst(f, "testdata/fuzz/go-corpus.tar.zst", testing.Short())
	f.Fuzz(func(t *testing.T, data []byte) {
		var want map[string]interface{}
		var wantA []interface{}
		if !utf8.Valid(data) {
			t.SkipNow()
		}
		if filterRaw {
			var tmp interface{}
			err := json.Unmarshal(data, &tmp)
			if err != nil {
				t.SkipNow()
			}
			data, err = json.Marshal(tmp)
			if err != nil {
				t.Fatal(err)
			}
			if tmp == nil {
				t.SkipNow()
			}
		}
		pj, err := Parse(data, nil)
		jErr := json.Unmarshal(data, &want)
		if err != nil {
			if jErr == nil {
				b, _ := json.Marshal(want)
				t.Fatalf("got error %v, but json.Unmarshal could unmarshal to %#v js: %s", err, want, string(b))
			}
			// Don't continue
			t.SkipNow()
		}
		if jErr != nil {
			want = nil
			if strings.Contains(jErr.Error(), "cannot unmarshal array into") {
				jErr2 := json.Unmarshal(data, &wantA)
				if jErr2 != nil {
					if failOnMissingError {
						t.Fatalf("no error reported, but json.Unmarshal (Array) reported: %v", jErr2)
					}
				}
			} else {
				if failOnMissingError {
					t.Fatalf("no error reported, but json.Unmarshal reported: %v", jErr)
				}
				return
			}
		}
		// Check if we can convert back
		var got map[string]interface{}
		var gotA []interface{}

		i := pj.Iter()
		if i.PeekNextTag() == TagEnd {
			if len(want)+len(wantA) > 0 {
				msg := fmt.Sprintf("stdlib returned data %#v, but nothing from simdjson (tap:%d, str:%d, err:%v)", want, len(pj.Tape), len(pj.Strings.B), err)
				panic(msg)
			}
			t.SkipNow()
		}

		data, err = i.MarshalJSON()
		if err != nil {
			switch {
			// This is ok.
			case strings.Contains(err.Error(), "INF or NaN number found"):
			default:
				panic(err)
			}
		}
		var wantB []byte
		var gotB []byte
		if want != nil {
			// We should be able to unmarshal into msi
			i := pj.Iter()
			i.AdvanceInto()
			for i.Type() != TypeNone {
				switch i.Type() {
				case TypeRoot:
					i.Advance()
				case TypeObject:
					obj, err := i.Object(nil)
					if err != nil {
						panic(err)
					}
					got, err = obj.Map(got)
					if err != nil {
						panic(err)
					}
					i.Advance()
				default:
					allOfit := pj.Iter()
					msg, _ := allOfit.MarshalJSON()
					t.Fatalf("Unexpected type: %v, all: %s", i.Type(), string(msg))
				}
			}
			gotB, err = json.Marshal(got)
			if err != nil {
				panic(err)
			}
			wantB, err = json.Marshal(want)
			if err != nil {
				panic(err)
			}
		}
		if wantA != nil {
			// We should be able to unmarshal into msi
			i := pj.Iter()
			i.AdvanceInto()
			for i.Type() != TypeNone {
				switch i.Type() {
				case TypeRoot:
					i.Advance()
				case TypeArray:
					arr, err := i.Array(nil)
					if err != nil {
						panic(err)
					}
					gotA, err = arr.Interface()
					if err != nil {
						panic(err)
					}
					i.Advance()
				default:
					t.Fatalf("Unexpected type: %v", i.Type())
				}
			}
			gotB, err = json.Marshal(gotA)
			if err != nil {
				panic(err)
			}
			wantB, err = json.Marshal(wantA)
			if err != nil {
				panic(err)
			}
		}
		if !bytes.Equal(gotB, wantB) {
			if len(want)+len(got) == 0 {
				t.SkipNow()
			}
			if bytes.Equal(bytes.ReplaceAll(wantB, []byte("-0"), []byte("0")), bytes.ReplaceAll(gotB, []byte("-0"), []byte("0"))) {
				// let -0 == 0
				return
			}
			allOfit := pj.Iter()
			simdOut, _ := allOfit.MarshalJSON()

			t.Fatalf("Marshal data mismatch:\nstdlib: %v\nsimdjson:%v\n\nsimdjson:%s", string(wantB), string(gotB), string(simdOut))
		}

		return
	})
}

// FuzzCorrect will check for correctness and compare output to stdlib.
func FuzzSerialize(f *testing.F) {
	if !SupportedCPU() {
		f.SkipNow()
	}
	addBytesFromTarZst(f, "testdata/fuzz/corpus.tar.zst", testing.Short())
	addBytesFromTarZst(f, "testdata/fuzz/go-corpus.tar.zst", testing.Short())
	f.Fuzz(func(t *testing.T, data []byte) {
		// Create a tape from the input and ensure that the output of JSON matches.
		pj, err := Parse(data, nil)
		if err != nil {
			pj, err = ParseND(data, pj)
			if err != nil {
				// Don't continue
				t.SkipNow()
			}
		}
		i := pj.Iter()
		want, err := i.MarshalJSON()
		if err != nil {
			panic(err)
		}
		// Check if we can convert back
		s := NewSerializer()
		got := make([]byte, 0, len(want))
		var dst []byte
		var target *ParsedJson
		for _, comp := range []CompressMode{CompressNone, CompressFast, CompressDefault, CompressBest} {
			level := fmt.Sprintf("level-%d:", comp)
			s.CompressMode(comp)
			dst = s.Serialize(dst[:0], *pj)
			target, err = s.Deserialize(dst, target)
			if err != nil {
				t.Error(level + err.Error())
			}
			i := target.Iter()
			got, err = i.MarshalJSONBuffer(got[:0])
			if err != nil {
				t.Error(level + err.Error())
			}
			if !bytes.Equal(want, got) {
				err := fmt.Sprintf("%s JSON mismatch:\nwant: %s\ngot :%s", level, string(want), string(got))
				err += fmt.Sprintf("\ntap0:%x", pj.Tape)
				err += fmt.Sprintf("\ntap1:%x", target.Tape)
				t.Error(err)
			}
		}
		return
	})
}
func addBytesFromTarZst(f *testing.F, filename string, short bool) {
	file, err := os.Open(filename)
	if err != nil {
		f.Fatal(err)
	}
	defer file.Close()
	zr, err := zstd.NewReader(file)
	if err != nil {
		f.Fatal(err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	i := 0
	for h, err := tr.Next(); err == nil; h, err = tr.Next() {
		i++
		if short && i%100 != 0 {
			continue
		}
		b := make([]byte, h.Size)
		_, err := io.ReadFull(tr, b)
		if err != nil {
			f.Fatal(err)
		}
		raw := true
		if bytes.HasPrefix(b, []byte("go test fuzz")) {
			raw = false
		}
		if raw {
			f.Add(b)
			continue
		}
		vals, err := unmarshalCorpusFile(b)
		if err != nil {
			f.Fatal(err)
		}
		for _, v := range vals {
			f.Add(v)
		}
	}
}

// unmarshalCorpusFile decodes corpus bytes into their respective values.
func unmarshalCorpusFile(b []byte) ([][]byte, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("cannot unmarshal empty string")
	}
	lines := bytes.Split(b, []byte("\n"))
	if len(lines) < 2 {
		return nil, fmt.Errorf("must include version and at least one value")
	}
	var vals = make([][]byte, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		v, err := parseCorpusValue(line)
		if err != nil {
			return nil, fmt.Errorf("malformed line %q: %v", line, err)
		}
		vals = append(vals, v)
	}
	return vals, nil
}

// parseCorpusValue
func parseCorpusValue(line []byte) ([]byte, error) {
	fs := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fs, "(test)", line, 0)
	if err != nil {
		return nil, err
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, fmt.Errorf("expected call expression")
	}
	if len(call.Args) != 1 {
		return nil, fmt.Errorf("expected call expression with 1 argument; got %d", len(call.Args))
	}
	arg := call.Args[0]

	if arrayType, ok := call.Fun.(*ast.ArrayType); ok {
		if arrayType.Len != nil {
			return nil, fmt.Errorf("expected []byte or primitive type")
		}
		elt, ok := arrayType.Elt.(*ast.Ident)
		if !ok || elt.Name != "byte" {
			return nil, fmt.Errorf("expected []byte")
		}
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return nil, fmt.Errorf("string literal required for type []byte")
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return nil, err
		}
		return []byte(s), nil
	}
	return nil, fmt.Errorf("expected []byte")
}
