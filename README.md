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
 Therefore these values are available as the appropriate `int` and `float64` representations after parsing.

Additionally `simdjson-go` has the following features:

- No 4 GB object limit
- Support for [ndjson](http://ndjson.org/) (newline delimited json)
- Pure Go (no need for cgo)

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

## Parsing Objects

If you are only interested in one key in an object you can use `FindKey` to quickly select it.

An object kan be traversed manually by using `NextElement(dst *Iter) (name string, t Type, err error)`.
The key of the element will be returned as a string and the type of the value will be returned
and the provided `Iter` will contain an iterator which will allow access to the content.

There is a `NextElementBytes` which provides the same, but without the need to allocate a string.

All elements of the object can be retrieved using a pretty lightweight [`Parse`](https://pkg.go.dev/github.com/minio/simdjson-go#Object.Parse)
which provides a map of all keys and all elements an a slide.

All elements of the object can be returned as `map[string]interface{}` using the `Map` method on the object.
This will naturally perform allocations for all elements.

## Parsing Arrays

[Arrays](https://pkg.go.dev/github.com/minio/simdjson-go#Array) in JSON can have mixed types. 
To iterate over the array with mixed types use the [`Iter`](https://pkg.go.dev/github.com/minio/simdjson-go#Array.Iter) 
method to get an iterator.

There are methods that allow you to retrieve all elements as a single type, 
[]int64, []uint64, float64 and strings.  

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

## Parsing NDSJON stream

Newline delimited json is sent as packets with each line being a root element.

Here is an example that counts the number of `"Make": "HOND"` in NDSJON similar to this:

```
{"Age":20, "Make": "HOND"}
{"Age":22, "Make": "TLSA"}
```

```Go
func findHondas(r io.Reader) {
	// Temp values.
	var tmpO simdjson.Object{}
	var tmpE simdjson.Element{}
	var tmpI simdjson.Iter
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

		all := got.Value.Iter()
		// NDJSON is a separated by root objects.
		for all.Advance() == simdjson.TypeRoot {
			// Read inside root.
			t, i, err := all.Root(&tmpI)
			if t != simdjson.TypeObject {
				log.Println("got type", t.String())
				continue
			}

			// Prepare object.
			obj, err := i.Object(&tmpO)
			if err != nil {
				log.Println("got err", err)
				continue
			}

			// Find Make key.
			elem := obj.FindKey("Make", &tmpE)
			if elem.Type != TypeString {
				log.Println("got type", err)
				continue
			}
			
			// Get value as bytes.
			asB, err := elem.Iter.StringBytes()
			if err != nil {
				log.Println("got err", err)
				continue
			}
			if bytes.Equal(asB, []byte("HOND")) {
				nFound++
			}
		}
		reuse <- got.Value
	}
	fmt.Println("Found", nFound, "Hondas")
}
```

More examples can be found in the examples subdirectory and further documentation can be found at [godoc](https://pkg.go.dev/github.com/minio/simdjson-go?tab=doc). 

## Serializing parsed json

It is possible to serialize parsed JSON for more compact storage and faster load time.

To create a new serialized use [NewSerializer](https://pkg.go.dev/github.com/minio/simdjson-go#NewSerializer).
This serializer can be reused for several JSON blocks.

The serializer will provide string deduplication and compression of elements. 
This can be finetuned using the [`CompressMode`](https://pkg.go.dev/github.com/minio/simdjson-go#Serializer.CompressMode) setting.

To serialize a block of parsed data use the [`Serialize`](https://pkg.go.dev/github.com/minio/simdjson-go#Serializer.Serialize) method.

To read back use the [`Deserialize`](https://pkg.go.dev/github.com/minio/simdjson-go#Serializer.Deserialize) method.
For deserializing the compression mode does not need to match since it is read from the stream. 

Example of speed for serializer/deserializer on [`parking-citations-1M`](https://files.klauspost.com/compress/parking-citations-1M.json.zst).

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

Below is a performance comparison to Golang's standard package `encoding/json` based on the same set of JSON test files.

Comparisons with default settings:

```
λ benchcmp enc-json.txt simdjson.txt
benchmark                      old ns/op     new ns/op     delta
BenchmarkApache_builds-32      1230661       139291        -88.68%
BenchmarkCanada-32             38488323      15586291      -59.50%
BenchmarkCitm_catalog-32       17024796      1929255       -88.67%
BenchmarkGithub_events-32      597174        75786         -87.31%
BenchmarkGsoc_2018-32          20571947      1276244       -93.80%
BenchmarkInstruments-32        2641314       351849        -86.68%
BenchmarkMarine_ik-32          55671538      18104424      -67.48%
BenchmarkMesh-32               13299865      5420953       -59.24%
BenchmarkMesh_pretty-32        18132769      6190117       -65.86%
BenchmarkNumbers-32            2110372       1071121       -49.24%
BenchmarkRandom-32             7253270       1042823       -85.62%
BenchmarkTwitter-32            6593341       657462        -90.03%
BenchmarkTwitterescaped-32     6253261       1041771       -83.34%
BenchmarkUpdate_center-32      6424612       714683        -88.88%

benchmark                      old MB/s     new MB/s     speedup
BenchmarkApache_builds-32      103.42       913.73       8.84x
BenchmarkCanada-32             58.49        144.43       2.47x
BenchmarkCitm_catalog-32       101.45       895.27       8.82x
BenchmarkGithub_events-32      109.07       859.42       7.88x
BenchmarkGsoc_2018-32          161.77       2607.52      16.12x
BenchmarkInstruments-32        83.42        626.25       7.51x
BenchmarkMarine_ik-32          53.59        164.79       3.08x
BenchmarkMesh-32               54.41        133.48       2.45x
BenchmarkMesh_pretty-32        86.99        254.82       2.93x
BenchmarkNumbers-32            71.14        140.16       1.97x
BenchmarkRandom-32             70.38        489.51       6.96x
BenchmarkTwitter-32            95.78        960.53       10.03x
BenchmarkTwitterescaped-32     89.94        539.86       6.00x
BenchmarkUpdate_center-32      82.99        746.03       8.99x

benchmark                      old allocs     new allocs     delta
BenchmarkApache_builds-32      9717           22             -99.77%
BenchmarkCanada-32             392536         111376         -71.63%
BenchmarkCitm_catalog-32       95373          14502          -84.79%
BenchmarkGithub_events-32      3329           120            -96.40%
BenchmarkGsoc_2018-32          58616          67             -99.89%
BenchmarkInstruments-32        13336          1793           -86.56%
BenchmarkMarine_ik-32          614777         206678         -66.38%
BenchmarkMesh-32               149505         69441          -53.55%
BenchmarkMesh_pretty-32        149505         69441          -53.55%
BenchmarkNumbers-32            20026          10029          -49.92%
BenchmarkRandom-32             66086          2068           -96.87%
BenchmarkTwitter-32            31261          1564           -95.00%
BenchmarkTwitterescaped-32     31759          1564           -95.08%
BenchmarkUpdate_center-32      49074          58             -99.88%

benchmark                      old bytes     new bytes     delta
BenchmarkApache_builds-32      461568        965           -99.79%
BenchmarkCanada-32             10943859      2690611       -75.41%
BenchmarkCitm_catalog-32       5122770       223104        -95.64%
BenchmarkGithub_events-32      186180        1603          -99.14%
BenchmarkGsoc_2018-32          7032094       17507         -99.75%
BenchmarkInstruments-32        882029        6270          -99.29%
BenchmarkMarine_ik-32          22564451      1712188       -92.41%
BenchmarkMesh-32               7130947       658553        -90.76%
BenchmarkMesh_pretty-32        7288675       811955        -88.86%
BenchmarkNumbers-32            1066320       161464        -84.86%
BenchmarkRandom-32             2787496       10326         -99.63%
BenchmarkTwitter-32            2151770       14243         -99.34%
BenchmarkTwitterescaped-32     2330825       14633         -99.37%
BenchmarkUpdate_center-32      2729372       3237          -99.88% 
```

Here is another benchmark comparison to `json-iterator/go`:

```
λ benchcmp jsiter.txt simdjson.txt
benchmark                      old ns/op     new ns/op     delta
BenchmarkApache_builds-32      890804        139291        -84.36%
BenchmarkCanada-32             51905191      15586291      -69.97%
BenchmarkCitm_catalog-32       10027797      1929255       -80.76%
BenchmarkGithub_events-32      400636        75786         -81.08%
BenchmarkGsoc_2018-32          15434726      1276244       -91.73%
BenchmarkInstruments-32        1862559       351849        -81.11%
BenchmarkMarine_ik-32          48921744      18104424      -62.99%
BenchmarkMesh-32               14660768      5420953       -63.02%
BenchmarkMesh_pretty-32        17397735      6190117       -64.42%
BenchmarkNumbers-32            2876574       1071121       -62.76%
BenchmarkRandom-32             6040910       1042823       -82.74%
BenchmarkTwitter-32            4632070       657462        -85.81%
BenchmarkTwitterescaped-32     5458215       1041771       -80.91%
BenchmarkUpdate_center-32      5441402       714683        -86.87%

benchmark                      old MB/s     new MB/s     speedup
BenchmarkApache_builds-32      142.88       913.73       6.40x
BenchmarkCanada-32             43.37        144.43       3.33x
BenchmarkCitm_catalog-32       172.24       895.27       5.20x
BenchmarkGithub_events-32      162.57       859.42       5.29x
BenchmarkGsoc_2018-32          215.61       2607.52      12.09x
BenchmarkInstruments-32        118.30       626.25       5.29x
BenchmarkMarine_ik-32          60.98        164.79       2.70x
BenchmarkMesh-32               49.36        133.48       2.70x
BenchmarkMesh_pretty-32        90.66        254.82       2.81x
BenchmarkNumbers-32            52.19        140.16       2.69x
BenchmarkRandom-32             84.50        489.51       5.79x
BenchmarkTwitter-32            136.34       960.53       7.05x
BenchmarkTwitterescaped-32     103.04       539.86       5.24x
BenchmarkUpdate_center-32      97.99        746.03       7.61x

benchmark                      old allocs     new allocs     delta
BenchmarkApache_builds-32      13249          22             -99.83%
BenchmarkCanada-32             665989         111376         -83.28%
BenchmarkCitm_catalog-32       118756         14502          -87.79%
BenchmarkGithub_events-32      4443           120            -97.30%
BenchmarkGsoc_2018-32          90916          67             -99.93%
BenchmarkInstruments-32        18777          1793           -90.45%
BenchmarkMarine_ik-32          692513         206678         -70.16%
BenchmarkMesh-32               184138         69441          -62.29%
BenchmarkMesh_pretty-32        204038         69441          -65.97%
BenchmarkNumbers-32            30038          10029          -66.61%
BenchmarkRandom-32             88092          2068           -97.65%
BenchmarkTwitter-32            45042          1564           -96.53%
BenchmarkTwitterescaped-32     47201          1564           -96.69%
BenchmarkUpdate_center-32      66759          58             -99.91%

benchmark                      old bytes     new bytes     delta
BenchmarkApache_builds-32      518361        965           -99.81%
BenchmarkCanada-32             16189365      2690611       -83.38%
BenchmarkCitm_catalog-32       5571965       223104        -96.00%
BenchmarkGithub_events-32      221617        1603          -99.28%
BenchmarkGsoc_2018-32          11771542      17507         -99.85%
BenchmarkInstruments-32        991753        6270          -99.37%
BenchmarkMarine_ik-32          25257289      1712188       -93.22%
BenchmarkMesh-32               7991695       658553        -91.76%
BenchmarkMesh_pretty-32        8628524       811955        -90.59%
BenchmarkNumbers-32            1226522       161464        -86.84%
BenchmarkRandom-32             3167547       10326         -99.67%
BenchmarkTwitter-32            2427192       14243         -99.41%
BenchmarkTwitterescaped-32     2607679       14633         -99.44%
BenchmarkUpdate_center-32      3052780       3237          -99.89% 
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
BenchmarkApache_builds/copy-32                	    8295	    139291 ns/op	 913.73 MB/s	     965 B/op	      22 allocs/op
BenchmarkApache_builds/nocopy-32              	   10000	    103805 ns/op	1226.10 MB/s	     931 B/op	      22 allocs/op

BenchmarkCanada/copy-32                       	      76	  15586291 ns/op	 144.43 MB/s	 2690611 B/op	  111376 allocs/op
BenchmarkCanada/nocopy-32                     	      74	  15626266 ns/op	 144.06 MB/s	 2691687 B/op	  111376 allocs/op

BenchmarkCitm_catalog/copy-32                 	     631	   1929255 ns/op	 895.27 MB/s	  223104 B/op	   14502 allocs/op
BenchmarkCitm_catalog/nocopy-32               	     691	   1712232 ns/op	1008.74 MB/s	  222215 B/op	   14502 allocs/op

BenchmarkGithub_events/copy-32                	   15157	     75786 ns/op	 859.42 MB/s	    1603 B/op	     120 allocs/op
BenchmarkGithub_events/nocopy-32              	   18826	     65453 ns/op	 995.09 MB/s	    1594 B/op	     120 allocs/op

BenchmarkGsoc_2018/copy-32                    	     930	   1276244 ns/op	2607.52 MB/s	   17507 B/op	      67 allocs/op
BenchmarkGsoc_2018/nocopy-32                  	    1119	   1034831 ns/op	3215.82 MB/s	   10126 B/op	      67 allocs/op

BenchmarkInstruments/copy-32                  	    3426	    351849 ns/op	 626.25 MB/s	    6270 B/op	    1793 allocs/op
BenchmarkInstruments/nocopy-32                	    3994	    305460 ns/op	 721.36 MB/s	    6213 B/op	    1793 allocs/op

BenchmarkMarine_ik/copy-32                    	      66	  18104424 ns/op	 164.79 MB/s	 1712188 B/op	  206678 allocs/op
BenchmarkMarine_ik/nocopy-32                  	      66	  17672827 ns/op	 168.82 MB/s	 1712195 B/op	  206678 allocs/op

BenchmarkMesh/copy-32                         	     220	   5420953 ns/op	 133.48 MB/s	  658553 B/op	   69441 allocs/op
BenchmarkMesh/nocopy-32                       	     220	   5436766 ns/op	 133.09 MB/s	  658556 B/op	   69441 allocs/op

BenchmarkMesh_pretty/copy-32                  	     194	   6190117 ns/op	 254.82 MB/s	  811955 B/op	   69441 allocs/op
BenchmarkMesh_pretty/nocopy-32                	     196	   6175009 ns/op	 255.44 MB/s	  811863 B/op	   69441 allocs/op

BenchmarkNumbers/copy-32                      	    1105	   1071121 ns/op	 140.16 MB/s	  161464 B/op	   10029 allocs/op
BenchmarkNumbers/nocopy-32                    	    1124	   1073658 ns/op	 139.82 MB/s	  161440 B/op	   10029 allocs/op

BenchmarkRandom/copy-32                       	    1156	   1042823 ns/op	 489.51 MB/s	   10326 B/op	    2068 allocs/op
BenchmarkRandom/nocopy-32                     	    1482	    819914 ns/op	 622.60 MB/s	    9401 B/op	    2068 allocs/op

BenchmarkTwitter/copy-32                      	    1801	    657462 ns/op	 960.53 MB/s	   14243 B/op	    1564 allocs/op
BenchmarkTwitter/nocopy-32                    	    2301	    527717 ns/op	1196.69 MB/s	   13633 B/op	    1564 allocs/op

BenchmarkTwitterescaped/copy-32               	    1170	   1041771 ns/op	 539.86 MB/s	   14633 B/op	    1564 allocs/op
BenchmarkTwitterescaped/nocopy-32             	    1300	    912331 ns/op	 616.45 MB/s	   14139 B/op	    1564 allocs/op

BenchmarkUpdate_center/copy-32                	    1663	    714683 ns/op	 746.03 MB/s	    3237 B/op	      58 allocs/op
BenchmarkUpdate_center/nocopy-32              	    2348	    514878 ns/op	1035.54 MB/s	    2114 B/op	      58 allocs/op
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

The fuzzers and corpus are contained in a separate repository at [github.com/minio/simdjson-fuzz](https://github.com/minio/simdjson-fuzz)

The repo contains information on how to run them.

## License

`simdjson-go` is released under the Apache License v2.0. You can find the complete text in the file LICENSE.

## Contributing

Contributions are welcome, please send PRs for any enhancements.

If your PR include parsing changes please run fuzz testers for a couple of hours.
