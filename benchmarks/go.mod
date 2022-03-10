module github.com/minio/simdjson-go/benchmarks

go 1.17

require (
	github.com/buger/jsonparser v1.1.1
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.14.2
	github.com/minio/simdjson-go v0.0.0-00010101000000-000000000000
)

replace github.com/minio/simdjson-go => ../

require (
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
)
