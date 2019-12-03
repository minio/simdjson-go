package simdjson

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"os"
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

func TestLoadTape(t *testing.T) {
	//TODO: Re-enable tests
	t.SkipNow()

	for _, tt := range testCases {

		t.Run(tt.name, func(t *testing.T) {
			tap, sb, ref := loadCompressed(t, tt.name)

			var tmp interface{} = map[string]interface{}{}
			if tt.array {
				tmp = []interface{}{}
			}
			var refJSON []byte
			err := json.Unmarshal(ref, &tmp)
			if err != nil {
				t.Fatal(err)
			}
			refJSON, err = json.MarshalIndent(tmp, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			pj, err := loadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				t.Fatal(err)
			}
			i := pj.Iter()
			cpy := i
			b, err := cpy.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if false {
				t.Log(string(b))
			}
			//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".json"), b, os.ModePerm)

			for {
				var next Iter
				typ, err := i.AdvanceIter(&next)
				if err != nil {
					t.Fatal(err)
				}
				switch typ {
				case TypeNone:
					return
				case TypeRoot:
					i = next
				case TypeArray:
					arr, err := next.Array(nil)
					if err != nil {
						t.Fatal(err)
					}
					got, err := arr.Interface()
					if err != nil {
						t.Fatal(err)
					}
					b, err := json.MarshalIndent(got, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(b, refJSON) {
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".want"), refJSON, os.ModePerm)
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".got"), b, os.ModePerm)
						t.Error("Content mismatch. Output dumped to testdata.")
					}

				case TypeObject:
					obj, err := next.Object(nil)
					if err != nil {
						t.Fatal(err)
					}
					got, err := obj.Map(nil)
					if err != nil {
						t.Fatal(err)
					}
					b, err := json.MarshalIndent(got, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(b, refJSON) {
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".want"), refJSON, os.ModePerm)
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".got"), b, os.ModePerm)
						t.Error("Content mismatch. Output dumped to testdata.")
					}
				}
			}
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
	pj.initialize(len(msg) * 2)

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
