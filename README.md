# 香山/xiangshan

Xiangshan is a Go reverse geocoder for OvertureMaps administrative divisions. It builds a size-prefixed FlatBuffers dataset and queries it through mmap with a two-tier grid index.

## Install

```sh
go get github.com/ringsaturn/xiangshan
```

Download prebuilt data from(No reliablity guarantee, consider building from source if you need it for production use), based on 2026-04-15.0 OvertureMaps data:

```
Under ODbL license
© OpenStreetMap contributors, Overture Maps Foundation
https://dataset.ringsaturn.me/xiangshan/divisions.cf.bin
```

### Build Data

The default pipeline reads OvertureMaps division parquet files under `data/divisions` and writes `build/divisions.cf.bin`.

```sh
make pipeline
```

Individual stages are available as `make extract`, `make simplify`, and `make encode`.

## Go API

```go
package main

import (
	"fmt"

	"github.com/ringsaturn/xiangshan"
)

func main() {
	finder, err := xiangshan.NewFinder("build/divisions.cf.bin")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer finder.Close()

	res := finder.Query(2.2945, 48.8584)
	fmt.Println(res.Country, res.Region, res.County, res.LocalAdmin)
	// Output: France Île-de-France Paris Paris
}
```

`Finder` is safe for concurrent reads after construction. Query input is longitude, latitude in WGS84 degrees.

## CLI

```bash
go install github.com/ringsaturn/xiangshan/cmd/xs-query@latest
```

```sh
xs-query -data build/divisions.cf.bin -lng 2.2945 -lat 48.8584
xs-query -data build/divisions.cf.bin -format json -lng 139.6917 -lat 35.6895
printf '2.2945,48.8584\n139.6917,35.6895\n' | xs-query -data build/divisions.cf.bin -stdin
```

## HTTP Server

```bash
go install github.com/ringsaturn/xiangshan/cmd/xs-serve@latest
```

```sh
xs-serve -data build/divisions.cf.bin -addr :8080
curl 'http://localhost:8080/query?lng=2.2945&lat=48.8584'
```

Response:

```json
{
  "country": "France",
  "region": "Île-de-France",
  "county": "Paris",
  "local_admin": "Paris",
  "locality": "Paris",
  "country_id": "51bc7545-7602-435d-8b11-90117246a387",
  "region_id": "ad3154a9-92ec-40ef-ba0d-8443d8e024fd",
  "county_id": "a86c4ba9-8261-4a37-9fac-0c6aa9456d05",
  "local_admin_id": "4e5c3982-82ce-43ba-aef2-6b501d542604",
  "locality_id": "97b66514-3f41-47ac-a348-9cfd51d983d5"
}
```

## Benchmarks

```sh
go test -bench=. -benchmem ./...
go test -bench=BenchmarkQuery -benchmem -memprofile=mem.out .
go tool pprof -alloc_objects mem.out
```

A sample benchmark run on Apple M3 Max:

```
go test -bench=. -benchmem ./...
goos: darwin
goarch: arm64
pkg: github.com/ringsaturn/xiangshan
cpu: Apple M3 Max
BenchmarkQuery_WorldCities-16             145689             11687 ns/op               0 B/op          0 allocs/op
BenchmarkQuery_DenseEurope-16             156381             14802 ns/op               0 B/op          0 allocs/op
BenchmarkQuery_Parallel-16               1000000              1084 ns/op               0 B/op          0 allocs/op
PASS
```

## License

- Codes under [MIT License](./LICENSE)
- Data under [ODbL License](./LICENSE_DATA)
  - © OpenStreetMap contributors, Overture Maps Foundation
  - Upstream: https://github.com/OvertureMaps/data
