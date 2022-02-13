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
	"testing"
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
	b.Run("nocopy-par", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			pj := &ParsedJson{}
			b.SetBytes(int64(len(msg)))
			b.ReportAllocs()
			b.ResetTimer()
			for pb.Next() {
				// Reset tape
				var err error
				pj, err = Parse(msg, pj, WithCopyStrings(false))
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

}

func BenchmarkParseSmall(b *testing.B)  { benchmarkFromFile(b, "payload-small") }
func BenchmarkParseMedium(b *testing.B) { benchmarkFromFile(b, "payload-medium") }
func BenchmarkParseLarge(b *testing.B)  { benchmarkFromFile(b, "payload-large") }

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
			elem, err = iter.FindElement(elem, "users")
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar, err = elem.Iter.Array(ar)
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar.ForEach(func(i Iter) {
				elem, err = i.FindElement(elem, "username")
				if checkErrs && err != nil {
					b.Fatal(err)
				}
				_, _ = elem.Iter.StringBytes()
			})

			elem, err = iter.FindElement(elem, "topics", "topics")
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar, err = elem.Iter.Array(ar)
			if checkErrs && err != nil {
				b.Fatal(err)
			}
			ar.ForEach(func(i Iter) {
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
					elem, err = i.FindElement(elem, "id")
					if checkErrs && err != nil {
						b.Fatal(err)
					}
					_, _ = elem.Iter.Int()
					//b.Log(elem.Iter.Int())
					elem, err = i.FindElement(elem, "slug")
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
