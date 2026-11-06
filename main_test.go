package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/protomaps/go-pmtiles/pmtiles"
	"github.com/stretchr/testify/assert"
)

const worldMaxZoom = 3

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func worldArchive() []byte {
	ids := make([]uint64, 0)
	for z := uint8(0); z <= worldMaxZoom; z++ {
		for x := uint32(0); x < 1<<z; x++ {
			for y := uint32(0); y < 1<<z; y++ {
				ids = append(ids, pmtiles.ZxyToID(z, x, y))
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	entries := make([]pmtiles.EntryV3, 0, len(ids))
	tileData := make([]byte, 0)
	for _, id := range ids {
		z, x, y := pmtiles.IDToZxy(id)
		tile := []byte(strings.Join([]string{string(rune('0' + z)), string(rune('0' + x)), string(rune('0' + y))}, "/"))
		entries = append(entries, pmtiles.EntryV3{TileID: id, Offset: uint64(len(tileData)), Length: uint32(len(tile)), RunLength: 1})
		tileData = append(tileData, tile...)
	}
	rootBytes := pmtiles.SerializeEntries(entries, pmtiles.Gzip)
	metadataBytes, _ := pmtiles.SerializeMetadata(map[string]interface{}{"name": "world"}, pmtiles.Gzip)
	header := pmtiles.HeaderV3{
		SpecVersion:         3,
		RootOffset:          pmtiles.HeaderV3LenBytes,
		RootLength:          uint64(len(rootBytes)),
		MetadataOffset:      pmtiles.HeaderV3LenBytes + uint64(len(rootBytes)),
		MetadataLength:      uint64(len(metadataBytes)),
		LeafDirectoryOffset: pmtiles.HeaderV3LenBytes + uint64(len(rootBytes)) + uint64(len(metadataBytes)),
		TileDataOffset:      pmtiles.HeaderV3LenBytes + uint64(len(rootBytes)) + uint64(len(metadataBytes)),
		TileDataLength:      uint64(len(tileData)),
		AddressedTilesCount: uint64(len(entries)),
		TileEntriesCount:    uint64(len(entries)),
		TileContentsCount:   uint64(len(entries)),
		Clustered:           true,
		InternalCompression: pmtiles.Gzip,
		TileCompression:     pmtiles.NoCompression,
		TileType:            pmtiles.Png,
		MinZoom:             0,
		MaxZoom:             worldMaxZoom,
		MinLonE7:            -180 * 10000000,
		MinLatE7:            -85 * 10000000,
		MaxLonE7:            180 * 10000000,
		MaxLatE7:            85 * 10000000,
	}
	archive := pmtiles.SerializeHeader(header)
	archive = append(archive, rootBytes...)
	archive = append(archive, metadataBytes...)
	return append(archive, tileData...)
}

func worldDir(t *testing.T) string {
	dir := t.TempDir()
	assert.Nil(t, os.WriteFile(filepath.Join(dir, "world.pmtiles"), worldArchive(), 0666))
	return dir
}

type zxy struct {
	z    uint8
	x, y uint32
}

func archiveTiles(t *testing.T, data []byte) []zxy {
	header, err := pmtiles.DeserializeHeader(data[0:pmtiles.HeaderV3LenBytes])
	assert.Nil(t, err)
	tiles := make([]zxy, 0)
	err = pmtiles.IterateEntries(header,
		func(offset uint64, length uint64) ([]byte, error) {
			return data[offset : offset+length], nil
		},
		func(e pmtiles.EntryV3) {
			for i := uint64(0); i < uint64(e.RunLength); i++ {
				z, x, y := pmtiles.IDToZxy(e.TileID + i)
				start := header.TileDataOffset + e.Offset
				assert.Equal(t, []byte(strings.Join([]string{string(rune('0' + z)), string(rune('0' + x)), string(rune('0' + y))}, "/")), data[start:start+uint64(e.Length)])
				tiles = append(tiles, zxy{z, x, y})
			}
		})
	assert.Nil(t, err)
	return tiles
}

func zoomCounts(tiles []zxy) map[uint8]int {
	counts := make(map[uint8]int)
	for _, tile := range tiles {
		counts[tile.z]++
	}
	return counts
}

func serve(handler http.HandlerFunc, method, target string, body []byte) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(method, target, reader))
	return rec
}

func assertExtract(t *testing.T, rec *httptest.ResponseRecorder, fileName string) []zxy {
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, `attachment; filename=`+fileName, rec.Header().Get("Content-Disposition"))
	return archiveTiles(t, rec.Body.Bytes())
}

func TestServeExtractMaxZoomPath(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world-2.pmtiles", nil), "world-2.pmtiles")
	assert.Equal(t, map[uint8]int{0: 1, 1: 4, 2: 16}, zoomCounts(tiles))
}

