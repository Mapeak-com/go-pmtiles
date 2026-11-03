package pmtiles

import (
	"github.com/paulmach/orb"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBboxRegion(t *testing.T) {
	result, err := BboxRegion("-1.906033,50.680367,1.097501,52.304934")
	assert.Nil(t, err)
	assert.Equal(t, -1.906033, result[0][0][0][0])
	assert.Equal(t, 52.304934, result[0][0][0][1])
	assert.Equal(t, 1.097501, result[0][0][2][0])
	assert.Equal(t, 50.680367, result[0][0][2][1])
}

func TestBboxRegionAntimeridianCrossing(t *testing.T) {
	result, err := BboxRegion("170,-10,-170,10")
	assert.Nil(t, err)
	assert.Equal(t, orb.MultiPolygon{
		{{{170, 10}, {180, 10}, {180, -10}, {170, -10}, {170, 10}}},
		{{{-180, 10}, {-170, 10}, {-170, -10}, {-180, -10}, {-180, 10}}},
	}, result)
}

func TestBboxRegionAntimeridianCrossingRingOrder(t *testing.T) {
	result, err := BboxRegion("179.5,-5,-179.5,5")
	assert.Nil(t, err)
	assert.Equal(t, orb.MultiPolygon{
		{{{179.5, 5}, {180, 5}, {180, -5}, {179.5, -5}, {179.5, 5}}},
		{{{-180, 5}, {-179.5, 5}, {-179.5, -5}, {-180, -5}, {-180, 5}}},
	}, result)
}

func TestRawPolygonRegion(t *testing.T) {
	result, err := UnmarshalRegion([]byte(`{
		"type": "Polygon",
		"coordinates": [[[0, 0],[0,1],[1,1],[0,0]]]
	}`))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
}

func TestRawMultiPolygonRegion(t *testing.T) {
	result, err := UnmarshalRegion([]byte(`{
		"type": "MultiPolygon",
		"coordinates": [[[[0, 0],[0,1],[1,1],[0,0]]]]
	}`))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
}

func TestRawPolygonFeatureRegion(t *testing.T) {
	result, err := UnmarshalRegion([]byte(`{
		"type": "Feature",
		"geometry": {
			"type": "Polygon",
			"coordinates": [[[0, 0],[0,1],[1,1],[0,0]]]
		}
	}`))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
}

func TestRawMultiPolygonFeatureRegion(t *testing.T) {
	result, err := UnmarshalRegion([]byte(`{
		"type": "Feature",
		"geometry": {
			"type": "MultiPolygon",
			"coordinates": [[[[0, 0],[0,1],[1,1],[0,0]]]]
		}
	}`))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(result))
}

func TestFeatureCollectionRegion(t *testing.T) {
	result, err := UnmarshalRegion([]byte(`{
		"type": "FeatureCollection",
		"features": [
			{
				"type": "Feature",
				"geometry": {
					"type": "MultiPolygon",
					"coordinates": [[[[0, 0],[0,1],[1,1],[0,0]]]]
				}
			},
			{
				"type": "Feature",
				"geometry": {
					"type": "Polygon",
					"coordinates": [[[1, 1],[1,2],[2,2],[1,1]]]
				}
			}
		]
	}`))
	assert.Nil(t, err)
	assert.Equal(t, 2, len(result))
}

func TestEmptyFeatureCollectionRegion(t *testing.T) {
	_, err := UnmarshalRegion([]byte(`{
		"type": "FeatureCollection",
		"features": [
		]
	}`))
	assert.NotNil(t, err)
}

func TestUnmarshalPoly(t *testing.T) {
	result, err := UnmarshalPoly([]byte(`test
1
  0 0
  0 10
  10 10
  10 0
END
!2
  2 2
  2 4
  4 4
  4 2
  2 2
END
3
  20 20
  20 21
  21 21
END
END
`))
	assert.Nil(t, err)
	assert.Equal(t, orb.MultiPolygon{
		{{{0, 0}, {0, 10}, {10, 10}, {10, 0}, {0, 0}}, {{2, 2}, {2, 4}, {4, 4}, {4, 2}, {2, 2}}},
		{{{20, 20}, {20, 21}, {21, 21}, {20, 20}}},
	}, result)
}

func TestUnmarshalPolyInvalid(t *testing.T) {
	_, err := UnmarshalPoly([]byte("test\n1\n 0 0\n 0 1\n 1 1\nEND\n"))
	assert.NotNil(t, err)
	_, err = UnmarshalPoly([]byte("test\n1\n 0 0\n 0 1\nEND\nEND\n"))
	assert.NotNil(t, err)
	_, err = UnmarshalPoly([]byte("test\n1\n 0 x\n 0 1\n 1 1\nEND\nEND\n"))
	assert.NotNil(t, err)
}

func TestUnmarshalPolygonDetectsFormat(t *testing.T) {
	expected := orb.MultiPolygon{{{{0, 0}, {0, 1}, {1, 1}, {0, 0}}}}
	result, err := UnmarshalPolygon([]byte(` {"type": "Polygon", "coordinates": [[[0, 0],[0,1],[1,1],[0,0]]]}`))
	assert.Nil(t, err)
	assert.Equal(t, expected, result)
	result, err = UnmarshalPolygon([]byte("test\r\n1\r\n 0 0\r\n 0 1\r\n 1 1\r\nEND\r\nEND\r\n"))
	assert.Nil(t, err)
	assert.Equal(t, expected, result)
}
