package simdjson

import (
	"bytes"
	"testing"
)

func BenchmarkSerialize(b *testing.B) {
	for _, tt := range testCases {
		s := NewSerializer()
		b.Run(tt.name, func(b *testing.B) {
			tap, sb, org := loadCompressed(b, tt.name)
			pj, err := LoadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				b.Fatal(err)
			}
			output := s.Serialize(nil, *pj)
			if true {
				b.Log(len(org), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(org)), "%")
			}
			//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
			b.SetBytes(int64(len(org)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output = s.Serialize(output[:0], *pj)
			}
		})
	}
}

func BenchmarkDeSerialize(b *testing.B) {
	for _, tt := range testCases {
		s := NewSerializer()
		b.Run(tt.name, func(b *testing.B) {
			tap, sb, org := loadCompressed(b, tt.name)
			pj, err := LoadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
			if err != nil {
				b.Fatal(err)
			}
			s.compTags = blockTypeS2
			s.compValues = blockTypeS2
			output := s.Serialize(nil, *pj)
			if false {
				b.Log(len(org), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(org)), "%")
			}
			//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
			pj2, err := s.Deserialize(output, nil)
			if err != nil {
				b.Fatal(err)
			}

			b.SetBytes(int64(len(org)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pj2, err = s.Deserialize(output, pj2)
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

	s := NewSerializer()
	b.Run("all", func(b *testing.B) {
		output := s.Serialize(nil, pj.ParsedJson)
		if true {
			b.Log(len(ndjson), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(ndjson)), "%")
		}
		//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			output = s.Serialize(output[:0], pj.ParsedJson)
		}
	})
}

func BenchmarkDeSerializeNDJSON(b *testing.B) {
	ndjson := getPatchedNdjson("testdata/parking-citations-1M.json.zst")

	pj := internalParsedJson{}
	pj.initialize(len(ndjson) * 3 / 2)
	pj.parseMessage(ndjson)

	s := NewSerializer()
	b.Run("all", func(b *testing.B) {
		output := s.Serialize(nil, pj.ParsedJson)
		if false {
			b.Log(len(ndjson), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(ndjson)), "%")
		}
		pj2, err := s.Deserialize(output, nil)
		if err != nil {
			b.Fatal(err)
		}
		//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pj2, err = s.Deserialize(output, pj2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
