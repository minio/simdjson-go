/*
 * MinIO Cloud Storage, (C) 2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package simdjson

import (
	"encoding/json"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

func benchmarkFromFile(b *testing.B, filename string) {
	if !SupportedCPU() {
		b.SkipNow()
	}
	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := &ParsedJson{}

	for i := 0; i < b.N; i++ {
		// Reset tape
		var err error
		pj, err = Parse(msg, pj)
		if err != nil {
			b.Fatal(err)
		}
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

func benchmarkJsoniter(b *testing.B, filename string) {

	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	for i := 0; i < b.N; i++ {

		var parsed interface{}
		if err := json.Unmarshal(msg, &parsed); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkEncodingJson(b *testing.B, filename string) {

	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		var parsed interface{}
		if err := json.Unmarshal(msg, &parsed); err != nil {
			b.Fatal(err)
		}
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

func BenchmarkJsoniterApache_builds(b *testing.B)  { benchmarkJsoniter(b, "apache_builds") }
func BenchmarkJsoniterCanada(b *testing.B)         { benchmarkJsoniter(b, "canada") }
func BenchmarkJsoniterCitm_catalog(b *testing.B)   { benchmarkJsoniter(b, "citm_catalog") }
func BenchmarkJsoniterGithub_events(b *testing.B)  { benchmarkJsoniter(b, "github_events") }
func BenchmarkJsoniterGsoc_2018(b *testing.B)      { benchmarkJsoniter(b, "gsoc-2018") }
func BenchmarkJsoniterInstruments(b *testing.B)    { benchmarkJsoniter(b, "instruments") }
func BenchmarkJsoniterMarine_ik(b *testing.B)      { benchmarkJsoniter(b, "marine_ik") }
func BenchmarkJsoniterMesh(b *testing.B)           { benchmarkJsoniter(b, "mesh") }
func BenchmarkJsoniterMesh_pretty(b *testing.B)    { benchmarkJsoniter(b, "mesh.pretty") }
func BenchmarkJsoniterNumbers(b *testing.B)        { benchmarkJsoniter(b, "numbers") }
func BenchmarkJsoniterRandom(b *testing.B)         { benchmarkJsoniter(b, "random") }
func BenchmarkJsoniterTwitter(b *testing.B)        { benchmarkJsoniter(b, "twitter") }
func BenchmarkJsoniterTwitterescaped(b *testing.B) { benchmarkJsoniter(b, "twitterescaped") }
func BenchmarkJsoniterUpdate_center(b *testing.B)  { benchmarkJsoniter(b, "update-center") }
