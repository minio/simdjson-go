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
	"bytes"
	"testing"
)

func BenchmarkSerialize(b *testing.B) {
	bench := func(b *testing.B, s *serializer) {
		for _, tt := range testCases {
			s := newSerializer()
			b.Run(tt.name, func(b *testing.B) {
				tap, sb, org := loadCompressed(b, tt.name)
				pj, err := loadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
				if err != nil {
					b.Fatal(err)
				}
				output := s.Serialize(nil, *pj)
				if true {
					b.Log(len(org), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(org)), "%")
				}
				//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
				b.SetBytes(int64(len(org)))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					output = s.Serialize(output[:0], *pj)
				}
			})
		}
	}
	b.Run("default", func(b *testing.B) {
		s := newSerializer()
		bench(b, s)
	})
	b.Run("none", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressNone)
		bench(b, s)
	})
	b.Run("fast", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressFast)
		bench(b, s)
	})
	b.Run("best", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressBest)
		bench(b, s)
	})
}

func BenchmarkDeSerialize(b *testing.B) {
	bench := func(b *testing.B, s *serializer) {
		for _, tt := range testCases {
			b.Run(tt.name, func(b *testing.B) {
				tap, sb, org := loadCompressed(b, tt.name)
				pj, err := loadTape(bytes.NewBuffer(tap), bytes.NewBuffer(sb))
				if err != nil {
					b.Fatal(err)
				}

				output := s.Serialize(nil, *pj)
				if false {
					b.Log(len(org), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(org)), "%")
				}
				//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
				pj2, err := s.Deserialize(output, nil)
				if err != nil {
					b.Fatal(err)
				}

				b.SetBytes(int64(len(org)))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					pj2, err = s.Deserialize(output, pj2)
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}

	b.Run("default", func(b *testing.B) {
		s := newSerializer()
		bench(b, s)
	})
	b.Run("none", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressNone)
		bench(b, s)
	})
	b.Run("fast", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressFast)
		bench(b, s)
	})
	b.Run("best", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressBest)
		bench(b, s)
	})
}

func BenchmarkSerializeNDJSON(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj, err := ParseND(ndjson, nil)
	if err != nil {
		b.Fatal(err)
	}
	bench := func(b *testing.B, s *serializer) {
		output := s.Serialize(nil, *pj)
		if true {
			b.Log(len(ndjson), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(ndjson)), "%")
		}
		//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			output = s.Serialize(output[:0], *pj)
		}
	}
	b.Run("default", func(b *testing.B) {
		s := newSerializer()
		bench(b, s)
	})
	b.Run("none", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressNone)
		bench(b, s)
	})
	b.Run("fast", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressFast)
		bench(b, s)
	})
	b.Run("best", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressBest)
		bench(b, s)
	})
}

func BenchmarkDeSerializeNDJSON(b *testing.B) {
	ndjson := loadFile("testdata/parking-citations-1M.json.zst")

	pj, err := ParseND(ndjson, nil)
	if err != nil {
		b.Fatal(err)
	}
	bench := func(b *testing.B, s *serializer) {
		output := s.Serialize(nil, *pj)
		if false {
			b.Log(len(ndjson), "(JSON) ->", len(output), "(Serialized)", 100*float64(len(output))/float64(len(ndjson)), "%")
		}
		pj2, err := s.Deserialize(output, nil)
		if err != nil {
			b.Fatal(err)
		}
		//_ = ioutil.WriteFile(filepath.Join("testdata", tt.name+".compressed"), output, os.ModePerm)
		b.SetBytes(int64(len(ndjson)))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pj2, err = s.Deserialize(output, pj2)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
	b.Run("default", func(b *testing.B) {
		s := newSerializer()
		bench(b, s)
	})
	b.Run("none", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressNone)
		bench(b, s)
	})
	b.Run("fast", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressFast)
		bench(b, s)
	})
	b.Run("best", func(b *testing.B) {
		s := newSerializer()
		s.CompressMode(CompressBest)
		bench(b, s)
	})
}
