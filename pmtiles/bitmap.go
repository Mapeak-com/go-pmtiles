package pmtiles

import (
	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/maptile"
	"github.com/paulmach/orb/maptile/tilecover"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/orb/project"
	"math"
)

func bitmapMultiPolygon(zoom uint8, multipolygon orb.MultiPolygon) (*roaring64.Bitmap, *roaring64.Bitmap) {
	boundarySet := roaring64.New()

	for _, polygon := range multipolygon {
		for _, ring := range polygon {
			boundaryTiles, _ := tilecover.Geometry(orb.LineString(ring), maptile.Zoom(zoom)) // TODO is this buffer-aware?
			for tile := range boundaryTiles {
				boundarySet.Add(ZxyToID(uint8(tile.Z), tile.X, tile.Y))
			}
		}
	}

	multipolygonProjected := project.MultiPolygon(multipolygon.Clone(), project.WGS84.ToMercator)

	interiorSet := roaring64.New()
	i := boundarySet.Iterator()
	for i.HasNext() {
		id := i.Next()
		if !boundarySet.Contains(id+1) && i.HasNext() {
			z, x, y := IDToZxy(id + 1)
			tile := maptile.New(x, y, maptile.Zoom(z))
			if planar.MultiPolygonContains(multipolygonProjected, project.Point(tile.Center(), project.WGS84.ToMercator)) {
				interiorSet.AddRange(id+1, i.PeekNext())
			}
		}
	}

	return boundarySet, interiorSet
}

// tileFraction returns the fractional tile coordinates of p at zoom, snapping values within floating point noise of a tile edge onto it.
func tileFraction(p orb.Point, zoom maptile.Zoom) orb.Point {
	f := maptile.Fraction(p, zoom)
	for i := range f {
		if r := math.Round(f[i]); math.Abs(f[i]-r) < 1e-9 {
			f[i] = r
		}
	}
	return f
}

// segmentCrossesTile reports whether the segment from a to b, in fractional tile coordinates, passes through the interior of tile x, y.
func segmentCrossesTile(a orb.Point, b orb.Point, x uint32, y uint32) bool {
	t0 := 0.0
	t1 := 1.0
	low := [2]float64{float64(x), float64(y)}
	for i := 0; i < 2; i++ {
		d := b[i] - a[i]
		if d == 0 {
			if a[i] <= low[i] || a[i] >= low[i]+1 {
				return false
			}
			continue
		}
		enter := (low[i] - a[i]) / d
		exit := (low[i] + 1 - a[i]) / d
		t0 = math.Max(t0, math.Min(enter, exit))
		t1 = math.Min(t1, math.Max(enter, exit))
	}
	return t0 < t1
}

// bitmapMultiPolygonIntersecting returns the tiles at zoom that overlap multipolygon, excluding tiles that only touch its boundary.
func bitmapMultiPolygonIntersecting(zoom uint8, multipolygon orb.MultiPolygon) *roaring64.Bitmap {
	boundarySet, interiorSet := bitmapMultiPolygon(zoom, multipolygon)
	relevantSet := roaring64.New()
	for _, polygon := range multipolygon {
		for _, ring := range polygon {
			for i := 0; i+1 < len(ring); i++ {
				a := tileFraction(ring[i], maptile.Zoom(zoom))
				b := tileFraction(ring[i+1], maptile.Zoom(zoom))
				for tile := range tilecover.LineString(orb.LineString{ring[i], ring[i+1]}, maptile.Zoom(zoom)) {
					if segmentCrossesTile(a, b, tile.X, tile.Y) {
						relevantSet.Add(ZxyToID(uint8(tile.Z), tile.X, tile.Y))
					}
				}
			}
		}
	}
	multipolygonProjected := project.MultiPolygon(multipolygon.Clone(), project.WGS84.ToMercator)
	touchedSet := roaring64.AndNot(boundarySet, relevantSet)
	i := touchedSet.Iterator()
	for i.HasNext() {
		id := i.Next()
		z, x, y := IDToZxy(id)
		tile := maptile.New(x, y, maptile.Zoom(z))
		if planar.MultiPolygonContains(multipolygonProjected, project.Point(tile.Center(), project.WGS84.ToMercator)) {
			relevantSet.Add(id)
		}
	}
	relevantSet.Or(interiorSet)
	return relevantSet
}

// bitmapMultiPolygonZooms returns the tiles that overlap multipolygon, computed separately at each zoom from minzoom to maxzoom, inclusive.
func bitmapMultiPolygonZooms(minzoom uint8, maxzoom uint8, multipolygon orb.MultiPolygon) *roaring64.Bitmap {
	relevantSet := roaring64.New()
	for z := int(minzoom); z <= int(maxzoom); z++ {
		relevantSet.Or(bitmapMultiPolygonIntersecting(uint8(z), multipolygon))
	}
	return relevantSet
}

func generalizeOr(r *roaring64.Bitmap, minzoom uint8) {
	if r.GetCardinality() == 0 {
		return
	}
	maxZ, _, _ := IDToZxy(r.ReverseIterator().Next())

	var temp *roaring64.Bitmap
	var toIterate *roaring64.Bitmap

	temp = roaring64.New()
	toIterate = r

	for currentZ := int(maxZ); currentZ > int(minzoom); currentZ-- {
		iter := toIterate.Iterator()
		for iter.HasNext() {
			parentID := ParentID(iter.Next())
			temp.Add(parentID)
		}
		toIterate = temp
		r.Or(temp)
		temp = roaring64.New()
	}
}

func generalizeAnd(r *roaring64.Bitmap) {
	if r.GetCardinality() == 0 {
		return
	}
	maxZ, _, _ := IDToZxy(r.ReverseIterator().Next())

	var temp *roaring64.Bitmap
	var toIterate *roaring64.Bitmap

	temp = roaring64.New()
	toIterate = r

	for currentZ := int(maxZ); currentZ > 0; currentZ-- {
		iter := toIterate.Iterator()
		filled := 0
		current := uint64(0) // check me...
		for iter.HasNext() {
			id := iter.Next()
			parentID := ParentID(id)
			if parentID == current {
				filled++
				if filled == 4 {
					temp.Add(parentID)
				}
			} else {
				current = parentID
				filled = 1
			}
		}
		toIterate = temp
		r.Or(temp)
		temp = roaring64.New()
	}
}
