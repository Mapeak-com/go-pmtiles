package pmtiles

import (
	"bytes"
	"fmt"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
	"strconv"
	"strings"
)

func bboxToMultiPolygon(minLon, minLat, maxLon, maxLat float64) orb.MultiPolygon {
	if minLon > maxLon {
		return orb.MultiPolygon{
			{{{minLon, maxLat}, {180, maxLat}, {180, minLat}, {minLon, minLat}, {minLon, maxLat}}},
			{{{-180, maxLat}, {maxLon, maxLat}, {maxLon, minLat}, {-180, minLat}, {-180, maxLat}}},
		}
	}
	return orb.MultiPolygon{{{{minLon, maxLat}, {maxLon, maxLat}, {maxLon, minLat}, {minLon, minLat}, {minLon, maxLat}}}}
}

// BboxRegion parses a bbox string into an orb.MultiPolygon region.
func BboxRegion(bbox string) (orb.MultiPolygon, error) {
	parts := strings.Split(bbox, ",")
	minLon, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return nil, err
	}
	minLat, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, err
	}
	maxLon, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, err
	}
	maxLat, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return nil, err
	}
	return bboxToMultiPolygon(minLon, minLat, maxLon, maxLat), nil
}

// UnmarshalRegion parses JSON bytes into an orb.MultiPolygon region.
func UnmarshalRegion(data []byte) (orb.MultiPolygon, error) {
	fc, err := geojson.UnmarshalFeatureCollection(data)

	if err == nil {
		retval := make([]orb.Polygon, 0)
		for _, f := range fc.Features {
			switch v := f.Geometry.(type) {
			case orb.Polygon:
				retval = append(retval, v)
			case orb.MultiPolygon:
				retval = append(retval, v...)
			}
		}
		if len(retval) > 0 {
			return retval, nil
		}
	}

	f, err := geojson.UnmarshalFeature(data)

	if err == nil {
		switch v := f.Geometry.(type) {
		case orb.Polygon:
			return []orb.Polygon{v}, nil
		case orb.MultiPolygon:
			return v, nil
		}
	}

	g, err := geojson.UnmarshalGeometry(data)

	if err != nil {
		return nil, err
	}

	switch v := g.Geometry().(type) {
	case orb.Polygon:
		return []orb.Polygon{v}, nil
	case orb.MultiPolygon:
		return v, nil
	}

	return nil, fmt.Errorf("No geometry")
}

// UnmarshalPoly parses an Osmosis polygon filter file into an orb.MultiPolygon region.
// A ring whose header starts with '!' is a hole in each preceding ring that contains it.
func UnmarshalPoly(data []byte) (orb.MultiPolygon, error) {
	var result orb.MultiPolygon
	var ring orb.Ring
	firstLine := true
	inRing := false
	inPolygon := true
	hole := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case !inPolygon:
			return nil, fmt.Errorf("poly file continues after end of polygon")
		case firstLine:
			firstLine = false
		case inRing && line == "END":
			if len(ring) > 0 && !ring.Closed() {
				ring = append(ring, ring[0])
			}
			if len(ring) < 4 {
				return nil, fmt.Errorf("poly ring has fewer than 3 distinct points")
			}
			if hole {
				for i := range result {
					if planar.RingContains(result[i][0], ring[0]) {
						result[i] = append(result[i], ring)
					}
				}
			} else {
				result = append(result, orb.Polygon{ring})
			}
			ring = nil
			inRing = false
		case inRing:
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return nil, fmt.Errorf("invalid poly coordinate: %s", line)
			}
			lon, err := strconv.ParseFloat(fields[0], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid poly coordinate: %w", err)
			}
			lat, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid poly coordinate: %w", err)
			}
			ring = append(ring, orb.Point{lon, lat})
		case line == "END":
			inPolygon = false
		default:
			hole = line[0] == '!'
			inRing = true
		}
	}
	if inRing {
		return nil, fmt.Errorf("poly file has an unclosed ring")
	}
	if inPolygon {
		return nil, fmt.Errorf("poly file ends before end of polygon")
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("No geometry")
	}
	return result, nil
}

// UnmarshalPolygon parses a GeoJSON Polygon or MultiPolygon, or an Osmosis polygon filter file, into an orb.MultiPolygon region.
func UnmarshalPolygon(data []byte) (orb.MultiPolygon, error) {
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '{' {
		return UnmarshalRegion(data)
	}
	return UnmarshalPoly(data)
}
