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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

const demo_json = `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

type tester interface {
	Fatal(args ...interface{})
}

func loadCompressed(t tester, file string) (ref []byte) {
	dec, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatal(err)
	}
	ref, err = ioutil.ReadFile(filepath.Join("testdata", file+".json.zst"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err = dec.DecodeAll(ref, nil)
	if err != nil {
		t.Fatal(err)
	}

	return ref
}

var testCases = []struct {
	name  string
	array bool
}{
	{
		name: "apache_builds",
	},
	{
		name: "canada",
	},
	{
		name: "citm_catalog",
	},
	{
		name:  "github_events",
		array: true,
	},
	{
		name: "gsoc-2018",
	},
	{
		name: "instruments",
	},
	{
		name:  "numbers",
		array: true,
	},
	{
		name: "marine_ik",
	},
	{
		name: "mesh",
	},
	{
		name: "mesh.pretty",
	},
	{
		name: "twitterescaped",
	},
	{
		name: "twitter",
	},
	{
		name: "random",
	},
	{
		name: "update-center",
	},
}

func BenchmarkIter_MarshalJSONBuffer(b *testing.B) {
	if !SupportedCPU() {
		b.SkipNow()
	}
	for _, tt := range testCases {
		b.Run(tt.name, func(b *testing.B) {
			ref := loadCompressed(b, tt.name)
			pj, err := Parse(ref, nil)
			if err != nil {
				b.Fatal(err)
			}
			iter := pj.Iter()
			cpy := iter
			output, err := cpy.MarshalJSON()
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(output)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cpy := iter
				output, err = cpy.MarshalJSONBuffer(output[:0])
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkGoMarshalJSON(b *testing.B) {
	for _, tt := range testCases {
		b.Run(tt.name, func(b *testing.B) {
			ref := loadCompressed(b, tt.name)
			var m interface{}
			m = map[string]interface{}{}
			if tt.array {
				m = []interface{}{}
			}
			err := json.Unmarshal(ref, &m)
			if err != nil {
				b.Fatal(err)
			}
			output, err := json.Marshal(m)
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(output)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output, err = json.Marshal(m)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestPrintJson(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	msg := []byte(demo_json)
	expected := `{"Image":{"Width":800,"Height":600,"Title":"View from 15th Floor","Thumbnail":{"Url":"http://www.example.com/image/481989943","Height":125,"Width":100},"Animated":false,"IDs":[116,943,234,38793]}}`

	pj, err := Parse(msg, nil)

	if err != nil {
		t.Errorf("parseMessage failed\n")
	}

	iter := pj.Iter()
	out, err := iter.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != expected {
		t.Errorf("TestPrintJson: got: %s want: %s", out, expected)
	}
}

func TestExchange(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"value": -20}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		t.Errorf("Parse failed: %v", err)
		return
	}
	for i := 0; i < 200; i++ {
		i := i
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			var cl *ParsedJson
			var o *Object
			for j := 0; j < 10; j++ {
				cl = pj.Clone(cl)
				iter := cl.Iter()
				iter.Advance()
				_, r, err := iter.Root(&iter)
				if err != nil {
					t.Fatalf("Root failed: %v", err)
				}
				o, err = r.Object(o)
				if err != nil {
					t.Fatalf("Object failed: %v", err)
				}
				_, _, err = o.NextElementBytes(r)
				if err != nil {
					t.Fatalf("NextElementBytes failed: %v", err)
				}
				want := uint64(i + j*100)
				err = r.SetUInt(want)
				if err != nil {
					t.Fatalf("SetUInt failed: %v", err)
					return
				}
				time.Sleep(10 * time.Millisecond)
				v, err := r.Uint()
				if err != nil {
					t.Fatalf("Uint failed: %v", err)
					return
				}
				if v != want {
					t.Errorf("want %d, got %d", want, v)
				}
			}
		})
	}
}

func TestIter_SetNull(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		want string
	}{
		{
			want: `{"0val":{"true":null,"false":null,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,null,null,"astring",-42,9223372036854775808,1.23455]}`,
		},
	}

	for _, test := range tests {
		t.Run("null", func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeBool, TypeNull:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetNull()
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}

					if iter.Type() != TypeNull {
						t.Errorf("Want type %v, got %v", TypeNull, iter.Type())
					}
				default:
					err := iter.SetNull()
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func TestIter_SetBool(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		setTo bool
		want  string
	}{
		{
			setTo: true,
			want:  `{"0val":{"true":true,"false":true,"nullval":true},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[true,true,true,"astring",-42,9223372036854775808,1.23455]}`,
		},
		{
			setTo: false,
			want:  `{"0val":{"true":false,"false":false,"nullval":false},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[false,false,false,"astring",-42,9223372036854775808,1.23455]}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.setTo), func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeBool, TypeNull:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetBool(test.setTo)
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}
					val, err := iter.Bool()
					if err != nil {
						t.Errorf("Unable to retrieve value: %v", err)
					}

					if val != test.setTo {
						t.Errorf("Want value %v, got %v", test.setTo, val)
					}
				default:
					err := iter.SetBool(test.setTo)
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func TestIter_SetFloat(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		setTo float64
		want  string
	}{
		{
			setTo: 69.420,
			want:  `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":69.42,"int":69.42,"uint":69.42},"stringval":"initial value","array":[null,true,false,"astring",69.42,69.42,69.42]}`,
		},
		{
			setTo: 10e30,
			want:  `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":1e+31,"int":1e+31,"uint":1e+31},"stringval":"initial value","array":[null,true,false,"astring",1e+31,1e+31,1e+31]}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.setTo), func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeInt, TypeFloat, TypeUint:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetFloat(test.setTo)
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}
					val, err := iter.Float()
					if err != nil {
						t.Errorf("Unable to retrieve value: %v", err)
					}

					if val != test.setTo {
						t.Errorf("Want value %v, got %v", test.setTo, val)
					}
				case TypeString:
					// Do not replace strings...
				default:
					err := iter.SetFloat(test.setTo)
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func TestIter_SetInt(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		setTo int64
		want  string
	}{
		{
			setTo: -69,
			want:  `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":-69,"int":-69,"uint":-69},"stringval":"initial value","array":[null,true,false,"astring",-69,-69,-69]}`,
		},
		{
			setTo: 42,
			want:  `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":42,"int":42,"uint":42},"stringval":"initial value","array":[null,true,false,"astring",42,42,42]}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.setTo), func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeInt, TypeFloat, TypeUint:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetInt(test.setTo)
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}
					val, err := iter.Int()
					if err != nil {
						t.Errorf("Unable to retrieve value: %v", err)
					}

					if val != test.setTo {
						t.Errorf("Want value %v, got %v", test.setTo, val)
					}
				case TypeString:
					// Do not replace strings...

				default:
					err := iter.SetInt(test.setTo)
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func TestIter_SetUInt(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		setTo uint64
		want  string
	}{
		{
			setTo: 69,
			want:  `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":69,"int":69,"uint":69},"stringval":"initial value","array":[null,true,false,"astring",69,69,69]}`,
		},
		{
			setTo: 420,
			want:  `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":420,"int":420,"uint":420},"stringval":"initial value","array":[null,true,false,"astring",420,420,420]}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.setTo), func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeInt, TypeFloat, TypeUint:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetUInt(test.setTo)
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}
					val, err := iter.Uint()
					if err != nil {
						t.Errorf("Unable to retrieve value: %v", err)
					}

					if val != test.setTo {
						t.Errorf("Want value %v, got %v", test.setTo, val)
					}
				case TypeString:
					// Do not replace strings...
				default:
					err := iter.SetUInt(test.setTo)
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func TestIter_SetString(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		setTo string
		want  string
	}{
		{
			setTo: "anotherval",
			want:  `{"anotherval":{"anotherval":true,"anotherval":false,"anotherval":null},"anotherval":{"anotherval":"anotherval","anotherval":"anotherval","anotherval":"anotherval"},"anotherval":"anotherval","anotherval":[null,true,false,"anotherval","anotherval","anotherval","anotherval"]}`,
		},
		{
			setTo: "",
			want:  `{"":{"":true,"":false,"":null},"":{"":"","":"","":""},"":"","":[null,true,false,"","","",""]}`,
		},
		{
			setTo: "\t",
			want:  `{"\t":{"\t":true,"\t":false,"\t":null},"\t":{"\t":"\t","\t":"\t","\t":"\t"},"\t":"\t","\t":[null,true,false,"\t","\t","\t","\t"]}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.setTo), func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeString, TypeInt, TypeFloat, TypeUint:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetString(test.setTo)
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}
					val, err := iter.String()
					if err != nil {
						t.Errorf("Unable to retrieve value: %v", err)
					}

					if val != test.setTo {
						t.Errorf("Want value %v, got %v", test.setTo, val)
					}
				default:
					err := iter.SetString(test.setTo)
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func TestIter_SetStringBytes(t *testing.T) {
	if !SupportedCPU() {
		t.SkipNow()
	}
	input := `{"0val":{"true":true,"false":false,"nullval":null},"1val":{"float":12.3456,"int":-42,"uint":9223372036854775808},"stringval":"initial value","array":[null,true,false,"astring",-42,9223372036854775808,1.23455]}`
	tests := []struct {
		setTo []byte
		want  string
	}{
		{
			setTo: []byte("anotherval"),
			want:  `{"anotherval":{"anotherval":true,"anotherval":false,"anotherval":null},"anotherval":{"anotherval":"anotherval","anotherval":"anotherval","anotherval":"anotherval"},"anotherval":"anotherval","anotherval":[null,true,false,"anotherval","anotherval","anotherval","anotherval"]}`,
		},
		{
			setTo: []byte{},
			want:  `{"":{"":true,"":false,"":null},"":{"":"","":"","":""},"":"","":[null,true,false,"","","",""]}`,
		},
		{
			setTo: []byte(nil),
			want:  `{"":{"":true,"":false,"":null},"":{"":"","":"","":""},"":"","":[null,true,false,"","","",""]}`,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprint(test.setTo), func(t *testing.T) {
			pj, err := Parse([]byte(input), nil)
			if err != nil {
				t.Errorf("parseMessage failed\n")
				return
			}
			root := pj.Iter()
			// Queue root
			root.AdvanceInto()
			if err != nil {
				t.Errorf("root failed: %v", err)
				return
			}
			iter := root
			for {
				typ := iter.Type()
				switch typ {
				case TypeString, TypeInt, TypeFloat, TypeUint:
					//t.Logf("setting to %v", test.setTo)
					err := iter.SetStringBytes(test.setTo)
					if err != nil {
						t.Errorf("Unable to set value: %v", err)
					}
					val, err := iter.StringBytes()
					if err != nil {
						t.Errorf("Unable to retrieve value: %v", err)
					}

					if !bytes.Equal(val, test.setTo) {
						t.Errorf("Want value %v, got %v", test.setTo, val)
					}
				default:
					err := iter.SetStringBytes(test.setTo)
					if err == nil {
						t.Errorf("Value should not be settable for type %v", typ)
					}
				}
				if iter.PeekNextTag() == TagEnd {
					break
				}
				iter.AdvanceInto()
			}
			out, err := root.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != test.want {
				t.Errorf("want: %s\n got: %s", test.want, string(out))
			}
		})
	}
}

