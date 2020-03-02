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
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"
)

func closeEnough(d1, d2 float64) (ce bool) {
	return math.Abs((d1-d2)/(0.5*(d1+d2))) < 1e-20
}

func closeEnoughLessPrecision(d1, d2 float64) (ce bool) {
	return math.Abs((d1-d2)/(0.5*(d1+d2))) < 1e-15
}

// The following benchmarking code is borrowed from Golang (https://golang.org/src/strconv/atoi_test.go)

func BenchmarkParseIntGolang(b *testing.B) {
	b.Run("Pos", func(b *testing.B) {
		benchmarkParseIntGolang(b, 1)
	})
	b.Run("Neg", func(b *testing.B) {
		benchmarkParseIntGolang(b, -1)
	})
}

type benchCase struct {
	name string
	num  int64
}

func benchmarkParseIntGolang(b *testing.B, neg int) {
	cases := []benchCase{
		{"63bit", 1<<63 - 1},
	}
	for _, cs := range cases {
		b.Run(cs.name, func(b *testing.B) {
			s := fmt.Sprintf("%d", cs.num*int64(neg))
			for i := 0; i < b.N; i++ {
				out, _ := strconv.ParseInt(s, 10, 64)
				BenchSink += int(out)
			}
		})
	}
}

func BenchmarkAtoiGolang(b *testing.B) {
	b.Run("Pos", func(b *testing.B) {
		benchmarkAtoiGolang(b, 1)
	})
	b.Run("Neg", func(b *testing.B) {
		benchmarkAtoiGolang(b, -1)
	})
}

func benchmarkAtoiGolang(b *testing.B, neg int) {
	cases := []benchCase{}
	if strconv.IntSize == 64 {
		cases = append(cases, []benchCase{
			{"63bit", 1<<63 - 1},
		}...)
	}
	for _, cs := range cases {
		b.Run(cs.name, func(b *testing.B) {
			s := fmt.Sprintf("%d", cs.num*int64(neg))
			for i := 0; i < b.N; i++ {
				out, _ := strconv.Atoi(s)
				BenchSink += out
			}
		})
	}
}

var BenchSink int // make sure compiler cannot optimize away benchmarks

// The following benchmarking code is borrowed from Golang (https://golang.org/src/strconv/atof_test.go)

type atofSimpleTest struct {
	x float64
	s string
}

var (
	atofOnce                   sync.Once
	benchmarksRandomBits       [1024]string
	benchmarksRandomNormal     [1024]string
	benchmarksRandomBitsSimd   [1024]string
	benchmarksRandomNormalSimd [1024]string
)

func initAtof() {
	atofOnce.Do(initAtofOnce)
}

func initAtofOnce() {

	// Generate random inputs for tests and benchmarks
	rand.Seed(time.Now().UnixNano())

	for i := range benchmarksRandomBits {
		bits := uint64(rand.Uint32())<<32 | uint64(rand.Uint32())
		x := math.Float64frombits(bits)
		benchmarksRandomBits[i] = strconv.FormatFloat(x, 'g', -1, 64)
		benchmarksRandomBitsSimd[i] = benchmarksRandomBits[i] + ":"
	}

	for i := range benchmarksRandomNormal {
		x := rand.NormFloat64()
		benchmarksRandomNormal[i] = strconv.FormatFloat(x, 'g', -1, 64)
		benchmarksRandomNormalSimd[i] = benchmarksRandomNormal[i] + ":"
	}
}

func BenchmarkParseAtof64FloatExpGolang(b *testing.B) {
	for i := 0; i < b.N; i++ {
		strconv.ParseFloat("-5.09e75", 64)
	}
}

func BenchmarkParseAtof64BigGolang(b *testing.B) {
	for i := 0; i < b.N; i++ {
		strconv.ParseFloat("123456789123456789123456789", 64)
	}
}

func BenchmarkParseAtof64RandomBitsGolang(b *testing.B) {
	initAtof()
	for i := 0; i < b.N; i++ {
		strconv.ParseFloat(benchmarksRandomBits[i%1024], 64)
	}
}

func BenchmarkParseAtof64RandomFloatsGolang(b *testing.B) {
	initAtof()
	for i := 0; i < b.N; i++ {
		strconv.ParseFloat(benchmarksRandomNormal[i%1024], 64)
	}
}
