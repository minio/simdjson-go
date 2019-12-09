package simdjson

import (
	"encoding/json"
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

func benchmarkEncodingJson(b *testing.B, filename string) {

	_, _, msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		var parsed map[string]interface{}

		json.Unmarshal(msg, &parsed)
	}
}

func BenchmarkEncodingJsonApache_builds(b *testing.B)  { benchmarkEncodingJson(b, "apache_builds") }
func BenchmarkEncodingJsonCanada(b *testing.B)         { benchmarkEncodingJson(b, "canada") }
func BenchmarkEncodingJsonCitm_catalog(b *testing.B)   { benchmarkEncodingJson(b, "citm_catalog") }
func BenchmarkEncodingJsonGithub_events(b *testing.B)  { benchmarkEncodingJson(b, "github_events") }
func BenchmarkEncodingJsonGsoc_2018(b *testing.B)      { benchmarkEncodingJson(b, "gsoc-2018") }
func BenchmarkEncodingJsonInstruments(b *testing.B)    { benchmarkEncodingJson(b, "instruments") }
func BenchmarkEncodingJsonMarine_ik(b *testing.B)      { benchmarkEncodingJson(b, "marine_ik") }
func BenchmarkEncodingJsonMesh(b *testing.B)           { benchmarkEncodingJson(b, "mesh") }
func BenchmarkEncodingJsonMesh_pretty(b *testing.B)    { benchmarkEncodingJson(b, "mesh.pretty") }
func BenchmarkEncodingJsonNumbers(b *testing.B)        { benchmarkEncodingJson(b, "numbers") }
func BenchmarkEncodingJsonRandom(b *testing.B)         { benchmarkEncodingJson(b, "random") }
func BenchmarkEncodingJsonTwitter(b *testing.B)        { benchmarkEncodingJson(b, "twitter") }
func BenchmarkEncodingJsonTwitterescaped(b *testing.B) { benchmarkEncodingJson(b, "twitterescaped") }
func BenchmarkEncodingJsonUpdate_center(b *testing.B)  { benchmarkEncodingJson(b, "update-center") }
