package report

import (
	"encoding/json"
	"io"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
)

// JSON renders an InfoData payload as pretty-printed JSON to w.
func JSON(w io.Writer, d *InfoData) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(toJSONShape(d))
}

type infoJSON struct {
	File            string            `json:"file"`
	SizeBytes       int64             `json:"size_bytes"`
	Metadata        *mbtiles.Metadata `json:"metadata,omitempty"`
	TileYConvention string            `json:"tile_y_convention"`
	ZoomLevels      []zoomJSON        `json:"zoom_levels"`
}

type zoomJSON struct {
	Zoom          int     `json:"zoom"`
	TileCount     int64   `json:"tile_count"`
	ExpectedCount int     `json:"expected_count,omitempty"`
	CoveragePct   float64 `json:"coverage_pct,omitempty"`
	SizeBytes     int64   `json:"size_bytes"`
	SizeAvg       int64   `json:"size_avg"`
	SizeMin       int64   `json:"size_min"`
	SizeMax       int64   `json:"size_max"`
}

func toJSONShape(d *InfoData) *infoJSON {
	out := &infoJSON{
		File:            d.Path,
		SizeBytes:       d.SizeBytes,
		Metadata:        d.Metadata,
		TileYConvention: d.YConv.String(),
	}
	for _, z := range d.Zooms {
		out.ZoomLevels = append(out.ZoomLevels, zoomJSON{
			Zoom:          z.Stats.Zoom,
			TileCount:     z.Stats.TileCount,
			ExpectedCount: z.Expected,
			CoveragePct:   z.CoveragePct,
			SizeBytes:     z.Stats.TotalSize,
			SizeAvg:       z.Stats.AvgSize,
			SizeMin:       z.Stats.MinSize,
			SizeMax:       z.Stats.MaxSize,
		})
	}
	return out
}
