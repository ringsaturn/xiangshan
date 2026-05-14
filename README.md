# xiangshan

Xiangshan is a Go reverse geocoder for OvertureMaps administrative divisions. It builds a size-prefixed FlatBuffers dataset and queries it through mmap with a two-tier grid index.

## Install

```sh
go install github.com/ringsaturn/xiangshan/cmd/xs-query@latest
go install github.com/ringsaturn/xiangshan/cmd/xs-serve@latest
```

Download prebuilt data from(No reliablity guarantee, consider building from source if you need it for production use):

```
Under ODbL license
© OpenStreetMap contributors, Overture Maps Foundation
https://dataset.ringsaturn.me/xiangshan/divisions.cf.bin
```

## Build Data

The default pipeline reads OvertureMaps division parquet files under `data/divisions` and writes `build/divisions.cf.bin`.

```sh
make pipeline
```

Individual stages are available as `make extract`, `make simplify`, and `make encode`.

## Go API

```go
finder, err := xiangshan.NewFinder("build/divisions.cf.bin")
if err != nil {
    return err
}
defer finder.Close()

res := finder.Query(2.2945, 48.8584)
fmt.Println(res.Country, res.Region, res.County, res.LocalAdmin)
```

`Finder` is safe for concurrent reads after construction. Query input is longitude, latitude in WGS84 degrees.

## CLI

```sh
xs-query -data build/divisions.cf.bin -lng 2.2945 -lat 48.8584
xs-query -data build/divisions.cf.bin -format json -lng 139.6917 -lat 35.6895
printf '2.2945,48.8584\n139.6917,35.6895\n' | xs-query -data build/divisions.cf.bin -stdin
```

## HTTP Server

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
  "local_admin": "Paris"
}
```

## Benchmarks

```sh
go test -bench=. -benchmem ./...
go test -bench=BenchmarkQuery -benchmem -memprofile=mem.out .
go tool pprof -alloc_objects mem.out
```

## License

- Codes under [MIT License](./LICENSE)
- Data under [ODbL License](./LICENSE_DATA)
  - © OpenStreetMap contributors, Overture Maps Foundation
  - Upstream: https://github.com/OvertureMaps/data
