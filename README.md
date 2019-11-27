# simdjson-go

This is a Golang port of [simdjson](https://github.com/lemire/simdjson).

## Performance

NB These are work-in-progress numbers:

```
BenchmarkApache_builds-8                            8191            123565 ns/op        1030.02 MB/s       54029 B/op         16 allocs/op
BenchmarkCanada-8                                    250           4175523 ns/op         539.11 MB/s     1588042 B/op        351 allocs/op
BenchmarkCitm_catalog-8                             1141           1049429 ns/op        1645.85 MB/s      609392 B/op        145 allocs/op
BenchmarkGithub_events-8                           18038             66106 ns/op         985.27 MB/s       21042 B/op          8 allocs/op
BenchmarkGsoc_2018-8                                 999           1172012 ns/op        2839.42 MB/s      392243 B/op        111 allocs/op
BenchmarkInstruments-8                              4551            244737 ns/op         900.34 MB/s      120155 B/op         32 allocs/op
BenchmarkMarine_ik-8                                 152           6886706 ns/op         433.22 MB/s     3073637 B/op        667 allocs/op
BenchmarkMesh-8                                      621           1897085 ns/op         381.43 MB/s      676852 B/op        163 allocs/op
BenchmarkMesh_pretty-8                               602           1963028 ns/op         803.53 MB/s      703045 B/op        163 allocs/op
BenchmarkNumbers-8                                  3692            318313 ns/op         471.62 MB/s       87252 B/op         24 allocs/op
BenchmarkRandom-8                                   1492            774949 ns/op         658.72 MB/s      383505 B/op         95 allocs/op
BenchmarkTwitter-8                                  2374            485234 ns/op        1301.46 MB/s      242859 B/op         61 allocs/op
BenchmarkTwitterescaped-8                           1635            747260 ns/op         752.63 MB/s      244271 B/op         61 allocs/op
BenchmarkUpdate_center-8                            1886            576591 ns/op         924.71 MB/s      275938 B/op         69 allocs/op
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