func TestServeExtractTilePath(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world+2-1-1.pmtiles", nil), "world+2-1-1.pmtiles")
	assert.ElementsMatch(t, []zxy{{2, 1, 1}, {3, 2, 2}, {3, 3, 2}, {3, 2, 3}, {3, 3, 3}}, tiles)
}

func TestServeExtractPathErrors(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	for target, status := range map[string]int{
		"/world-25.pmtiles":      http.StatusBadRequest,
		"/world+25-0-0.pmtiles":  http.StatusBadRequest,
		"/world+2-4-0.pmtiles":   http.StatusBadRequest,
		"/world+2-0-4.pmtiles":   http.StatusBadRequest,
		"/missing-2.pmtiles":     http.StatusNotFound,
		"/missing+1-0-0.pmtiles": http.StatusNotFound,
		"/missing":               http.StatusNotFound,
		"/dir/world-2.pmtiles":   http.StatusNotFound,
		"/":                      http.StatusNotFound,
	} {
		assert.Equal(t, status, serve(handler, http.MethodGet, target, nil).Code, target)
	}
}

func TestServeExtractZoomLimits(t *testing.T) {
	dir := worldDir(t)
	handler := ExtractServeHandler("", dir, "", 1, 2, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world-3.pmtiles", nil), "world-3.pmtiles")
	assert.Equal(t, map[uint8]int{1: 4, 2: 16}, zoomCounts(tiles))
	tiles = assertExtract(t, serve(handler, http.MethodGet, "/world?minzoom=0&maxzoom=3", nil), "world.pmtiles")
	assert.Equal(t, map[uint8]int{1: 4, 2: 16}, zoomCounts(tiles))
	assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodGet, "/world+3-0-0.pmtiles", nil).Code)
}

func TestServeExtractSingleArchiveName(t *testing.T) {
	source := filepath.Join(worldDir(t), "world.pmtiles")
	handler := ExtractServeHandler("", source, "planet", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/planet-1.pmtiles", nil), "planet-1.pmtiles")
	assert.Equal(t, map[uint8]int{0: 1, 1: 4}, zoomCounts(tiles))
	assert.Equal(t, http.StatusNotFound, serve(handler, http.MethodGet, "/world-1.pmtiles", nil).Code)
}

func TestServeExtractRemoteArchive(t *testing.T) {
	archive := worldArchive()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "world.pmtiles", time.Time{}, bytes.NewReader(archive))
	}))
	t.Cleanup(server.Close)
	handler := ExtractServeHandler("", server.URL+"/world.pmtiles", "world", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world+1-0-0.pmtiles", nil), "world+1-0-0.pmtiles")
	assert.Equal(t, map[uint8]int{1: 1, 2: 4, 3: 16}, zoomCounts(tiles))
}

func TestServeExtractRemoteBucket(t *testing.T) {
	archive := worldArchive()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tiles/world.pmtiles" {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "world.pmtiles", time.Time{}, bytes.NewReader(archive))
	}))
	t.Cleanup(server.Close)
	handler := ExtractServeHandler(server.URL+"/tiles", "", "", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world-1.pmtiles", nil), "world-1.pmtiles")
	assert.Equal(t, map[uint8]int{0: 1, 1: 4}, zoomCounts(tiles))
	assert.Equal(t, http.StatusNotFound, serve(handler, http.MethodGet, "/missing-1.pmtiles", nil).Code)
}

func TestServeExtractHeadAndRange(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	full := serve(handler, http.MethodGet, "/world-1.pmtiles", nil)
	assert.Equal(t, http.StatusOK, full.Code)
	head := serve(handler, http.MethodHead, "/world-1.pmtiles", nil)
	assert.Equal(t, http.StatusOK, head.Code)
	assert.Equal(t, full.Header().Get("Content-Length"), head.Header().Get("Content-Length"))
	assert.Equal(t, 0, head.Body.Len())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/world-1.pmtiles", nil)
	req.Header.Set("Range", "bytes=0-6")
	handler(rec, req)
	assert.Equal(t, http.StatusPartialContent, rec.Code)
	assert.Equal(t, full.Body.Bytes()[0:7], rec.Body.Bytes())
}

func TestServeExtractWholeSource(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	for _, target := range []string{"/world", "/world.pmtiles"} {
		tiles := assertExtract(t, serve(handler, http.MethodGet, target, nil), "world.pmtiles")
		assert.Equal(t, map[uint8]int{0: 1, 1: 4, 2: 16, 3: 64}, zoomCounts(tiles), target)
	}
}

func TestServeExtractZoomParams(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world?maxzoom=1", nil), "world.pmtiles")
	assert.Equal(t, map[uint8]int{0: 1, 1: 4}, zoomCounts(tiles))
	tiles = assertExtract(t, serve(handler, http.MethodGet, "/world?minzoom=2&maxzoom=2", nil), "world.pmtiles")
	assert.Equal(t, map[uint8]int{2: 16}, zoomCounts(tiles))
}

