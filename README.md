# simdjson-go

## Introduction

This is a Golang port of [simdjson](https://github.com/lemire/simdjson),
a high performance JSON parser developed by Daniel Lemire and Geoff Langdale.
It makes extensive use of SIMD instructions to achieve parsing performance of gigabytes of JSON per second.

Performance wise, `simdjson-go` runs on average at about 40% to 60% of the speed of simdjson.
Compared to Golang's standard package `encoding/json`, `simdjson-go` is about 10x faster.

[![Documentation](https://godoc.org/github.com/minio/simdjson-go?status.svg)](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc)

## Features

`simdjson-go` is a validating parser, meaning that it amongst others validates and checks numerical values, booleans etc.
 Therefore, these values are available as the appropriate `int` and `float64` representations after parsing.

Additionally `simdjson-go` has the following features:

- No 4 GB object limit
- Support for [ndjson](http://ndjson.org/) (newline delimited json)
- Pure Go (no need for cgo)
- Object search/traversal.
- In-place value replacement.
- Remove object/array members.
- Serialize parsed JSONas binary data.
- Re-serialize parts as JSON.

## Requirements

`simdjson-go` has the following requirements for parsing:

A CPU with both AVX2 and CLMUL is required (Haswell from 2013 onwards should do for Intel, for AMD a Ryzen/EPYC CPU (Q1 2017) should be sufficient).
This can be checked using the provided [`SupportedCPU()`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#SupportedCPU`) function.

The package does not provide fallback for unsupported CPUs, but serialized data can be deserialized on an unsupported CPU.

Using the `gccgo` will also always return unsupported CPU since it cannot compile assembly.

## Usage

Run the following command in order to install `simdjson-go`

```bash
go get -u github.com/minio/simdjson-go
```

In order to parse a JSON byte stream, you either call [`simdjson.Parse()`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#Parse)
or [`simdjson.ParseND()`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#ParseND) for newline delimited JSON files.
Both of these functions return a [`ParsedJson`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#ParsedJson)
struct that can be used to navigate the JSON object by calling [`Iter()`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#ParsedJson.Iter).

The easiest use is to call [`ForEach()`]((https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#ParsedJson.ForEach)) function of the returned `ParsedJson`.

```Go
func main() {
	// Parse JSON:
	pj, err := Parse([]byte(`{"Image":{"URL":"http://example.com/example.gif"}}`), nil)
	if err != nil {
		log.Fatal(err)
	}

	// Iterate each top level element.
	_ = pj.ForEach(func(i Iter) error {
		fmt.Println("Got iterator for type:", i.Type())
		element, err := i.FindElement(nil, "Image", "URL")
		if err == nil {
			value, _ := element.Iter.StringCvt()
			fmt.Println("Found element:", element.Name, "Type:", element.Type, "Value:", value)
		}
		return nil
	})

	// Output:
	// Got iterator for type: object
	// Found element: URL Type: string Value: http://example.com/example.gif
}
```

### Parsing with iterators

Using the type [`Iter`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#Iter) you can call
[`Advance()`](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc#Iter.Advance) to iterate over the tape, like so:

```Go
for {
    typ := iter.Advance()

    switch typ {
    case simdjson.TypeRoot:
        if typ, tmp, err = iter.Root(tmp); err != nil {
            return
        }

        if typ == simdjson.TypeObject {
            if obj, err = tmp.Object(obj); err != nil {
                return
            }

            e := obj.FindKey(key, &elem)
            if e != nil && elem.Type == simdjson.TypeString {
                v, _ := elem.Iter.StringBytes()
                fmt.Println(string(v))
            }
        }

    default:
        return
    }
}
```

When you advance the Iter you get the next type currently queued.

Each type then has helpers to access the data. When you get a type you can use these to access the data:

| Type       | Action on Iter             |
|------------|----------------------------|
| TypeNone   | Nothing follows. Iter done |
| TypeNull   | Null value                 |
| TypeString | `String()`/`StringBytes()` |
| TypeInt    | `Int()`/`Float()`          |
| TypeUint   | `Uint()`/`Float()`         |
| TypeFloat  | `Float()`                  |
| TypeBool   | `Bool()`                   |
| TypeObject | `Object()`                 |
| TypeArray  | `Array()`                  |
| TypeRoot   | `Root()`                   |

You can also get the next value as an `interface{}` using the [Interface()](https://pkg.go.dev/github.com/minio/simdjson-go#Iter.Interface) method.

Note that arrays and objects that are null are always returned as `TypeNull`.

The complex types returns helpers that will help parse each of the underlying structures.

It is up to you to keep track of the nesting level you are operating at.

For any `Iter` it is possible to marshal the recursive content of the Iter using
[`MarshalJSON()`](https://pkg.go.dev/github.com/minio/simdjson-go#Iter.MarshalJSON) or
[`MarshalJSONBuffer(...)`](https://pkg.go.dev/github.com/minio/simdjson-go#Iter.MarshalJSONBuffer).

Currently, it is not possible to unmarshal into structs.

### Search by path

It is possible to search by path to find elements by traversing objects.

For example:

```
	// Find element in path.
	elem, err := i.FindElement(nil, "Image", "URL")
```

Will locate the field inside a json object with the following structure:

```
{
    "Image": {
        "URL": "value"
    }
}
```

The values can be any type. The [Element](https://pkg.go.dev/github.com/minio/simdjson-go#Element)
will contain the element information and an Iter to access the content.

## Parsing Objects

If you are only interested in one key in an object you can use `FindKey` to quickly select it.

It is possible to use the `ForEach(fn func(key []byte, i Iter), onlyKeys map[string]struct{})` 
which makes it possible to get a callback for each element in the object. 

An object can be traversed manually by using `NextElement(dst *Iter) (name string, t Type, err error)`.
The key of the element will be returned as a string and the type of the value will be returned
and the provided `Iter` will contain an iterator which will allow access to the content.

There is a `NextElementBytes` which provides the same, but without the need to allocate a string.

All elements of the object can be retrieved using a pretty lightweight [`Parse`](https://pkg.go.dev/github.com/minio/simdjson-go#Object.Parse)
which provides a map of all keys and all elements an a slide.

All elements of the object can be returned as `map[string]interface{}` using the `Map` method on the object.
This will naturally perform allocations for all elements.

## Parsing Arrays

[Arrays](https://pkg.go.dev/github.com/minio/simdjson-go#Array) in JSON can have mixed types.

It is possible to call `ForEach(fn func(i Iter))` to get each element.

To iterate over the array with mixed types use the [`Iter`](https://pkg.go.dev/github.com/minio/simdjson-go#Array.Iter)
method to get an iterator.

There are methods that allow you to retrieve all elements as a single type,
[]int64, []uint64, []float64 and []string with AsInteger(), AsUint64(), AsFloat() and AsString().

## Number parsing

Numbers in JSON are untyped and are returned by the following rules in order:

* If there is any float point notation, like exponents, or a dot notation, it is always returned as float.
* If number is a pure integer and it fits within an int64 it is returned as such.
* If number is a pure positive integer and fits within a uint64 it is returned as such.
* If the number is valid number it is returned as float64.

If the number was converted from integer notation to a float due to not fitting inside int64/uint64
the `FloatOverflowedInteger` flag is set, which can be retrieved using `(Iter).FloatFlags()` method.

JSON numbers follow JavaScript’s double-precision floating-point format.

* Represented in base 10 with no superfluous leading zeros (e.g. 67, 1, 100).
* Include digits between 0 and 9.
* Can be a negative number (e.g. -10).
* Can be a fraction (e.g. .5).
* Can also have an exponent of 10, prefixed by e or E with a plus or minus sign to indicate positive or negative exponentiation.
* Octal and hexadecimal formats are not supported.
* Can not have a value of NaN (Not A Number) or Infinity.

## Parsing NDJSON stream

Newline delimited json is sent as packets with each line being a root element.

Here is an example that counts the number of `"Make": "HOND"` in NDJSON similar to this:

```
{"Age":20, "Make": "HOND"}
{"Age":22, "Make": "TLSA"}
```

```Go
func findHondas(r io.Reader) {
	var nFound int

	// Communication
	reuse := make(chan *simdjson.ParsedJson, 10)
	res := make(chan simdjson.Stream, 10)

	simdjson.ParseNDStream(r, res, reuse)
	// Read results in blocks...
	for got := range res {
		if got.Error != nil {
			if got.Error == io.EOF {
				break
			}
			log.Fatal(got.Error)
		}

		var result int
		var elem *Element
		err := got.Value.ForEach(func(i Iter) error {
			var err error
			elem, err = i.FindElement(elem, "Make")
			if err != nil {
				return nil
			}
			bts, _ := elem.Iter.StringBytes()
			if string(bts) == "HOND" {
				result++
			}
			return nil
		})
		reuse <- got.Value
	}
	fmt.Println("Found", nFound, "Hondas")
}
```

More examples can be found in the examples subdirectory and further documentation can be found at [godoc](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc).


### In-place Value Replacement

It is possible to replace a few, basic internal values.
This means that when re-parsing or re-serializing the parsed JSON these values will be output.

Boolean (true/false) and null values can be freely exchanged.

Numeric values (float, int, uint) can be exchanged freely.

Strings can also be exchanged with different values.

Strings and numbers can be exchanged. However, note that there is no checks for numbers inserted as object keys,
so if used for this invalid JSON is possible.

There is no way to modify objects, arrays, other than value types above inside each.
It is not possible to remove or add elements.

To replace a value, of value referenced by an `Iter` simply call `SetNull`, `SetBool`, `SetFloat`, `SetInt`, `SetUInt`,
`SetString` or `SetStringBytes`.

### Object & Array Element Deletion

It is possible to delete one or more elements in an object.

`(*Object).DeleteElems(fn, onlyKeys)` will call back fn for each key+ value.

If true is returned, the key+value is deleted. A key filter can be provided for optional filtering.
If the callback function is nil all elements matching the filter will be deleted.
If both are nil all elements are deleted.

Example:

```Go
	// The object we are modifying
	var obj *simdjson.Object

	// Delete all entries where the key is "unwanted":
	err = obj.DeleteElems(func(key []byte, i Iter) bool {
		return string(key) == "unwanted")
	}, nil)

	// Alternative version with prefiltered keys:
	err = obj.DeleteElems(nil, map[string]struct{}{"unwanted": {}})
```

`(*Array).DeleteElems(fn func(i Iter) bool)` will call back fn for each array value.
If the function returns true the element is deleted in the array.

```Go
	// The array we are modifying
	var array *simdjson.Array

	// Delete all entries that are strings.
	array.DeleteElems(func(i Iter) bool {
		return i.Type() == TypeString
	})
```

## Serializing parsed json

It is possible to serialize parsed JSON for more compact storage and faster load time.

To create a new serialized use [NewSerializer](https://pkg.go.dev/github.com/minio/simdjson-go#NewSerializer).
This serializer can be reused for several JSON blocks.

The serializer will provide string deduplication and compression of elements.
This can be finetuned using the [`CompressMode`](https://pkg.go.dev/github.com/minio/simdjson-go#Serializer.CompressMode) setting.

To serialize a block of parsed data use the [`Serialize`](https://pkg.go.dev/github.com/minio/simdjson-go#Serializer.Serialize) method.

To read back use the [`Deserialize`](https://pkg.go.dev/github.com/minio/simdjson-go#Serializer.Deserialize) method.
For deserializing the compression mode does not need to match since it is read from the stream.

Example of speed for serializer/deserializer on [`parking-citations-1M`](https://dl.minio.io/assets/parking-citations-1M.json.zst).

| Compress Mode | % of JSON size | Serialize Speed | Deserialize Speed |
|---------------|----------------|-----------------|-------------------|
| None          | 177.26%        | 425.70 MB/s     | 2334.33 MB/s      |
| Fast          | 17.20%         | 412.75 MB/s     | 1234.76 MB/s      |
| Default       | 16.85%         | 411.59 MB/s     | 1242.09 MB/s      |
| Best          | 10.91%         | 337.17 MB/s     | 806.23 MB/s       |

In some cases the speed difference and compression difference will be bigger.

## Performance vs `encoding/json` and `json-iterator/go`

Though simdjson provides different output than traditional unmarshal functions this can give
an overview of the expected performance for reading specific data in JSON.

Below is a performance comparison to Golang's standard package `encoding/json` based on the same set of JSON test files, unmarshal to `interface{}`.

Comparisons with default settings:

```
λ benchcmp enc-json.txt simdjson.txt
benchmark                      old ns/op     new ns/op     delta
BenchmarkApache_builds-32      1219080       142972        -88.27%
BenchmarkCanada-32             38362219      13417193      -65.02%
BenchmarkCitm_catalog-32       17051899      1359983       -92.02%
BenchmarkGithub_events-32      603037        74042         -87.72%
BenchmarkGsoc_2018-32          20777333      1259171       -93.94%
BenchmarkInstruments-32        2626808       301370        -88.53%
BenchmarkMarine_ik-32          56630295      14419901      -74.54%
BenchmarkMesh-32               13411486      4206251       -68.64%
BenchmarkMesh_pretty-32        18226803      4786081       -73.74%
BenchmarkNumbers-32            2131951       909641        -57.33%
BenchmarkRandom-32             7360966       1004387       -86.36%
BenchmarkTwitter-32            6635848       588773        -91.13%
BenchmarkTwitterescaped-32     6292856       972250        -84.55%
BenchmarkUpdate_center-32      6396501       708717        -88.92%

benchmark                      old MB/s     new MB/s     speedup
BenchmarkApache_builds-32      104.40       890.21       8.53x
BenchmarkCanada-32             58.68        167.77       2.86x
BenchmarkCitm_catalog-32       101.29       1270.02      12.54x
BenchmarkGithub_events-32      108.01       879.67       8.14x
BenchmarkGsoc_2018-32          160.17       2642.88      16.50x
BenchmarkInstruments-32        83.88        731.15       8.72x
BenchmarkMarine_ik-32          52.68        206.90       3.93x
BenchmarkMesh-32               53.95        172.03       3.19x
BenchmarkMesh_pretty-32        86.54        329.57       3.81x
BenchmarkNumbers-32            70.42        165.04       2.34x
BenchmarkRandom-32             69.35        508.25       7.33x
BenchmarkTwitter-32            95.17        1072.59      11.27x
BenchmarkTwitterescaped-32     89.37        578.46       6.47x
BenchmarkUpdate_center-32      83.35        752.31       9.03x

benchmark                      old allocs     new allocs     delta
BenchmarkApache_builds-32      9716           22             -99.77%
BenchmarkCanada-32             392535         250            -99.94%
BenchmarkCitm_catalog-32       95372          110            -99.88%
BenchmarkGithub_events-32      3328           17             -99.49%
BenchmarkGsoc_2018-32          58615          67             -99.89%
BenchmarkInstruments-32        13336          33             -99.75%
BenchmarkMarine_ik-32          614776         467            -99.92%
BenchmarkMesh-32               149504         122            -99.92%
BenchmarkMesh_pretty-32        149504         122            -99.92%
BenchmarkNumbers-32            20025          28             -99.86%
BenchmarkRandom-32             66083          76             -99.88%
BenchmarkTwitter-32            31261          53             -99.83%
BenchmarkTwitterescaped-32     31757          53             -99.83%
BenchmarkUpdate_center-32      49074          58             -99.88%

benchmark                      old bytes     new bytes     delta
BenchmarkApache_builds-32      461556        965           -99.79%
BenchmarkCanada-32             10943847      39793         -99.64%
BenchmarkCitm_catalog-32       5122732       6089          -99.88%
BenchmarkGithub_events-32      186148        802           -99.57%
BenchmarkGsoc_2018-32          7032092       17215         -99.76%
BenchmarkInstruments-32        882265        1310          -99.85%
BenchmarkMarine_ik-32          22564413      189870        -99.16%
BenchmarkMesh-32               7130934       15483         -99.78%
BenchmarkMesh_pretty-32        7288661       12066         -99.83%
BenchmarkNumbers-32            1066304       1280          -99.88%
BenchmarkRandom-32             2787054       4096          -99.85%
BenchmarkTwitter-32            2152260       2550          -99.88%
BenchmarkTwitterescaped-32     2330548       3062          -99.87%
BenchmarkUpdate_center-32      2729631       3235          -99.88%
```

Here is another benchmark comparison to `json-iterator/go`, unmarshal to `interface{}`.

```
λ benchcmp jsiter.txt simdjson.txt
benchmark                      old ns/op     new ns/op     delta
BenchmarkApache_builds-32      891370        142972        -83.96%
BenchmarkCanada-32             52365386      13417193      -74.38%
BenchmarkCitm_catalog-32       10154544      1359983       -86.61%
BenchmarkGithub_events-32      398741        74042         -81.43%
BenchmarkGsoc_2018-32          15584278      1259171       -91.92%
BenchmarkInstruments-32        1858339       301370        -83.78%
BenchmarkMarine_ik-32          49881479      14419901      -71.09%
BenchmarkMesh-32               15038300      4206251       -72.03%
BenchmarkMesh_pretty-32        17655583      4786081       -72.89%
BenchmarkNumbers-32            2903165       909641        -68.67%
BenchmarkRandom-32             6156849       1004387       -83.69%
BenchmarkTwitter-32            4655981       588773        -87.35%
BenchmarkTwitterescaped-32     5521004       972250        -82.39%
BenchmarkUpdate_center-32      5540200       708717        -87.21%

benchmark                      old MB/s     new MB/s     speedup
BenchmarkApache_builds-32      142.79       890.21       6.23x
BenchmarkCanada-32             42.99        167.77       3.90x
BenchmarkCitm_catalog-32       170.09       1270.02      7.47x
BenchmarkGithub_events-32      163.34       879.67       5.39x
BenchmarkGsoc_2018-32          213.54       2642.88      12.38x
BenchmarkInstruments-32        118.57       731.15       6.17x
BenchmarkMarine_ik-32          59.81        206.90       3.46x
BenchmarkMesh-32               48.12        172.03       3.58x
BenchmarkMesh_pretty-32        89.34        329.57       3.69x
BenchmarkNumbers-32            51.71        165.04       3.19x
BenchmarkRandom-32             82.91        508.25       6.13x
BenchmarkTwitter-32            135.64       1072.59      7.91x
BenchmarkTwitterescaped-32     101.87       578.46       5.68x
BenchmarkUpdate_center-32      96.24        752.31       7.82x

benchmark                      old allocs     new allocs     delta
BenchmarkApache_builds-32      13248          22             -99.83%
BenchmarkCanada-32             665988         250            -99.96%
BenchmarkCitm_catalog-32       118755         110            -99.91%
BenchmarkGithub_events-32      4442           17             -99.62%
BenchmarkGsoc_2018-32          90915          67             -99.93%
BenchmarkInstruments-32        18776          33             -99.82%
BenchmarkMarine_ik-32          692512         467            -99.93%
BenchmarkMesh-32               184137         122            -99.93%
BenchmarkMesh_pretty-32        204037         122            -99.94%
BenchmarkNumbers-32            30037          28             -99.91%
BenchmarkRandom-32             88091          76             -99.91%
BenchmarkTwitter-32            45040          53             -99.88%
BenchmarkTwitterescaped-32     47198          53             -99.89%
BenchmarkUpdate_center-32      66757          58             -99.91%

benchmark                      old bytes     new bytes     delta
BenchmarkApache_builds-32      518350        965           -99.81%
BenchmarkCanada-32             16189358      39793         -99.75%
BenchmarkCitm_catalog-32       5571982       6089          -99.89%
BenchmarkGithub_events-32      221631        802           -99.64%
BenchmarkGsoc_2018-32          11771591      17215         -99.85%
BenchmarkInstruments-32        991674        1310          -99.87%
BenchmarkMarine_ik-32          25257277      189870        -99.25%
BenchmarkMesh-32               7991707       15483         -99.81%
BenchmarkMesh_pretty-32        8628570       12066         -99.86%
BenchmarkNumbers-32            1226518       1280          -99.90%
BenchmarkRandom-32             3167528       4096          -99.87%
BenchmarkTwitter-32            2426730       2550          -99.89%
BenchmarkTwitterescaped-32     2607198       3062          -99.88%
BenchmarkUpdate_center-32      3052382       3235          -99.89%
```


### Inplace strings

The best performance is obtained by keeping the JSON message fully mapped in memory and using the
`WithCopyStrings(false)` option. This prevents duplicate copies of string values being made
but mandates that the original JSON buffer is kept alive until the `ParsedJson` object is no longer needed
(ie iteration over the tape format has been completed).

In case the JSON message buffer is freed earlier (or for streaming use cases where memory is reused)
`WithCopyStrings(true)` should be used (which is the default behaviour).

The performance impact differs based on the input type, but this is the general differences:

```
BenchmarkApache_builds/copy-32                	    8242	    142972 ns/op	 890.21 MB/s	     965 B/op	      22 allocs/op
BenchmarkApache_builds/nocopy-32              	   10000	    111189 ns/op	1144.68 MB/s	     932 B/op	      22 allocs/op

BenchmarkCanada/copy-32                       	      91	  13417193 ns/op	 167.77 MB/s	   39793 B/op	     250 allocs/op
BenchmarkCanada/nocopy-32                     	      87	  13392401 ns/op	 168.08 MB/s	   41334 B/op	     250 allocs/op

BenchmarkCitm_catalog/copy-32                 	     889	   1359983 ns/op	1270.02 MB/s	    6089 B/op	     110 allocs/op
BenchmarkCitm_catalog/nocopy-32               	     924	   1268470 ns/op	1361.64 MB/s	    5582 B/op	     110 allocs/op

BenchmarkGithub_events/copy-32                	   16092	     74042 ns/op	 879.67 MB/s	     802 B/op	      17 allocs/op
BenchmarkGithub_events/nocopy-32              	   19446	     62143 ns/op	1048.10 MB/s	     794 B/op	      17 allocs/op

BenchmarkGsoc_2018/copy-32                    	     948	   1259171 ns/op	2642.88 MB/s	   17215 B/op	      67 allocs/op
BenchmarkGsoc_2018/nocopy-32                  	    1144	   1040864 ns/op	3197.18 MB/s	    9947 B/op	      67 allocs/op

BenchmarkInstruments/copy-32                  	    3932	    301370 ns/op	 731.15 MB/s	    1310 B/op	      33 allocs/op
BenchmarkInstruments/nocopy-32                	    4443	    271500 ns/op	 811.59 MB/s	    1258 B/op	      33 allocs/op

BenchmarkMarine_ik/copy-32                    	      79	  14419901 ns/op	 206.90 MB/s	  189870 B/op	     467 allocs/op
BenchmarkMarine_ik/nocopy-32                  	      79	  14176758 ns/op	 210.45 MB/s	  189867 B/op	     467 allocs/op

BenchmarkMesh/copy-32                         	     288	   4206251 ns/op	 172.03 MB/s	   15483 B/op	     122 allocs/op
BenchmarkMesh/nocopy-32                       	     285	   4207299 ns/op	 171.99 MB/s	   15615 B/op	     122 allocs/op

BenchmarkMesh_pretty/copy-32                  	     248	   4786081 ns/op	 329.57 MB/s	   12066 B/op	     122 allocs/op
BenchmarkMesh_pretty/nocopy-32                	     250	   4803647 ns/op	 328.37 MB/s	   12009 B/op	     122 allocs/op

BenchmarkNumbers/copy-32                      	    1336	    909641 ns/op	 165.04 MB/s	    1280 B/op	      28 allocs/op
BenchmarkNumbers/nocopy-32                    	    1321	    910493 ns/op	 164.88 MB/s	    1281 B/op	      28 allocs/op

BenchmarkRandom/copy-32                       	    1201	   1004387 ns/op	 508.25 MB/s	    4096 B/op	      76 allocs/op
BenchmarkRandom/nocopy-32                     	    1554	    773142 ns/op	 660.26 MB/s	    3198 B/op	      76 allocs/op

BenchmarkTwitter/copy-32                      	    2035	    588773 ns/op	1072.59 MB/s	    2550 B/op	      53 allocs/op
BenchmarkTwitter/nocopy-32                    	    2485	    475949 ns/op	1326.85 MB/s	    2029 B/op	      53 allocs/op

BenchmarkTwitterescaped/copy-32               	    1189	    972250 ns/op	 578.46 MB/s	    3062 B/op	      53 allocs/op
BenchmarkTwitterescaped/nocopy-32             	    1372	    874972 ns/op	 642.77 MB/s	    2518 B/op	      53 allocs/op

BenchmarkUpdate_center/copy-32                	    1665	    708717 ns/op	 752.31 MB/s	    3235 B/op	      58 allocs/op
BenchmarkUpdate_center/nocopy-32              	    2241	    536027 ns/op	 994.68 MB/s	    2130 B/op	      58 allocs/op
```

## Design

`simdjson-go` follows the same two stage design as `simdjson`.
During the first stage the structural elements (`{`, `}`, `[`, `]`, `:`, and `,`)
are detected and forwarded as offsets in the message buffer to the second stage.
The second stage builds a tape format of the structure of the JSON document.

Note that in contrast to `simdjson`, `simdjson-go` outputs `uint32`
increments (as opposed to absolute values) to the second stage.
This allows arbitrarily large JSON files to be parsed (as long as a single (string) element does not surpass 4 GB...).

Also, for better performance,
both stages run concurrently as separate go routines and a go channel is used to communicate between the two stages.

### Stage 1

Stage 1 has been converted from the original C code (containing the SIMD intrinsics) to Golang assembly using [c2goasm](https://github.com/minio/c2goasm).
It essentially consists of five separate steps, being:

- `find_odd_backslash_sequences`: detect backslash characters used to escape quotes
- `find_quote_mask_and_bits`: generate a mask with bits turned on for characters between quotes
- `find_whitespace_and_structurals`: generate a mask for whitespace plus a mask for the structural characters
- `finalize_structurals`: combine the masks computed above into a final mask where each active bit represents the position of a structural character in the input message.
- `flatten_bits_incremental`: output the active bits in the final mask as incremental offsets.

For more details you can take a look at the various test cases in `find_subroutines_amd64_test.go` to see how
the individual routines can be invoked (typically with a 64 byte input buffer that generates one or more 64-bit masks).

There is one final routine, `find_structural_bits_in_slice`, that ties it all together and is
invoked with a slice of the message buffer in order to find the incremental offsets.

### Stage 2

During Stage 2 the tape structure is constructed.
It is essentially a single function that jumps around as it finds the various structural characters
and builds the hierarchy of the JSON document that it processes.
The values of the JSON elements such as strings, integers, booleans etc. are parsed and written to the tape.

Any errors (such as an array not being closed or a missing closing brace) are detected and reported back as errors to the client.

## Tape format

Similarly to `simdjson`, `simdjson-go` parses the structure onto a 'tape' format.
With this format it is possible to skip over arrays and (sub)objects as the sizes are recorded in the tape.

`simdjson-go` format is exactly the same as the `simdjson` [tape](https://github.com/lemire/simdjson/blob/master/doc/tape.md)
format with the following 2 exceptions:

- In order to support ndjson, it is possible to have more than one root element on the tape.
Also, to allow for fast navigation over root elements, a root points to the next root element
(and as such the last root element points 1 index past the length of the tape).

A "NOP" tag is added. The value contains the number of tape entries to skip forward for next tag.

- Strings are handled differently, unlike `simdjson` the string size is not prepended in the String buffer
but is added as an additional element to the tape itself (much like integers and floats).
  - In case `WithCopyStrings(false)` Only strings that contain special characters are copied to the String buffer
in which case the payload from the tape is the offset into the String buffer.
For string values without special characters the tape's payload points directly into the message buffer.
  - In case `WithCopyStrings(true)` (default): Strings are always copied to the String buffer.

For more information, see `TestStage2BuildTape` in `stage2_build_tape_test.go`.

## Fuzz Tests

`simdjson-go` has been extensively fuzz tested to ensure that input cannot generate crashes and that output matches
the standard library.

The fuzz tests are included as Go 1.18+ compatible tests.

## License

`simdjson-go` is released under the Apache License v2.0. You can find the complete text in the file LICENSE.

## Contributing

Contributions are welcome, please send PRs for any enhancements.

If your PR include parsing changes please run fuzz testers for a couple of hours.
