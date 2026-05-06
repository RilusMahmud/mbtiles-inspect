// Package gaps detects coverage gaps in mbtiles files by comparing the
// expected tile range (computed from declared bounds) against the tiles
// actually present in the file.
package gaps

import (
	"context"
	"fmt"
	"sort"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// Result is the gap analysis for a single zoom level.
//
// All Y values in Missing are normalized to XYZ convention regardless of
// how the file stores them, so output is consistent across files.
type Result struct {
	Zoom     int
	Expected int
	Present  int
	Missing  []tiles.Coord // Y in XYZ convention
	BBox     tiles.BBox    // bounds used for expected-set calculation
}

// CoveragePct returns the percentage of expected tiles that are present.
func (r Result) CoveragePct() float64 {
	if r.Expected == 0 {
		return 0
	}
	return 100.0 * float64(r.Present) / float64(r.Expected)
}

// Detect computes gap analysis for one zoom level.
func Detect(ctx context.Context, r *mbtiles.Reader, md *mbtiles.Metadata, zoom int, yconv tiles.YConvention) (*Result, error) {
	if md == nil || md.Bounds == nil {
		return nil, fmt.Errorf("cannot detect gaps without bounds metadata")
	}
	rng := tiles.BBoxToRange(*md.Bounds, zoom)
	expected := rng.Count()

	rows, err := r.TilesAtZoom(ctx, zoom)
	if err != nil {
		return nil, fmt.Errorf("read tiles at zoom %d: %w", zoom, err)
	}

	// Build present-set keyed by XYZ coords.
	present := make(map[[2]int]struct{}, len(rows))
	for _, row := range rows {
		yXYZ := row.Y
		if yconv == tiles.YTMS {
			yXYZ = tiles.FlipY(zoom, row.Y)
		}
		present[[2]int{row.X, yXYZ}] = struct{}{}
	}

	var missing []tiles.Coord
	for x := rng.XMin; x <= rng.XMax; x++ {
		for y := rng.YMin; y <= rng.YMax; y++ {
			if _, ok := present[[2]int{x, y}]; !ok {
				missing = append(missing, tiles.Coord{Z: zoom, X: x, Y: y})
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].X != missing[j].X {
			return missing[i].X < missing[j].X
		}
		return missing[i].Y < missing[j].Y
	})

	return &Result{
		Zoom:     zoom,
		Expected: expected,
		Present:  len(present),
		Missing:  missing,
		BBox:     *md.Bounds,
	}, nil
}
