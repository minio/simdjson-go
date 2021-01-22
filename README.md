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

## Performance vs simdjson

Based on the same set of JSON test files, the graph below shows a comparison between `simdjson` and `simdjson-go`.

![simdjson-vs-go-comparison](chart/simdjson-vs-simdjson-go.png)

These numbers were measured on a MacBook Pro equipped with a 3.1 GHz Intel Core i7.
Also, to make it a fair comparison, the constant `GOLANG_NUMBER_PARSING` was set to `false` (default is `true`)
in order to use the same number parsing function (which is faster at the expense of some precision; see more below).

In addition the constant `ALWAYS_COPY_STRINGS` was set to `false` (default is `true`) for non-streaming use case
scenarios where the full JSON message is kept in memory (similar to the `simdjson` behaviour).

## Performance vs `encoding/json` and `json-iterator/go`

Below is a performance comparison to Golang's standard package `encoding/json` based on the same set of JSON test files.

```
$ benchcmp                    encoding_json.txt      simdjson-go.txt
benchmark                     old MB/s               new MB/s         speedup
BenchmarkApache_builds-8      106.77                  948.75           8.89x
BenchmarkCanada-8              54.39                  519.85           9.56x
BenchmarkCitm_catalog-8       100.44                 1565.28          15.58x
BenchmarkGithub_events-8      159.49                  848.88           5.32x
BenchmarkGsoc_2018-8          152.93                 2515.59          16.45x
BenchmarkInstruments-8         82.82                  811.61           9.80x
BenchmarkMarine_ik-8           48.12                  422.43           8.78x
BenchmarkMesh-8                49.38                  371.39           7.52x
BenchmarkMesh_pretty-8         73.10                  784.89          10.74x
BenchmarkNumbers-8            160.69                  434.85           2.71x
BenchmarkRandom-8              66.56                  615.12           9.24x
BenchmarkTwitter-8             79.05                 1193.47          15.10x
BenchmarkTwitterescaped-8      83.96                  536.19           6.39x
BenchmarkUpdate_center-8       73.92                  860.52          11.64x
```

Also `simdjson-go` uses less additional memory and allocations.

Here is another benchmark comparison to `json-iterator/go`:

```
$ benchcmp                    json-iterator.txt      simdjson-go.txt
benchmark                     old MB/s               new MB/s         speedup
BenchmarkApache_builds-8      154.65                  948.75           6.13x
BenchmarkCanada-8              40.34                  519.85          12.89x
BenchmarkCitm_catalog-8       183.69                 1565.28           8.52x
BenchmarkGithub_events-8      170.77                  848.88           4.97x
BenchmarkGsoc_2018-8          225.13                 2515.59          11.17x
BenchmarkInstruments-8        120.39                  811.61           6.74x
BenchmarkMarine_ik-8           61.71                  422.43           6.85x
BenchmarkMesh-8                50.66                  371.39           7.33x
BenchmarkMesh_pretty-8         90.36                  784.89           8.69x
BenchmarkNumbers-8             52.61                  434.85           8.27x
BenchmarkRandom-8              85.87                  615.12           7.16x
BenchmarkTwitter-8            139.57                 1193.47           8.55x
BenchmarkTwitterescaped-8     102.28                  536.19           5.24x
BenchmarkUpdate_center-8      101.41                  860.52           8.49x
```

## AVX512 Acceleration

Stage 1 has been optimized using AVX512 instructions. Under full CPU load (8 threads) the AVX512 code is about 1 GB/sec (15%) faster as compared to the AVX2 code.

```
benchmark                                   AVX2 MB/s    AVX512 MB/s     speedup
BenchmarkFindStructuralBitsParallelLoop      7225.24      8302.96         1.15x
```

These benchmarks were generated on a c5.2xlarge EC2 instance with a Xeon Platinum 8124M CPU at 3.0 GHz.

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
  - In case `ALWAYS_COPY_STRINGS` is `false`: Only strings that contain special characters are copied to the String buffer 
in which case the payload from the tape is the offset into the String buffer. 
For string values without special characters the tape's payload points directly into the message buffer.
  - In case `ALWAYS_COPY_STRINGS` is `true` (default): Strings are always copied to the String buffer.

For more information, see `TestStage2BuildTape` in `stage2_build_tape_test.go`.

## Non streaming use cases

The best performance is obtained by keeping the JSON message fully mapped in memory and setting the
`ALWAYS_COPY_STRINGS` constant to `false`. This prevents duplicate copies of string values being made 
but mandates that the original JSON buffer is kept alive until the `ParsedJson` object is no longer needed
 (ie iteration over the tape format has been completed).
 
 In case the JSON message buffer is freed earlier (or for streaming use cases where memory is reused) 
 `ALWAYS_COPY_STRINGS` should be set to `true` (which is the default behaviour).

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
