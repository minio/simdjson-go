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

	"github.com/bytedance/sonic"
	jsoniter "github.com/json-iterator/go"
)

func benchmarkFromFile(b *testing.B, filename string) {
	if !SupportedCPU() {
		b.SkipNow()
	}
	msg := loadCompressed(b, filename)

	b.Run("copy", func(b *testing.B) {
		pj := &ParsedJson{}
		b.SetBytes(int64(len(msg)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Reset tape
			var err error
			pj, err = Parse(msg, pj, WithCopyStrings(true))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("nocopy", func(b *testing.B) {
		pj := &ParsedJson{}
		b.SetBytes(int64(len(msg)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Reset tape
			var err error
			pj, err = Parse(msg, pj, WithCopyStrings(false))
			if err != nil {
				b.Fatal(err)
			}
		}
	})

}

func BenchmarkSmall(b *testing.B)          { benchmarkFromFile(b, "payload-small") }
func BenchmarkMedium(b *testing.B)         { benchmarkFromFile(b, "payload-medium") }
func BenchmarkLarge(b *testing.B)          { benchmarkFromFile(b, "payload-large") }
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
func BenchmarkTwitterEscaped(b *testing.B) { benchmarkFromFile(b, "twitterescaped") }
func BenchmarkUpdate_center(b *testing.B)  { benchmarkFromFile(b, "update-center") }

func benchmarkSonicJson(b *testing.B, filename string) {
	msg := loadCompressed(b, filename)

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	var parsed interface{}
	for i := 0; i < b.N; i++ {
		if err := sonic.Unmarshal(msg, &parsed); err != nil {
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

func BenchmarkSonicJsonApache_builds(b *testing.B)  { benchmarkSonicJson(b, "apache_builds") }
func BenchmarkSonicJsonCanada(b *testing.B)         { benchmarkSonicJson(b, "canada") }
func BenchmarkSonicJsonCitm_catalog(b *testing.B)   { benchmarkSonicJson(b, "citm_catalog") }
func BenchmarkSonicJsonGithub_events(b *testing.B)  { benchmarkSonicJson(b, "github_events") }
func BenchmarkSonicJsonGsoc_2018(b *testing.B)      { benchmarkSonicJson(b, "gsoc-2018") }
func BenchmarkSonicJsonInstruments(b *testing.B)    { benchmarkSonicJson(b, "instruments") }
func BenchmarkSonicJsonMarine_ik(b *testing.B)      { benchmarkSonicJson(b, "marine_ik") }
func BenchmarkSonicJsonMesh(b *testing.B)           { benchmarkSonicJson(b, "mesh") }
func BenchmarkSonicJsonMesh_pretty(b *testing.B)    { benchmarkSonicJson(b, "mesh.pretty") }
func BenchmarkSonicJsonNumbers(b *testing.B)        { benchmarkSonicJson(b, "numbers") }
func BenchmarkSonicJsonRandom(b *testing.B)         { benchmarkSonicJson(b, "random") }
func BenchmarkSonicJsonTwitter(b *testing.B)        { benchmarkSonicJson(b, "twitter") }
func BenchmarkSonicJsonTwitterescaped(b *testing.B) { benchmarkSonicJson(b, "twitterescaped") }
func BenchmarkSonicJsonUpdate_center(b *testing.B)  { benchmarkSonicJson(b, "update-center") }

func BenchmarkJsonParserLarge(b *testing.B) {
	largeFixture := loadCompressed(b, "payload-large")

	b.Run("nocopy", func(b *testing.B) {
		pj := &ParsedJson{}
		b.SetBytes(int64(len(largeFixture)))
		b.ReportAllocs()
		b.ResetTimer()
		var elem *Element
		var ar *Array
		var obj *Object
		var onlyKeys = map[string]struct{}{
			"id":   {},
			"slug": {},
		}
		const checkErrs = false
		for i := 0; i < b.N; i++ {
			// Reset tape
			var err error
			pj, err = Parse(largeFixture, pj, WithCopyStrings(false))
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			iter := pj.Iter()
			elem, err = iter.FindElement("users", elem)
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar, err = elem.Iter.Array(ar)
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar.ForEach(func(t Type, i Iter) {
				elem, err = i.FindElement("username", elem)
				if checkErrs && err != nil {
					b.Fatal(err)
				}
				_, _ = elem.Iter.StringBytes()
			})

			elem, err = iter.FindElement("topics/topics", elem)
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar, err = elem.Iter.Array(ar)
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar.ForEach(func(t Type, i Iter) {
				if true {
					// Use foreach...
					obj, err = i.Object(obj)
					if checkErrs && err != nil {
						b.Fatal(err)
					}
					obj.ForEach(func(key []byte, i Iter) {
						if string(key) == "id" {
							_, err = i.Int()
							if checkErrs && err != nil {
								b.Fatal(err)
							}
						}
						if string(key) == "slug" {
							_, err = i.StringBytes()
							if checkErrs && err != nil {
								b.Fatal(err)
							}
						}

					}, onlyKeys)
				} else {
					elem, err = i.FindElement("id", elem)
					if checkErrs && err != nil {
						b.Fatal(err)
					}
					_, _ = elem.Iter.Int()
					//b.Log(elem.Iter.Int())
					elem, err = i.FindElement("slug", elem)
					if checkErrs && err != nil {
						b.Fatal(err)
					}
					_, _ = elem.Iter.StringBytes()
					//b.Log(elem.Iter.String())
				}
			})
		}
	})
}
