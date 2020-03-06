//+build !noasm
//+build !appengine
//+build gc

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
	"math/bits"
	"strings"
	"testing"
)

func TestStage1FindMarks(t *testing.T) {

	testCases := []struct {
		quoted                string
		structurals           string
		whitespace            string
		structurals_finalized string
	}{
		{
			// {"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor
			"0111111000111111000000111111100000011111100111111111111111111111", // quoted
			"1000000011000000010001000000001000100000001000000000000000000000", // structurals
			"0000000000000000000000000000000000000000000000001000010000100000", // whitespace
			"1100000011100000011001100000001100110000001100000000000000000000", // structurals_finalized
		},
	}

	prev_iter_ends_odd_backslash := uint64(0)
	odd_ends := find_odd_backslash_sequences([]byte(demo_json), &prev_iter_ends_odd_backslash)

	if odd_ends != 0 {
		t.Errorf("TestStage1FindMarks: got: %d want: %d", odd_ends, 0)
	}

	// detect insides of quote pairs ("quote_mask") and also our quote_bits themselves
	quote_bits := uint64(0)
	prev_iter_inside_quote, error_mask := uint64(0), uint64(0)
	quote_mask := find_quote_mask_and_bits([]byte(demo_json), odd_ends, &prev_iter_inside_quote, &quote_bits, &error_mask)
	quoted := fmt.Sprintf("%064b", bits.Reverse64(quote_mask))
	if quoted != testCases[0].quoted {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", quoted, testCases[0].quoted)
	}

	structurals_mask := uint64(0)
	whitespace_mask := uint64(0)
	find_whitespace_and_structurals([]byte(demo_json), &whitespace_mask, &structurals_mask)

	structurals := fmt.Sprintf("%064b", bits.Reverse64(structurals_mask))
	if structurals != testCases[0].structurals {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", structurals, testCases[0].structurals)
	}
	whitespace := fmt.Sprintf("%064b", bits.Reverse64(whitespace_mask))
	if whitespace != testCases[0].whitespace {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", whitespace, testCases[0].whitespace)
	}

	// fixup structurals to reflect quotes and add pseudo-structural characters
	prev_iter_ends_pseudo_pred := uint64(0)
	structurals_mask = finalize_structurals(structurals_mask, whitespace_mask, quote_mask, quote_bits, &prev_iter_ends_pseudo_pred)

	structural_finalized := fmt.Sprintf("%064b", bits.Reverse64(structurals_mask))
	if structural_finalized != testCases[0].structurals_finalized {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", structural_finalized, testCases[0].structurals_finalized)
	}
}

func TestFindStructuralIndices(t *testing.T) {

	parsed := []string{
		`{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		` "Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`        :{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`         {"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`          "Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                 :800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                  800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                     ,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                      "Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                              :600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                               600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                  ,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                   "Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                          :"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                           "View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                 ,"Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                  "Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                             :{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                              {"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                               "Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                    :"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                     "http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                             ,"Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                              "Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                      :125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                       125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                          ,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                           "Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                  :100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                   100},"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                      },"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                       ,"Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                        "Animated":false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                                  :false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                                   false,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                                        ,"IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                                         "IDs":[116,943,234,38793]}}`,
		`                                                                                                                                                                              :[116,943,234,38793]}}`,
		`                                                                                                                                                                               [116,943,234,38793]}}`,
		`                                                                                                                                                                                116,943,234,38793]}}`,
		`                                                                                                                                                                                   ,943,234,38793]}}`,
		`                                                                                                                                                                                    943,234,38793]}}`,
		`                                                                                                                                                                                       ,234,38793]}}`,
		`                                                                                                                                                                                        234,38793]}}`,
		`                                                                                                                                                                                           ,38793]}}`,
		`                                                                                                                                                                                            38793]}}`,
		`                                                                                                                                                                                                 ]}}`,
		`                                                                                                                                                                                                  }}`,
		`                                                                                                                                                                                                   }`,
	}

	pj := internalParsedJson{}
	pj.index_chan = make(chan indexChan, 16)

	// No need to spawn go-routine since the channel is large enough
	find_structural_indices([]byte(demo_json), &pj)

	ipos, pos := 0, ^uint64(0)
	for ic := range pj.index_chan {
		for j := 0; j < ic.length; j++ {
			pos += uint64((*ic.indexes)[j])
			result := fmt.Sprintf("%s%s", strings.Repeat(" ", int(pos)), demo_json[pos:])
			// fmt.Printf("`%s`,\n", result)
			if result != parsed[ipos] {
				t.Errorf("TestFindStructuralBits: got: %s want: %s", result, parsed[ipos])
			}
			ipos++
		}
	}
}

func BenchmarkStage1(b *testing.B) {
	msg := loadCompressed(b, "twitter")

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := internalParsedJson{}

	for i := 0; i < b.N; i++ {
		// Create new channel (large enough so we won't block)
		pj.index_chan = make(chan indexChan, 128)
		find_structural_indices([]byte(msg), &pj)
	}
}
