package pmtiles

import (
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/orb/project"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestGeneralizeAnd(t *testing.T) {
	b := roaring64.New()
	generalizeAnd(b)
	assert.Equal(t, uint64(0), b.GetCardinality())
	b = roaring64.New()
	b.Add(ZxyToID(3, 0, 0))
	generalizeAnd(b)
	assert.Equal(t, uint64(1), b.GetCardinality())
	b = roaring64.New()
	b.Add(ZxyToID(3, 0, 0))
	b.Add(ZxyToID(3, 0, 1))
	b.Add(ZxyToID(3, 1, 0))
	b.Add(ZxyToID(3, 1, 1))
	generalizeAnd(b)
	assert.Equal(t, uint64(5), b.GetCardinality())
	assert.True(t, b.Contains(ZxyToID(2, 0, 0)))
}

func TestGeneralizeOr(t *testing.T) {
	b := roaring64.New()
	generalizeOr(b, 0)
	assert.Equal(t, uint64(0), b.GetCardinality())
	b = roaring64.New()
	b.Add(ZxyToID(3, 0, 0))
	generalizeOr(b, 0)
	assert.Equal(t, uint64(4), b.GetCardinality())
	assert.True(t, b.Contains(ZxyToID(2, 0, 0)))
	assert.True(t, b.Contains(ZxyToID(1, 0, 0)))
	assert.True(t, b.Contains(ZxyToID(0, 0, 0)))
}

func TestGeneralizeOrMinZoom(t *testing.T) {
	b := roaring64.New()
	b.Add(ZxyToID(3, 0, 0))
	generalizeOr(b, 2)
	assert.Equal(t, uint64(2), b.GetCardinality())
	assert.True(t, b.Contains(ZxyToID(2, 0, 0)))
	assert.False(t, b.Contains(ZxyToID(1, 0, 0)))
}

func tileIntersectsMultiPolygon(z uint8, x uint32, y uint32, multipolygon orb.MultiPolygon) bool {
	scale := math.Exp2(float64(z))
	toTile := func(p orb.Point) orb.Point {
		m := project.Point(p, project.WGS84.ToMercator)
		return orb.Point{(m[0]/(2*math.Pi*6378137) + 0.5) * scale, (0.5 - m[1]/(2*math.Pi*6378137)) * scale}
	}
	minX, minY, maxX, maxY := float64(x), float64(y), float64(x+1), float64(y+1)
	for _, polygon := range multipolygon {
		for _, ring := range polygon {
			for i := 0; i+1 < len(ring); i++ {
				a := toTile(ring[i])
				b := toTile(ring[i+1])
				t0, t1 := 0.0, 1.0
				dx, dy := b[0]-a[0], b[1]-a[1]
				clip := func(p, q float64) bool {
					if p == 0 {
						return q >= 0
					}
					r := q / p
					if p < 0 {
						t0 = math.Max(t0, r)
					} else {
						t1 = math.Min(t1, r)
					}
					return t0 <= t1
				}
				if clip(-dx, a[0]-minX) && clip(dx, maxX-a[0]) && clip(-dy, a[1]-minY) && clip(dy, maxY-a[1]) {
					return true
				}
			}
		}
	}
	tile := maptile.New(x, y, maptile.Zoom(z))
	return planar.MultiPolygonContains(project.MultiPolygon(multipolygon.Clone(), project.WGS84.ToMercator), project.Point(tile.Center(), project.WGS84.ToMercator))
}

func TestBitmapMultiPolygonZooms(t *testing.T) {
	multipolygon := orb.MultiPolygon{
		{{{-3.1, 40.3}, {7.7, 58.9}, {21.3, 37.2}, {-3.1, 40.3}}, {{5.3, 43.1}, {9.6, 47.4}, {12.2, 42.6}, {5.3, 43.1}}},
		{{{-70.3, -30.1}, {-69.7, -30.1}, {-69.7, -29.6}, {-70.3, -29.6}, {-70.3, -30.1}}},
	}
	result := bitmapMultiPolygonZooms(2, 8, multipolygon)
	expected := roaring64.New()
	for z := uint8(2); z <= 8; z++ {
		for x := uint32(0); x < 1<<z; x++ {
			for y := uint32(0); y < 1<<z; y++ {
				if tileIntersectsMultiPolygon(z, x, y, multipolygon) {
					expected.Add(ZxyToID(z, x, y))
				}
			}
		}
	}
	assert.True(t, expected.GetCardinality() > 0)
	assert.Equal(t, expected.ToArray(), result.ToArray())
}

func TestBitmapMultiPolygonZoomsExcludesTouchingTiles(t *testing.T) {
	multipolygon := orb.MultiPolygon{maptile.New(612, 415, 10).Bound().ToPolygon()}
	result := bitmapMultiPolygonZooms(9, 11, multipolygon)
	assert.Equal(t, []uint64{
		ZxyToID(9, 306, 207),
		ZxyToID(10, 612, 415),
		ZxyToID(11, 1225, 831),
		ZxyToID(11, 1225, 830),
		ZxyToID(11, 1224, 830),
		ZxyToID(11, 1224, 831),
	}, result.ToArray())
}
