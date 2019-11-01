package simdjson

import (
	"fmt"
	"strings"
	"testing"
)

const demo_json = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

func reverseBinary(input string) string {
	// Get Unicode code points.
	n := 0
	rune := make([]rune, len(input))
	for _, r := range input {
		rune[n] = r
		n++
	}
	rune = rune[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		rune[i], rune[n-1-i] = rune[n-1-i], rune[i]
	}
	// Convert back to UTF-8.
	output := string(rune)
	if len(output) < 64 {
		output = output + strings.Repeat("0", 64-len(output))
	}
	return output
}

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
	quoted := reverseBinary(fmt.Sprintf("%b", quote_mask))
	if quoted != testCases[0].quoted {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", quoted, testCases[0].quoted)
	}

	structurals_mask := uint64(0)
	whitespace_mask := uint64(0)
	find_whitespace_and_structurals([]byte(demo_json), &whitespace_mask, &structurals_mask)

	structurals := reverseBinary(fmt.Sprintf("%b", structurals_mask))
	if structurals != testCases[0].structurals {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", structurals, testCases[0].structurals)
	}
	whitespace := reverseBinary(fmt.Sprintf("%b", whitespace_mask))
	if whitespace != testCases[0].whitespace {
		t.Errorf("TestStage1FindMarks: got: %s want: %s", whitespace, testCases[0].whitespace)
	}

	// fixup structurals to reflect quotes and add pseudo-structural characters
	prev_iter_ends_pseudo_pred := uint64(0)
	structurals_mask = finalize_structurals(structurals_mask, whitespace_mask, quote_mask, quote_bits, &prev_iter_ends_pseudo_pred)

	structural_finalized := reverseBinary(fmt.Sprintf("%b", structurals_mask))
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

	ipos, pos := 0, uint64(0xffffffffffffffff)
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

	_, _, msg := loadCompressed(b, "twitter")

	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	b.ResetTimer()

	pj := internalParsedJson{}

	for i := 0; i < b.N; i++ {
		// Create new channel (large enough so we won't block)
		pj.index_chan = make(chan indexChan, 32)
		find_structural_indices([]byte(msg), &pj)
	}
}
