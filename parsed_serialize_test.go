package simdjson

import (
	"bytes"
	"testing"
)

func BenchmarkSerialize(b *testing.B) {
	for _, tt := range testCases {
		var s serializer
		b.Run(tt.name, func(b *testing.B) {
			tap, sb, org := loadCompressed(b, tt.name)
			pj, err := LoadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				b.Fatal(err)
			}
			output, err := s.Serialize(nil, *pj)
			if err != nil {
				b.Fatal(err)
			}
			if true {
				b.Log(len(org), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(org)), "%")
			}
			//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
			b.SetBytes(int64(len(org)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output, err = s.Serialize(output[:0], *pj)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkDeSerialize(b *testing.B) {
	for _, tt := range testCases {
		var s serializer
		b.Run(tt.name, func(b *testing.B) {
			tap, sb, org := loadCompressed(b, tt.name)
			pj, err := LoadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				b.Fatal(err)
			}
			output, err := s.Serialize(nil, *pj)
			if err != nil {
				b.Fatal(err)
			}
			if false {
				b.Log(len(org), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(org)), "%")
			}
			//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
			pj2, err := s.DeSerialize(output, nil)
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(len(org)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pj2, err = s.DeSerialize(output, pj2)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSerializeNDJSON(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	var s serializer
	b.Run("all", func(b *testing.B) {
		output, err := s.Serialize(nil, pj.ParsedJson)
		if err != nil {
			b.Fatal(err)
		}
		if true {
			b.Log(len(ndjson), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(ndjson)), "%")
		}
		//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			output, err = s.Serialize(output[:0], pj.ParsedJson)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkDeSerializeNDJSON(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	var s serializer
	b.Run("all", func(b *testing.B) {
		output, err := s.Serialize(nil, pj.ParsedJson)
		if err != nil {
			b.Fatal(err)
		}
		if true {
			b.Log(len(ndjson), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(ndjson)), "%")
		}
		pj2, err := s.DeSerialize(output, nil)
		if err != nil {
			b.Fatal(err)
		}
		//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pj2, err = s.DeSerialize(output, pj2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
