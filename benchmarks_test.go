package simdjson

import (
	"testing"
)

func BenchmarkStage1(b *testing.B) {

	b.SetBytes(64)
	b.ReportAllocs()
	b.ResetTimer()

	base := make([]uint32, 0, 1024)

	for i := 0; i < b.N; i++ {
		prev_iter_ends_odd_backslash := uint64(0)
		odd_ends := find_odd_backslash_sequences([]byte(demo_json), &prev_iter_ends_odd_backslash)

		// detect insides of quote pairs ("quote_mask") and also our quote_bits themselves
		quote_bits := uint64(0)
		prev_iter_inside_quote, quote_bits, error_mask := uint64(0), uint64(0), uint64(0)
		quote_mask := find_quote_mask_and_bits([]byte(demo_json), odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)

		// take the previous iterations structural bits, not our current iteration, and flatten
		base = base[:0]
		idx, structurals := uint64(0), uint64(0x0)
		flatten_bits(&base, idx, structurals);

		whitespace := uint64(0)
		find_whitespace_and_structurals([]byte(demo_json), &whitespace, &structurals)

		// fixup structurals to reflect quotes and add pseudo-structural characters */
		prev_iter_ends_pseudo_pred := uint64(0)
		finalize_structurals(structurals, whitespace, quote_mask, quote_bits, &prev_iter_ends_pseudo_pred)
	}
}
