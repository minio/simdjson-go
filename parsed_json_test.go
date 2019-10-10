package simdjson

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

type tester interface {
	Fatal(args ...interface{})
}

func loadCompressed(t tester, file string) []byte {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	tap, err := ioutil.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	tap, err = dec.DecodeAll(tap, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tap
}

var testCases = []struct {
	ref, tape, stringbuf string
}{
	{
		ref:       "apache_builds.json.zst",
		tape:      "apache_builds.tape.zst",
		stringbuf: "apache_builds.stringbuf.zst",
	},
	{
		ref:       "citm_catalog.json.zst",
		tape:      "citm_catalog.tape.zst",
		stringbuf: "citm_catalog.stringbuf.zst",
	},
	{
		ref:       "github_events.json.zst",
		tape:      "github_events.tape.zst",
		stringbuf: "github_events.stringbuf.zst",
	},
	{
		ref:       "gsoc-2018.json.zst",
		tape:      "gsoc-2018.tape.zst",
		stringbuf: "gsoc-2018.stringbuf.zst",
	},
	{
		ref:       "instruments.json.zst",
		tape:      "instruments.tape.zst",
		stringbuf: "instruments.stringbuf.zst",
	},
	{
		ref:       "numbers.json.zst",
		tape:      "numbers.tape.zst",
		stringbuf: "numbers.stringbuf.zst",
	},
	{
		ref:       "random.json.zst",
		tape:      "random.tape.zst",
		stringbuf: "random.stringbuf.zst",
	},
	{
		ref:       "update-center.json.zst",
		tape:      "update-center.tape.zst",
		stringbuf: "update-center.stringbuf.zst",
	},
}

func TestLoadTape(t *testing.T) {
	for _, tt := range testCases {

		t.Run(tt.ref, func(t *testing.T) {
			tap := loadCompressed(t, tt.tape)
			sb := loadCompressed(t, tt.stringbuf)
			ref := loadCompressed(t, tt.ref)

			var refMap map[string]interface{}
			var refJSON []byte
			err := json.Unmarshal(ref, &refMap)
			if err == nil {
				refJSON, err = json.MarshalIndent(refMap, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
			} else {
				// Probably an array.
				var refArray []interface{}
				err := json.Unmarshal(ref, &refArray)
				if err != nil {
					t.Fatal(err)
				}
				refJSON, err = json.MarshalIndent(refArray, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
			}
			pj, err := LoadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				t.Fatal(err)
			}
			i := pj.Iter()
			cpy := i
			b, err := cpy.MarshalJSON()
			t.Log(string(b), err)
			_ = ioutil.WriteFile(filepath.Join("testdata", tt.ref+".json"), b, os.ModePerm)

			for {
				var next Iter
				typ, err := i.NextIter(&next)
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
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.ref+".want"), refJSON, os.ModePerm)
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.ref+".got"), b, os.ModePerm)
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
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.ref+".want"), refJSON, os.ModePerm)
						_ = ioutil.WriteFile(filepath.Join("testdata", tt.ref+".got"), b, os.ModePerm)
						t.Error("Content mismatch. Output dumped to testdata.")
					}
				}
			}
		})
	}
}

