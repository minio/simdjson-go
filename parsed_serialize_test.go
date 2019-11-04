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
			if false {
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
