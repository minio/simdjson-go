package simdjson

import (
	"testing"
)

func BenchmarkStage1(b *testing.B) {

	b.SetBytes(int64(len(demo_json)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := ParsedJson{}
	pj.structural_indexes = make([]uint32, 0, 1024)

	for i := 0; i < b.N; i++ {
		pj.structural_indexes = pj.structural_indexes[:0]
		find_structural_indices([]byte(demo_json), &pj)
	}
}
