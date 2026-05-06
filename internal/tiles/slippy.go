// Package tiles implements slippy-map tile coordinate math and bounding-box
// to tile-range conversions used throughout mbtiles-inspect.
//
// References:
//   - https://wiki.openstreetmap.org/wiki/Slippy_map_tilenames
//   - https://github.com/mapbox/mbtiles-spec/blob/master/1.3/spec.md
package tiles

import (
	"fmt"
	"math"
)

// YConvention identifies whether tile_row values are stored in XYZ
// (Google/Bing/Mapbox) or TMS (mbtiles spec) y-axis convention.
type YConvention int

const (
	YUnknown YConvention = iota
	YXYZ
	YTMS
)

func (c YConvention) String() string {
	switch c {
	case YXYZ:
		return "xyz"
	case YTMS:
		return "tms"
	default:
		return "unknown"
	}
}

// Coord is a tile coordinate at a given zoom.
type Coord struct {
	Z, X, Y int
}

func (c Coord) String() string { return fmt.Sprintf("%d/%d/%d", c.Z, c.X, c.Y) }

// BBox is a geographic bounding box: lon_min, lat_min, lon_max, lat_max
// in degrees, WGS84.
type BBox struct {
	LonMin, LatMin, LonMax, LatMax float64
}

// Range is a half-open inclusive rectangle of tile coordinates at a zoom.
type Range struct {
	Z          int
	XMin, YMin int
	XMax, YMax int // inclusive
}

// Count returns the number of tiles in the range.
func (r Range) Count() int {
	if r.XMax < r.XMin || r.YMax < r.YMin {
		return 0
	}
	return (r.XMax - r.XMin + 1) * (r.YMax - r.YMin + 1)
}

// LonLatToTile converts a (lon, lat) pair to the tile coordinates containing
// it at zoom z. Y is returned in XYZ convention.
//
// At the antimeridian and polar limits the result is clamped to valid tile
// indices [0, 2^z - 1].
func LonLatToTile(lon, lat float64, z int) (x, y int) {
	n := math.Exp2(float64(z))
	x = int(math.Floor((lon + 180.0) / 360.0 * n))

	latRad := lat * math.Pi / 180.0
	// Web Mercator clamps latitude to ±85.0511.
	const latLimit = 85.05112878
	if lat > latLimit {
		latRad = latLimit * math.Pi / 180.0
	} else if lat < -latLimit {
		latRad = -latLimit * math.Pi / 180.0
	}
	y = int(math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n))

	maxIdx := int(n) - 1
	x = clamp(x, 0, maxIdx)
	y = clamp(y, 0, maxIdx)
	return x, y
}

// BBoxToRange returns the inclusive tile range covering bbox at zoom z.
// Y values are XYZ convention.
func BBoxToRange(bbox BBox, z int) Range {
	xMin, yMax := LonLatToTile(bbox.LonMin, bbox.LatMin, z) // SW corner: min lon, max y
	xMax, yMin := LonLatToTile(bbox.LonMax, bbox.LatMax, z) // NE corner: max lon, min y
	if xMin > xMax {
		xMin, xMax = xMax, xMin
	}
	if yMin > yMax {
		yMin, yMax = yMax, yMin
	}
	return Range{Z: z, XMin: xMin, YMin: yMin, XMax: xMax, YMax: yMax}
}

// FlipY converts between XYZ and TMS y-axis at zoom z.
//
//	y_tms = (2^z - 1) - y_xyz
func FlipY(z, y int) int {
	return (1 << uint(z)) - 1 - y
}

// MaxIndex returns 2^z - 1, the highest valid tile index at zoom z.
func MaxIndex(z int) int { return (1 << uint(z)) - 1 }

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
