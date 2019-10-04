# simdjson-go

This is a Golang port of [simdjson](https://github.com/lemire/simdjson).

## Performance

NB These are work-in-progress numbers:

```
BenchmarkApache_builds-8                   10000            148883 ns/op         854.87 MB/s         333 B/op          0 allocs/op
BenchmarkCitm_catalog-8                     1000           1650748 ns/op        1046.32 MB/s       44925 B/op          0 allocs/op
BenchmarkGithub_events-8                   20000             67222 ns/op         968.91 MB/s          85 B/op          0 allocs/op
BenchmarkGsoc_2018-8                        1000           2259089 ns/op        1473.09 MB/s       86541 B/op          0 allocs/op
BenchmarkInstruments-8                      5000            302432 ns/op         728.58 MB/s        1150 B/op          0 allocs/op
BenchmarkNumbers-8                          5000            327197 ns/op         458.82 MB/s         784 B/op          0 allocs/op
BenchmarkRandom-8                           2000            907078 ns/op         562.77 MB/s        6648 B/op          0 allocs/op
BenchmarkUpdate_center-8                    2000            745267 ns/op         715.42 MB/s        6943 B/op          0 allocs/op
```
