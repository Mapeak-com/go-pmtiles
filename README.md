# go-pmtiles

The single-file utility for creating and working with [PMTiles](https://github.com/protomaps/PMTiles) archives.

## Installation

See [Releases](https://github.com/protomaps/go-pmtiles/releases) for your OS and architecture.

## Docs

See [docs.protomaps.com/pmtiles/cli](https://docs.protomaps.com/pmtiles/cli) for usage.

See [Go package docs](https://pkg.go.dev/github.com/protomaps/go-pmtiles/pmtiles) for API usage.

## serve-extract

`pmtiles serve-extract <source>` runs an HTTP server that extracts and returns a PMTiles archive for each request.

`<source>` is one of:

* A local directory, or a bucket prefix with `--bucket`, holding `<name>.pmtiles` archives
* A single local or remote `.pmtiles` archive, served as `<name>` (its file name without `.pmtiles`, or `--name`)

Options:

* `--name`: name to serve a single archive under
* `--minzoom`, `--maxzoom`: zoom limits applied to every request
* `--bucket`: remote bucket holding the source
* `--port` (default `8080`), `--interface` (default `0.0.0.0`), `--cors`
* `--download-threads` (default `4`), `--overfetch` (default `0.05`)

Endpoints:

* `GET /<name>-<z>.pmtiles`: zoom levels 0 to `z`
* `GET /<name>+<z>-<x>-<y>.pmtiles`: the area of tile `z/x/y`, from zoom `z` up
* `GET /<name>` or `GET /<name>.pmtiles` (also `HEAD`) with query parameters:
  * `bbox=min_lon,min_lat,max_lon,max_lat`: the same tile selection as `pmtiles extract --bbox`
  * `polygon=`: an Osmosis .poly or GeoJSON Polygon/MultiPolygon; at each zoom, the tiles that overlap the polygon, matching Planetiler `--polygon`
  * `slice=z/x/y`: the area of tile `z/x/y`, from `minzoom` (default `z`) up; fills `{z}`, `{x}`, `{y}` in `output`
  * `minzoom`, `maxzoom`, `overfetch`
  * `output`: the returned file name (default `<name>.pmtiles`, or `<name>+<z>-<x>-<y>.pmtiles` with `slice`)
* `POST /<name>` takes the same query parameters, with the raw polygon file (GeoJSON or Osmosis .poly, up to 64 MiB) as the request body in place of `polygon`

`slice`, `polygon` and `bbox` are mutually exclusive, and each can appear at most once.

Responses are sent with `Content-Disposition: attachment` and the output file name. An extraction is cancelled when the client disconnects. Responses from remote sources can take up to 120 seconds to write; responses from local sources up to 10 seconds.

`pmtiles extract --polygon <file>` selects tiles the same way as the `polygon` parameter.

## Development

Run the program in development:

```sh
go run main.go
```

Run the test suite:

```sh
go test . ./pmtiles
```
