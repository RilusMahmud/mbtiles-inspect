package mbtiles

import (
	"context"
	"fmt"

	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// DetectYConvention determines whether the file stores tile_row in TMS or
// XYZ convention.
//
// Strategy:
//  1. If `metadata.scheme` is set explicitly, trust it.
//  2. Otherwise, take the lowest populated zoom and a non-empty bounds.
//     Compute the expected XYZ y-range and the expected TMS y-range
//     (which is just the flipped XYZ range). Whichever the observed
//     [min,max] tile_row range falls inside is the convention.
//  3. If both match (small files, symmetric data), default to TMS per spec.
//  4. If nothing matches, return YUnknown — the caller should warn.
func DetectYConvention(ctx context.Context, r *Reader, md *Metadata) (tiles.YConvention, error) {
	if md != nil {
		switch md.Scheme {
		case "tms":
			return tiles.YTMS, nil
		case "xyz":
			return tiles.YXYZ, nil
		}
	}
	if md == nil || md.Bounds == nil {
		return tiles.YUnknown, nil
	}

	stats, err := r.ZoomStats(ctx)
	if err != nil {
		return tiles.YUnknown, fmt.Errorf("zoom stats: %w", err)
	}
	if len(stats) == 0 {
		return tiles.YUnknown, nil
	}

	// Use the lowest populated zoom — most discriminating relative range size.
	s := stats[0]
	rng := tiles.BBoxToRange(*md.Bounds, s.Zoom)

	xyzLo, xyzHi := rng.YMin, rng.YMax
	tmsLo, tmsHi := tiles.FlipY(s.Zoom, rng.YMax), tiles.FlipY(s.Zoom, rng.YMin)

	matchesXYZ := s.YMin >= xyzLo && s.YMax <= xyzHi
	matchesTMS := s.YMin >= tmsLo && s.YMax <= tmsHi

	switch {
	case matchesXYZ && !matchesTMS:
		return tiles.YXYZ, nil
	case matchesTMS && !matchesXYZ:
		return tiles.YTMS, nil
	case matchesXYZ && matchesTMS:
		// Ambiguous; spec default is TMS.
		return tiles.YTMS, nil
	default:
		return tiles.YUnknown, nil
	}
}
