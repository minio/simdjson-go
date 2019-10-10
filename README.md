# simdjson-go

This is a Golang port of [simdjson](https://github.com/lemire/simdjson).

## Performance

NB These are work-in-progress numbers:

```
BenchmarkApache_builds-8                   10000            149439 ns/op         851.68 MB/s         332 B/op          0 allocs/op
BenchmarkCanada-8                            300           4614895 ns/op         487.78 MB/s      195136 B/op          0 allocs/op
BenchmarkCitm_catalog-8                     1000           1641124 ns/op        1052.45 MB/s       44917 B/op          0 allocs/op
BenchmarkGithub_events-8                   20000             68402 ns/op         952.18 MB/s          85 B/op          0 allocs/op
BenchmarkGsoc_2018-8                        1000           2321913 ns/op        1433.23 MB/s       86533 B/op          0 allocs/op
BenchmarkInstruments-8                      5000            305609 ns/op         721.00 MB/s        1148 B/op          0 allocs/op
BenchmarkMarine_ik-8                         200           7241483 ns/op         412.00 MB/s      387937 B/op          0 allocs/op
BenchmarkMesh-8                             1000           1931930 ns/op         374.55 MB/s       18826 B/op          0 allocs/op
BenchmarkMesh_pretty-8                       500           2490304 ns/op         633.40 MB/s       82053 B/op          0 allocs/op
BenchmarkNumbers-8                          5000            342380 ns/op         438.47 MB/s         783 B/op          0 allocs/op
BenchmarkRandom-8                           2000            913360 ns/op         558.90 MB/s        6644 B/op          0 allocs/op
BenchmarkTwitter-8                          2000            700847 ns/op         901.07 MB/s        8217 B/op          0 allocs/op
BenchmarkTwitterescaped-8                   2000            896358 ns/op         627.44 MB/s        7320 B/op          0 allocs/op
BenchmarkUpdate_center-8                    2000            754479 ns/op         706.68 MB/s        6939 B/op          0 allocs/op
BenchmarkFindStructuralBits-8           50000000                23.1 ns/op      2776.45 MB/s           0 B/op          0 allocs/op
```

## Number parsing performance

### Integers 

```
benchmark                            old ns/op     new ns/op     delta
BenchmarkParseNumber/Pos/63bit-8     27.4          26.7          -2.55%
BenchmarkParseNumber/Neg/63bit-8     27.4          27.3          -0.36%
```

### Floats

```
benchmark                              old ns/op     new ns/op     delta
BenchmarkParseNumberFloat-8            24.5          13.6          -44.49%
BenchmarkParseNumberFloatExp-8         56.5          5.63          -90.04%
BenchmarkParseNumberBig-8              83.0          25.5          -69.28%
BenchmarkParseNumberRandomBits-8       143           27.0          -81.12%
BenchmarkParseNumberRandomFloats-8     103           24.5          -76.21%
```
