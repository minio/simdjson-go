/*
 * MinIO Cloud Storage, (C) 2022 MinIO, Inc.
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

package simdjson_benchmarks

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/buger/jsonparser"
	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/zstd"

	simdjson "github.com/minio/simdjson-go"
)

func benchmarkEncodingJson(b *testing.B, filename string) {
	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	var parsed interface{}
	for i := 0; i < b.N; i++ {
		if err := json.Unmarshal(msg, &parsed); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkJsoniter(b *testing.B, filename string) {
	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var parsed interface{}
	for i := 0; i < b.N; i++ {
		if err := json.Unmarshal(msg, &parsed); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkSimdJson(b *testing.B, filename string) {
	if !simdjson.SupportedCPU() {
		b.SkipNow()
	}

	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := &simdjson.ParsedJson{}
	for i := 0; i < b.N; i++ {
		// Reset tape
		var err error
		pj, err = simdjson.Parse(msg, pj, simdjson.WithCopyStrings(false))
		if err != nil {
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

func BenchmarkSimdJsonApache_builds(b *testing.B)  { benchmarkSimdJson(b, "apache_builds") }
func BenchmarkSimdJsonCanada(b *testing.B)         { benchmarkSimdJson(b, "canada") }
func BenchmarkSimdJsonCitm_catalog(b *testing.B)   { benchmarkSimdJson(b, "citm_catalog") }
func BenchmarkSimdJsonGithub_events(b *testing.B)  { benchmarkSimdJson(b, "github_events") }
func BenchmarkSimdJsonGsoc_2018(b *testing.B)      { benchmarkSimdJson(b, "gsoc-2018") }
func BenchmarkSimdJsonInstruments(b *testing.B)    { benchmarkSimdJson(b, "instruments") }
func BenchmarkSimdJsonMarine_ik(b *testing.B)      { benchmarkSimdJson(b, "marine_ik") }
func BenchmarkSimdJsonMesh(b *testing.B)           { benchmarkSimdJson(b, "mesh") }
func BenchmarkSimdJsonMesh_pretty(b *testing.B)    { benchmarkSimdJson(b, "mesh.pretty") }
func BenchmarkSimdJsonNumbers(b *testing.B)        { benchmarkSimdJson(b, "numbers") }
func BenchmarkSimdJsonRandom(b *testing.B)         { benchmarkSimdJson(b, "random") }
func BenchmarkSimdJsonTwitter(b *testing.B)        { benchmarkSimdJson(b, "twitter") }
func BenchmarkSimdJsonTwitterEscaped(b *testing.B) { benchmarkSimdJson(b, "twitterescaped") }
func BenchmarkSimdJsonUpdate_center(b *testing.B)  { benchmarkSimdJson(b, "update-center") }

func BenchmarkBugerJsonParserLarge(b *testing.B) {
	largeFixture := loadCompressed(b, "payload-large")
	const logVals = false
	b.SetBytes(int64(len(largeFixture)))
	b.ReportAllocs()
	b.ResetTimer()
	var dump int
	for i := 0; i < b.N; i++ {
		jsonparser.ArrayEach(largeFixture, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			sval, _, _, _ := jsonparser.Get(value, "username")
			if logVals && i == 0 {
				b.Log(string(sval))
			}
			dump += len(sval)
		}, "users")

		jsonparser.ArrayEach(largeFixture, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			ival, _ := jsonparser.GetInt(value, "id")
			if logVals && i == 0 {
				b.Log(ival)
			}
			dump += int(ival)
			sval, _, _, _ := jsonparser.Get(value, "slug")
			if logVals && i == 0 {
				b.Log(string(sval))
			}
			dump += len(sval)
		}, "topics", "topics")
	}
	if dump == 0 {
		b.Log("")
	}
}

// tester and loadCompressed should be kept in sync with minio/simdjson-go/parsed_json_test.go.
type tester interface {
	Fatal(args ...interface{})
}

func loadCompressed(t tester, file string) (ref []byte) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err = ioutil.ReadFile(filepath.Join("../", "testdata", file+".json.zst"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err = dec.DecodeAll(ref, nil)
	if err != nil {
		t.Fatal(err)
	}

	return ref
}