func TestServeExtractBboxParam(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world?bbox=10,10,20,20", nil), "world.pmtiles")
	assert.ElementsMatch(t, []zxy{{0, 0, 0}, {1, 1, 0}, {2, 2, 1}, {3, 4, 3}}, tiles)
}

func TestServeExtractSliceParam(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	for target, fileName := range map[string]string{
		"/world?slice=1/1/0":                              "world+1-1-0.pmtiles",
		"/world?slice=1/1/0&output=w-{z}-{x}-{y}.pmtiles": "w-1-1-0.pmtiles",
	} {
		tiles := assertExtract(t, serve(handler, http.MethodGet, target, nil), fileName)
		assert.Equal(t, map[uint8]int{1: 1, 2: 4, 3: 16}, zoomCounts(tiles), target)
		assert.Contains(t, tiles, zxy{1, 1, 0})
		assert.NotContains(t, tiles, zxy{1, 0, 0})
	}
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world?slice=1/1/0&minzoom=0", nil), "world+1-1-0.pmtiles")
	assert.Equal(t, map[uint8]int{0: 1, 1: 1, 2: 4, 3: 16}, zoomCounts(tiles))
}

func TestServeExtractOutputParam(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	assertExtract(t, serve(handler, http.MethodGet, "/world?maxzoom=0&output=custom.pmtiles", nil), "custom.pmtiles")
}

const geojsonPolygon = `{"type":"Polygon","coordinates":[[[10,10],[20,10],[20,20],[10,20],[10,10]]]}`

const osmosisPolygon = `test
1
   10 10
   20 10
   20 20
   10 20
   10 10
END
END
`

func TestServeExtractPolygonParam(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	tiles := assertExtract(t, serve(handler, http.MethodGet, "/world?polygon="+url.QueryEscape(geojsonPolygon), nil), "world.pmtiles")
	assert.ElementsMatch(t, []zxy{{0, 0, 0}, {1, 1, 0}, {2, 2, 1}, {3, 4, 3}}, tiles)
}

func TestServeExtractPolygonBody(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	for _, polygon := range []string{geojsonPolygon, osmosisPolygon} {
		tiles := assertExtract(t, serve(handler, http.MethodPost, "/world?output=area.pmtiles", []byte(polygon)), "area.pmtiles")
		assert.ElementsMatch(t, []zxy{{0, 0, 0}, {1, 1, 0}, {2, 2, 1}, {3, 4, 3}}, tiles)
	}
}

func TestServeExtractParamErrors(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	polygon := url.QueryEscape(geojsonPolygon)
	for _, target := range []string{
		"/world?slice=1/0/0&slice=1/0/0",
		"/world?bbox=0,0,1,1&bbox=0,0,1,1",
		"/world?polygon=" + polygon + "&polygon=" + polygon,
		"/world?bbox=0,0,1,1&polygon=" + polygon,
		"/world?slice=1/0/0&bbox=0,0,1,1",
		"/world?slice=1/0/0&polygon=" + polygon,
		"/world?slice",
		"/world?bbox=0,0,1",
		"/world?bbox=a,0,1,1",
		"/world?polygon=notapolygon",
		"/world?slice=1/0",
		"/world?slice=1,0,0",
		"/world?slice=25/0/0",
		"/world?slice=1/a/0",
		"/world?slice=1/0/a",
		"/world?slice=1/2/0",
		"/world?slice=1/0/2",
		"/world?minzoom=25",
		"/world?maxzoom=a",
		"/world?overfetch=-1",
		"/world?overfetch=a",
		"/world?minzoom=3&maxzoom=2",
	} {
		assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodGet, target, nil).Code, target)
	}
	assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodPost, "/world?polygon="+polygon, []byte(geojsonPolygon)).Code)
	assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodPost, "/world?polygon="+polygon, nil).Code)
	assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodPost, "/world?maxzoom=1", nil).Code)
	assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodPost, "/world", []byte("notapolygon")).Code)
	assert.Equal(t, http.StatusBadRequest, serve(handler, http.MethodPost, "/world?bbox=0,0,1,1", []byte(geojsonPolygon)).Code)
}

func TestServeExtractNotFound(t *testing.T) {
	handler := ExtractServeHandler("", worldDir(t), "", -1, -1, 1, 0.05)
	assert.Equal(t, http.StatusNotFound, serve(handler, http.MethodGet, "/missing?maxzoom=1", nil).Code)
	assert.Equal(t, http.StatusNotFound, serve(handler, http.MethodPost, "/missing", []byte(geojsonPolygon)).Code)
	assert.Equal(t, http.StatusNotFound, serve(handler, http.MethodPut, "/world", nil).Code)
	assert.Equal(t, http.StatusNotFound, serve(handler, http.MethodDelete, "/world.pmtiles", nil).Code)
}
