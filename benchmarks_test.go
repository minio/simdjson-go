package simdjson

import (
	"testing"
)

func benchmarkFromFile(b *testing.B, filename string) {

	_, _, msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := internalParsedJson{}
	pj.initialize(len(msg) * 2)

	for i := 0; i < b.N; i++ {

		// Reset tape
		pj.Tape = pj.Tape[:0]
		pj.Strings = pj.Strings[:0]

		pj.parseMessage(msg)
	}
}

func BenchmarkApache_builds(b *testing.B)  { benchmarkFromFile(b, "apache_builds") }
func BenchmarkCanada(b *testing.B)         { benchmarkFromFile(b, "canada") }
func BenchmarkCitm_catalog(b *testing.B)   { benchmarkFromFile(b, "citm_catalog") }
func BenchmarkGithub_events(b *testing.B)  { benchmarkFromFile(b, "github_events") }
func BenchmarkGsoc_2018(b *testing.B)      { benchmarkFromFile(b, "gsoc-2018") }
func BenchmarkInstruments(b *testing.B)    { benchmarkFromFile(b, "instruments") }
func BenchmarkMarine_ik(b *testing.B)      { benchmarkFromFile(b, "marine_ik") }
func BenchmarkMesh(b *testing.B)           { benchmarkFromFile(b, "mesh") }
func BenchmarkMesh_pretty(b *testing.B)    { benchmarkFromFile(b, "mesh.pretty") }
func BenchmarkNumbers(b *testing.B)        { benchmarkFromFile(b, "numbers") }
func BenchmarkRandom(b *testing.B)         { benchmarkFromFile(b, "random") }
func BenchmarkTwitter(b *testing.B)        { benchmarkFromFile(b, "twitter") }
func BenchmarkTwitterescaped(b *testing.B) { benchmarkFromFile(b, "twitterescaped") }
func BenchmarkUpdate_center(b *testing.B)  { benchmarkFromFile(b, "update-center") }
