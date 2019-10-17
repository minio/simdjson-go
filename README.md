# simdjson-go

This is a Golang port of [simdjson](https://github.com/lemire/simdjson).

## Performance

NB These are work-in-progress numbers:

```
BenchmarkApache_builds-8                            8727            135900 ns/op         936.54 MB/s       66330 B/op          7 allocs/op
BenchmarkCanada-8                                    280           3804952 ns/op         591.61 MB/s     1505172 B/op         86 allocs/op
BenchmarkCitm_catalog-8                             1083            974982 ns/op        1771.52 MB/s      586289 B/op         37 allocs/op
BenchmarkGithub_events-8                           14911             79354 ns/op         820.78 MB/s       33377 B/op          5 allocs/op
BenchmarkGsoc_2018-8                                 703           1609971 ns/op        2067.01 MB/s      397038 B/op         22 allocs/op
BenchmarkInstruments-8                              4618            247145 ns/op         891.56 MB/s      116081 B/op         10 allocs/op
BenchmarkMarine_ik-8                                 168           6235340 ns/op         478.48 MB/s     2941722 B/op        163 allocs/op
BenchmarkMesh-8                                      697           1689862 ns/op         428.20 MB/s      641824 B/op         41 allocs/op
BenchmarkMesh_pretty-8                               670           1767058 ns/op         892.64 MB/s      665503 B/op         41 allocs/op
BenchmarkNumbers-8                                  3528            325601 ns/op         461.07 MB/s       83217 B/op          8 allocs/op
BenchmarkRandom-8                                   1705            703734 ns/op         725.38 MB/s      366373 B/op         25 allocs/op
BenchmarkTwitter-8                                  2486            468589 ns/op        1347.69 MB/s      234483 B/op         17 allocs/op
BenchmarkTwitterescaped-8                           1646            704315 ns/op         798.52 MB/s      236053 B/op         17 allocs/op
BenchmarkUpdate_center-8                            2102            550810 ns/op         967.99 MB/s      267240 B/op         19 allocs/op
BenchmarkFindStructuralBits-8                   55953850                21.4 ns/op      2989.21 MB/s           0 B/op          0 allocs/op
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
