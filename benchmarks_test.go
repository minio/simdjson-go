package simdjson

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

func benchmarkFromFile(b *testing.B, filename string) {

	msg, err := ioutil.ReadFile(filepath.Join("testdata", filename + ".json"))
	if err != nil {
		panic("failed to read file")
	}

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := ParsedJson{}
	pj.initialize(len(msg)*2)

	for i := 0; i < b.N; i++ {
		pj.structural_indexes = pj.structural_indexes[:0]
		pj.tape = pj.tape[:0]
		pj.strings = pj.strings[:0]
		find_structural_indices(msg, &pj)
		unified_machine(msg, &pj)
	}
}

func BenchmarkApache_builds(b *testing.B) { benchmarkFromFile(b, "apache_builds") }
func BenchmarkCanada(b *testing.B) { benchmarkFromFile(b, "canada") }
func BenchmarkCitm_catalog(b *testing.B) { benchmarkFromFile(b, "citm_catalog") }
func BenchmarkGithub_events(b *testing.B) { benchmarkFromFile(b, "github_events") }
func BenchmarkGsoc_2018(b *testing.B) { benchmarkFromFile(b, "gsoc-2018") }
func BenchmarkInstruments(b *testing.B) { benchmarkFromFile(b, "instruments") }
func BenchmarkMarine_ik(b *testing.B) { benchmarkFromFile(b, "marine_ik") }
func BenchmarkMesh(b *testing.B) { benchmarkFromFile(b, "mesh") }
func BenchmarkMesh_pretty(b *testing.B) { benchmarkFromFile(b, "mesh.pretty") }
func BenchmarkNumbers(b *testing.B) { benchmarkFromFile(b, "numbers") }
func BenchmarkRandom(b *testing.B) { benchmarkFromFile(b, "random") }
func BenchmarkTwitter(b *testing.B) { benchmarkFromFile(b, "twitter") }
func BenchmarkTwitterescaped(b *testing.B) { benchmarkFromFile(b, "twitterescaped") }
func BenchmarkUpdate_center(b *testing.B) { benchmarkFromFile(b, "update-center") }

