# simdjson-go

This is a Golang port of [simdjson](https://github.com/lemire/simdjson).

## Performance

NB These are work-in-progress numbers:

```
BenchmarkApache_builds-8                   10000            168254 ns/op         756.44 MB/s         333 B/op          0 allocs/op
BenchmarkCitm_catalog-8                     1000           1842063 ns/op         937.65 MB/s       44925 B/op          0 allocs/op
BenchmarkGithub_events-8                   20000             76156 ns/op         855.24 MB/s          85 B/op          0 allocs/op
BenchmarkGsoc_2018-8                         500           2470895 ns/op        1346.81 MB/s      173082 B/op          0 allocs/op
BenchmarkInstruments-8                      5000            340362 ns/op         647.39 MB/s        1150 B/op          0 allocs/op
BenchmarkNumbers-8                          5000            360070 ns/op         416.93 MB/s         784 B/op          0 allocs/op
BenchmarkRandom-8                           2000           1037615 ns/op         491.97 MB/s        6648 B/op          0 allocs/op
BenchmarkUpdate_center-8                    2000            846469 ns/op         629.88 MB/s        6943 B/op          0 allocs/op
```
