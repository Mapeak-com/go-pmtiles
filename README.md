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

* A local directory, or a bucket prefix with `--bucket`, holding `<source>.pmtiles` archives
* A single local or remote `.pmtiles` archive, served as `<source>` (its file name without `.pmtiles`, or `--name`)

Options:

* `--name`: name to serve a single archive under
* `--minzoom`, `--maxzoom`: zoom limits applied to every request
* `--bucket`: remote bucket holding the source
* `--port` (default `8080`), `--interface` (default `0.0.0.0`), `--cors`
* `--download-threads` (default `4`), `--overfetch` (default `0.05`)

Endpoints:

* `GET /<source>-<z>.pmtiles`: zoom levels 0 to `z`
* `GET /<source>+<z>-<x>-<y>.pmtiles`: the area of tile `z/x/y`, from zoom `z` up
* `GET /<source>` and `POST /<source>` (also `HEAD`), both with an optional `.pmtiles` suffix: the selection given by the query parameters below

Polygon extracts select, at each zoom level, the tiles that overlap the polygon, excluding tiles that only touch it. This matches Planetiler `--polygon`.

A polygon is an Osmosis .poly file or a GeoJSON Polygon or MultiPolygon, and the format is detected automatically.

`GET` takes the polygon as the `polygon` query parameter. `POST` requires the polygon as the raw request body (up to 64 MiB) and rejects a `polygon` query parameter. `pmtiles extract` takes a polygon file with `--polygon`.

Other query parameters, accepted by both `GET` and `POST`:

* `output`: the returned file name (default `{source}.pmtiles`, or `{source}+{z}-{x}-{y}.pmtiles` with `slice`)
* `bbox=min_lon,min_lat,max_lon,max_lat` uses the tile selection of `pmtiles extract --bbox`
* `slice=z/x/y` selects the area of tile `z/x/y`
  * Sets `minzoom` to `z` unless `minzoom` is given
  * Sets `output` to `{source}+{z}-{x}-{y}.pmtiles` unless `output` is given
  * Fills any `{z}`, `{x}`, and `{y}` placeholders in `output`
* `minzoom`, `maxzoom` and `overfetch`

`slice`, `polygon` and `bbox` are mutually exclusive, and each can appear at most once.

Responses are sent with `Content-Disposition: attachment` and the output file name. An extraction is cancelled when the client disconnects. Responses from remote sources can take up to 120 seconds to write; responses from local sources up to 10 seconds.

## Development

Run the program in development:

```sh
go run main.go
```

Run the test suite:

```sh
go test . ./pmtiles
```
