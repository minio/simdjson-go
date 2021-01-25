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
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestNumberIsValid(t *testing.T) {
	// From: https://stackoverflow.com/a/13340826
	var jsonNumberRegexp = regexp.MustCompile(`^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?$`)
	isValidNumber := func(s string) bool {
		tag, _, _ := parseNumber([]byte(s))
		return tag != TagEnd
	}
	validTests := []string{
		"0",
		"-0",
		"1",
		"-1",
		"0.1",
		"-0.1",
		"1234",
		"-1234",
		"12.34",
		"-12.34",
		"12E0",
		"12E1",
		"12e34",
		"12E-0",
		"12e+1",
		"12e-34",
		"-12E0",
		"-12E1",
		"-12e34",
		"-12E-0",
		"-12e+1",
		"-12e-34",
		"1.2E0",
		"1.2E1",
		"1.2e34",
		"1.2E-0",
		"1.2e+1",
		"1.2e-34",
		"-1.2E0",
		"-1.2E1",
		"-1.2e34",
		"-1.2E-0",
		"-1.2e+1",
		"-1.2e-34",
		"0E0",
		"0E1",
		"0e34",
		"0E-0",
		"0e+1",
		"0e-34",
		"-0E0",
		"-0E1",
		"-0e34",
		"-0E-0",
		"-0e+1",
		"-0e-34",
	}

	for _, test := range validTests {
		if !isValidNumber(test) {
			t.Errorf("%s should be valid", test)
		}

		if !jsonNumberRegexp.MatchString(test) {
			t.Errorf("%s should be valid but regexp does not match", test)
		}
	}

	invalidTests := []string{
		"",
		"invalid",
		"1.0.1",
		"1..1",
		"-1-2",
		"012a42",
		"01.2",
		"012",
		"12E12.12",
		"1e2e3",
		"1e+-2",
		"1e--23",
		"1e",
		"e1",
		"1e+",
		"1ea",
		"1a",
		"1.a",
		"1.",
		"01",
		"1.e1",
	}

	for _, test := range invalidTests {
		if isValidNumber(test) {
			t.Errorf("%s should be invalid", test)
		}

		if jsonNumberRegexp.MatchString(test) {
			t.Errorf("%s should be invalid but matches regexp", test)
		}
	}
}

func closeEnough(d1, d2 float64) (ce bool) {
	return math.Abs((d1-d2)/(0.5*(d1+d2))) < 1e-20
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

var BenchSink int // make sure compiler cannot optimize away benchmarks

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