func BenchmarkIter_MarshalJSONBuffer(b *testing.B) {
	for _, tt := range testCases {
		b.Run(tt.ref, func(b *testing.B) {
			tap := loadCompressed(b, tt.tape)
			sb := loadCompressed(b, tt.stringbuf)

			pj, err := LoadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
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
		b.Run(tt.ref, func(b *testing.B) {
			ref := loadCompressed(b, tt.ref)
			var m interface{}
			m = map[string]interface{}{}
			err := json.Unmarshal(ref, &m)
			if err != nil {
				m = []interface{}{}
				err := json.Unmarshal(ref, &m)
				if err != nil {
					b.Fatal(err)
				}
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

	find_structural_indices(msg, &pj)
	success := unified_machine(msg, &pj)
	if !success {
		t.Errorf("Stage2 failed\n")
	}

	iter := pj.Iter()
	out, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	// back to normal state
	if string(out) != expected {
		t.Errorf("TestPrintJson: got: %s want: %s", out, expected)
	}
}

func TestDumpRawDemoJson(t *testing.T) {

	expected := string(dump2hex(`
00000000  30 20 3a 20 72 09 2f 2f  20 70 6f 69 6e 74 69 6e  |0 : r.// pointin|
00000010  67 20 74 6f 20 33 38 20  28 72 69 67 68 74 20 61  |g to 38 (right a|
00000020  66 74 65 72 20 6c 61 73  74 20 6e 6f 64 65 29 0a  |fter last node).|
00000030  31 20 3a 20 7b 09 2f 2f  20 70 6f 69 6e 74 69 6e  |1 : {.// pointin|
00000040  67 20 74 6f 20 6e 65 78  74 20 74 61 70 65 20 6c  |g to next tape l|
00000050  6f 63 61 74 69 6f 6e 20  33 38 20 28 66 69 72 73  |ocation 38 (firs|
00000060  74 20 6e 6f 64 65 20 61  66 74 65 72 20 74 68 65  |t node after the|
00000070  20 73 63 6f 70 65 29 20  0a 32 20 3a 20 73 74 72  | scope) .2 : str|
00000080  69 6e 67 20 22 49 6d 61  67 65 22 0a 33 20 3a 20  |ing "Image".3 : |
00000090  7b 09 2f 2f 20 70 6f 69  6e 74 69 6e 67 20 74 6f  |{.// pointing to|
000000a0  20 6e 65 78 74 20 74 61  70 65 20 6c 6f 63 61 74  | next tape locat|
000000b0  69 6f 6e 20 33 37 20 28  66 69 72 73 74 20 6e 6f  |ion 37 (first no|
000000c0  64 65 20 61 66 74 65 72  20 74 68 65 20 73 63 6f  |de after the sco|
000000d0  70 65 29 20 0a 34 20 3a  20 73 74 72 69 6e 67 20  |pe) .4 : string |
000000e0  22 57 69 64 74 68 22 0a  35 20 3a 20 69 6e 74 65  |"Width".5 : inte|
000000f0  67 65 72 20 38 30 30 0a  37 20 3a 20 73 74 72 69  |ger 800.7 : stri|
00000100  6e 67 20 22 48 65 69 67  68 74 22 0a 38 20 3a 20  |ng "Height".8 : |
00000110  69 6e 74 65 67 65 72 20  36 30 30 0a 31 30 20 3a  |integer 600.10 :|
00000120  20 73 74 72 69 6e 67 20  22 54 69 74 6c 65 22 0a  | string "Title".|
00000130  31 31 20 3a 20 73 74 72  69 6e 67 20 22 56 69 65  |11 : string "Vie|
00000140  77 20 66 72 6f 6d 20 31  35 74 68 20 46 6c 6f 6f  |w from 15th Floo|
00000150  72 22 0a 31 32 20 3a 20  73 74 72 69 6e 67 20 22  |r".12 : string "|
00000160  54 68 75 6d 62 6e 61 69  6c 22 0a 31 33 20 3a 20  |Thumbnail".13 : |
00000170  7b 09 2f 2f 20 70 6f 69  6e 74 69 6e 67 20 74 6f  |{.// pointing to|
00000180  20 6e 65 78 74 20 74 61  70 65 20 6c 6f 63 61 74  | next tape locat|
00000190  69 6f 6e 20 32 33 20 28  66 69 72 73 74 20 6e 6f  |ion 23 (first no|
000001a0  64 65 20 61 66 74 65 72  20 74 68 65 20 73 63 6f  |de after the sco|
000001b0  70 65 29 20 0a 31 34 20  3a 20 73 74 72 69 6e 67  |pe) .14 : string|
000001c0  20 22 55 72 6c 22 0a 31  35 20 3a 20 73 74 72 69  | "Url".15 : stri|
000001d0  6e 67 20 22 68 74 74 70  3a 2f 2f 77 77 77 2e 65  |ng "http://www.e|
000001e0  78 61 6d 70 6c 65 2e 63  6f 6d 2f 69 6d 61 67 65  |xample.com/image|
000001f0  2f 34 38 31 39 38 39 39  34 33 22 0a 31 36 20 3a  |/481989943".16 :|
00000200  20 73 74 72 69 6e 67 20  22 48 65 69 67 68 74 22  | string "Height"|
00000210  0a 31 37 20 3a 20 69 6e  74 65 67 65 72 20 31 32  |.17 : integer 12|
00000220  35 0a 31 39 20 3a 20 73  74 72 69 6e 67 20 22 57  |5.19 : string "W|
00000230  69 64 74 68 22 0a 32 30  20 3a 20 69 6e 74 65 67  |idth".20 : integ|
00000240  65 72 20 31 30 30 0a 32  32 20 3a 20 7d 09 2f 2f  |er 100.22 : }.//|
00000250  20 70 6f 69 6e 74 69 6e  67 20 74 6f 20 70 72 65  | pointing to pre|
00000260  76 69 6f 75 73 20 74 61  70 65 20 6c 6f 63 61 74  |vious tape locat|
00000270  69 6f 6e 20 31 33 20 28  73 74 61 72 74 20 6f 66  |ion 13 (start of|
00000280  20 74 68 65 20 73 63 6f  70 65 29 20 0a 32 33 20  | the scope) .23 |
00000290  3a 20 73 74 72 69 6e 67  20 22 41 6e 69 6d 61 74  |: string "Animat|
000002a0  65 64 22 0a 32 34 20 3a  20 66 61 6c 73 65 0a 32  |ed".24 : false.2|
000002b0  35 20 3a 20 73 74 72 69  6e 67 20 22 49 44 73 22  |5 : string "IDs"|
000002c0  0a 32 36 20 3a 20 09 2f  2f 20 70 6f 69 6e 74 69  |.26 : .// pointi|
000002d0  6e 67 20 74 6f 20 6e 65  78 74 20 74 61 70 65 20  |ng to next tape |
000002e0  6c 6f 63 61 74 69 6f 6e  20 33 36 20 28 66 69 72  |location 36 (fir|
000002f0  73 74 20 6e 6f 64 65 20  61 66 74 65 72 20 74 68  |st node after th|
00000300  65 20 73 63 6f 70 65 29  20 0a 32 37 20 3a 20 69  |e scope) .27 : i|
00000310  6e 74 65 67 65 72 20 31  31 36 0a 32 39 20 3a 20  |nteger 116.29 : |
00000320  69 6e 74 65 67 65 72 20  39 34 33 0a 33 31 20 3a  |integer 943.31 :|
00000330  20 69 6e 74 65 67 65 72  20 32 33 34 0a 33 33 20  | integer 234.33 |
00000340  3a 20 69 6e 74 65 67 65  72 20 33 38 37 39 33 0a  |: integer 38793.|
00000350  33 35 20 3a 20 5d 09 2f  2f 20 70 6f 69 6e 74 69  |35 : ].// pointi|
00000360  6e 67 20 74 6f 20 70 72  65 76 69 6f 75 73 20 74  |ng to previous t|
00000370  61 70 65 20 6c 6f 63 61  74 69 6f 6e 20 32 36 20  |ape location 26 |
00000380  28 73 74 61 72 74 20 6f  66 20 74 68 65 20 73 63  |(start of the sc|
00000390  6f 70 65 29 20 0a 33 36  20 3a 20 7d 09 2f 2f 20  |ope) .36 : }.// |
000003a0  70 6f 69 6e 74 69 6e 67  20 74 6f 20 70 72 65 76  |pointing to prev|
000003b0  69 6f 75 73 20 74 61 70  65 20 6c 6f 63 61 74 69  |ious tape locati|
000003c0  6f 6e 20 33 20 28 73 74  61 72 74 20 6f 66 20 74  |on 3 (start of t|
000003d0  68 65 20 73 63 6f 70 65  29 20 0a 33 37 20 3a 20  |he scope) .37 : |
000003e0  7d 09 2f 2f 20 70 6f 69  6e 74 69 6e 67 20 74 6f  |}.// pointing to|
000003f0  20 70 72 65 76 69 6f 75  73 20 74 61 70 65 20 6c  | previous tape l|
00000400  6f 63 61 74 69 6f 6e 20  31 20 28 73 74 61 72 74  |ocation 1 (start|
00000410  20 6f 66 20 74 68 65 20  73 63 6f 70 65 29 20 0a  | of the scope) .|
00000420  33 38 20 3a 20 72 09 2f  2f 20 70 6f 69 6e 74 69  |38 : r.// pointi|
00000430  6e 67 20 74 6f 20 30 20  28 73 74 61 72 74 20 72  |ng to 0 (start r|
00000440  6f 6f 74 29 0a                                    |oot).|
00000445
`))

	pj := internalParsedJson{}
	pj.initialize(1024)

	djsb := dump2hex(demo_json_stringbuf)
	pj.Strings = pj.Strings[:len(djsb)]
	pj.isvalid = true
	copy(pj.Strings[:], djsb)

	djt := dump2hex(demo_json_tape)
	for i := 0; i < len(djt); i += 8 {
		pj.Tape = append(pj.Tape, binary.LittleEndian.Uint64(djt[i:i+8]))
	}

	// keep backup of the current stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	pj.dump_raw_tape()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the previous stdout
	out := <-outC

	if out != expected {
		t.Errorf("TestDumpRawDemoJson: got: %s want: %s", out, expected)
	}
}

// Parse the output of hex.Dump([]byte) back into a byte slice
func dump2hex(data string) []byte {
	addr, addrFrom := uint64(0), ^uint64(0)
	blob := make([]byte, 0)
	lines := strings.Split(data, "\n")
	for _, l := range lines {
		sections := strings.Split(l, "  ")
		if len(sections) < 1 || len(sections[0]) == 0 {
			continue
		} else if sections[0] == "*" {
			addrFrom = addr
			continue
		}

		if a, err := strconv.ParseUint("0x"+sections[0], 0, 64); err != nil {
			log.Fatal(err)
		} else {
			addr = a
			if addrFrom != ^uint64(0) {
				for a := addrFrom + 16; a < addr; a += 16 {
					blob = append(blob, blob[len(blob)-16:len(blob)]...)
				}
				addrFrom = ^uint64(0)
			}
		}

		if len(sections) < 2 {
			continue
		}
		for s := 1; s <= 2; s++ {
			parts := strings.Split(sections[s], " ")
			decoded, err := hex.DecodeString(strings.Join(parts, ""))
			if err != nil {
				log.Fatal(err)
			}
			blob = append(blob, decoded...)
		}
	}
	return blob
}

const demo_json_tape = `
00000000  26 00 00 00 00 00 00 72  26 00 00 00 00 00 00 7b  |&......r&......{|
00000010  00 00 00 00 00 00 00 22  25 00 00 00 00 00 00 7b  |......."%......{|
00000020  0a 00 00 00 00 00 00 22  00 00 00 00 00 00 00 6c  |.......".......l|
00000030  20 03 00 00 00 00 00 00  14 00 00 00 00 00 00 22  | .............."|
00000040  00 00 00 00 00 00 00 6c  58 02 00 00 00 00 00 00  |.......lX.......|
00000050  1f 00 00 00 00 00 00 22  29 00 00 00 00 00 00 22  |.......")......"|
00000060  42 00 00 00 00 00 00 22  17 00 00 00 00 00 00 7b  |B......".......{|
00000070  50 00 00 00 00 00 00 22  58 00 00 00 00 00 00 22  |P......"X......"|
00000080  83 00 00 00 00 00 00 22  00 00 00 00 00 00 00 6c  |.......".......l|
00000090  7d 00 00 00 00 00 00 00  8e 00 00 00 00 00 00 22  |}.............."|
000000a0  00 00 00 00 00 00 00 6c  64 00 00 00 00 00 00 00  |.......ld.......|
000000b0  0d 00 00 00 00 00 00 7d  98 00 00 00 00 00 00 22  |.......}......."|
000000c0  00 00 00 00 00 00 00 66  a5 00 00 00 00 00 00 22  |.......f......."|
000000d0  24 00 00 00 00 00 00 5b  00 00 00 00 00 00 00 6c  |$......[.......l|
000000e0  74 00 00 00 00 00 00 00  00 00 00 00 00 00 00 6c  |t..............l|
000000f0  af 03 00 00 00 00 00 00  00 00 00 00 00 00 00 6c  |...............l|
00000100  ea 00 00 00 00 00 00 00  00 00 00 00 00 00 00 6c  |...............l|
00000110  89 97 00 00 00 00 00 00  1a 00 00 00 00 00 00 5d  |...............]|
00000120  03 00 00 00 00 00 00 7d  01 00 00 00 00 00 00 7d  |.......}.......}|
00000130  00 00 00 00 00 00 00 72                           |.......r|
00000138`

const demo_json_stringbuf = `
00000000  05 00 00 00 49 6d 61 67  65 00 05 00 00 00 57 69  |....Image.....Wi|
00000010  64 74 68 00 06 00 00 00  48 65 69 67 68 74 00 05  |dth.....Height..|
00000020  00 00 00 54 69 74 6c 65  00 14 00 00 00 56 69 65  |...Title.....Vie|
00000030  77 20 66 72 6f 6d 20 31  35 74 68 20 46 6c 6f 6f  |w from 15th Floo|
00000040  72 00 09 00 00 00 54 68  75 6d 62 6e 61 69 6c 00  |r.....Thumbnail.|
00000050  03 00 00 00 55 72 6c 00  26 00 00 00 68 74 74 70  |....Url.&...http|
00000060  3a 2f 2f 77 77 77 2e 65  78 61 6d 70 6c 65 2e 63  |://www.example.c|
00000070  6f 6d 2f 69 6d 61 67 65  2f 34 38 31 39 38 39 39  |om/image/4819899|
00000080  34 33 00 06 00 00 00 48  65 69 67 68 74 00 05 00  |43.....Height...|
00000090  00 00 57 69 64 74 68 00  08 00 00 00 41 6e 69 6d  |..Width.....Anim|
000000a0  61 74 65 64 00 03 00 00  00 49 44 73 00           |ated.....IDs.|
000000ad
`
