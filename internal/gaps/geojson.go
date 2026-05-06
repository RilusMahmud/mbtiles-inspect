package gaps

import (
	"encoding/json"
	"io"
	"math"

	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// WriteGeoJSON writes the missing tiles as a GeoJSON FeatureCollection of
// polygons, one per missing tile. Useful for visualizing gaps on a map.
func WriteGeoJSON(w io.Writer, r *Result) error {
	fc := featureCollection{Type: "FeatureCollection", Features: make([]feature, 0, len(r.Missing))}
	for _, c := range r.Missing {
		bbox := tileBBox(c.Z, c.X, c.Y)
		fc.Features = append(fc.Features, feature{
			Type: "Feature",
			Properties: map[string]any{
				"z": c.Z, "x": c.X, "y": c.Y,
			},
			Geometry: geometry{
				Type: "Polygon",
				Coordinates: [][][]float64{{
					{bbox.LonMin, bbox.LatMin},
					{bbox.LonMax, bbox.LatMin},
					{bbox.LonMax, bbox.LatMax},
					{bbox.LonMin, bbox.LatMax},
					{bbox.LonMin, bbox.LatMin},
				}},
			},
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fc)
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

type feature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   geometry       `json:"geometry"`
}

type geometry struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

// tileBBox returns the geographic bounding box of an XYZ tile.
func tileBBox(z, x, y int) tiles.BBox {
	n := math.Exp2(float64(z))
	lonMin := float64(x)/n*360.0 - 180.0
	lonMax := float64(x+1)/n*360.0 - 180.0
	latMax := tile2lat(float64(y), n)
	latMin := tile2lat(float64(y+1), n)
	return tiles.BBox{LonMin: lonMin, LatMin: latMin, LonMax: lonMax, LatMax: latMax}
}

func tile2lat(y, n float64) float64 {
	rad := math.Atan(math.Sinh(math.Pi * (1 - 2*y/n)))
	return rad * 180.0 / math.Pi
}