func ExampleIter_FindElement() {
	if !SupportedCPU() {
		// Fake it
		fmt.Println("int\n100 <nil>")
		return
	}
	input := `{
    "Image":
    {
        "Animated": false,
        "Height": 600,
        "IDs":
        [
            116,
            943,
            234,
            38793
        ],
        "Thumbnail":
        {
            "Height": 125,
            "Url": "http://www.example.com/image/481989943",
            "Width": 100
        },
        "Title": "View from 15th Floor",
        "Width": 800
    },
	"Alt": "Image of city" 
}`
	pj, err := Parse([]byte(input), nil)
	if err != nil {
		log.Fatal(err)
	}
	i := pj.Iter()

	// Find element in path.
	elem, err := i.FindElement(nil, "Image", "Thumbnail", "Width")
	if err != nil {
		log.Fatal(err)
	}

	// Print result:
	fmt.Println(elem.Type)
	fmt.Println(elem.Iter.StringCvt())

	// Output:
	// int
	// 100 <nil>
}

func ExampleParsedJson_ForEach() {
	if !SupportedCPU() {
		// Fake results
		fmt.Println("Got iterator for type: object\nFound element: URL Type: string Value: http://example.com/example.gif")
		return
	}

	// Parse JSON:
	pj, err := Parse([]byte(`{"Image":{"URL":"http://example.com/example.gif"}}`), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create an element we can reuse.
	var element *Element
	err = pj.ForEach(func(i Iter) error {
		fmt.Println("Got iterator for type:", i.Type())
		element, err = i.FindElement(element, "Image", "URL")
		if err == nil {
			value, _ := element.Iter.StringCvt()
			fmt.Println("Found element:", element.Name, "Type:", element.Type, "Value:", value)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// Got iterator for type: object
	// Found element: URL Type: string Value: http://example.com/example.gif
}
