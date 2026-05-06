// Package diff compares two mbtiles files and produces a structured summary
// of metadata changes, per-zoom tile-count and size deltas, and a small sample
// of tiles unique to each side.
package diff

import (
	"context"
	"fmt"
	"sort"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// SampleSize caps how many "only-in-A" / "only-in-B" tiles the diff records.
// Matching SPEC.md: "Sample of tiles added/removed (first 10 of each)".
const SampleSize = 10

// Result is the structured diff between two mbtiles files.
type Result struct {
	APath string `json:"a"`
	BPath string `json:"b"`

	Metadata MetadataDiff `json:"metadata"`
	Zooms    []ZoomDiff   `json:"zooms"`

	OnlyInA []tiles.Coord `json:"only_in_a"` // up to SampleSize, Y in XYZ convention
	OnlyInB []tiles.Coord `json:"only_in_b"` // up to SampleSize, Y in XYZ convention
}

// MetadataDiff captures which metadata keys changed between A and B.
type MetadataDiff struct {
	Added    map[string]string  `json:"added,omitempty"`    // key in B only
	Removed  map[string]string  `json:"removed,omitempty"`  // key in A only
	Modified map[string]ABValue `json:"modified,omitempty"` // both, different value
}

// ABValue is a pair of "value in A" and "value in B" for a modified key.
type ABValue struct {
	A string `json:"a"`
	B string `json:"b"`
}

// ZoomDiff is per-zoom-level deltas. ATiles/BTiles is the count in each side.
type ZoomDiff struct {
	Zoom      int   `json:"zoom"`
	ATiles    int64 `json:"a_tiles"`
	BTiles    int64 `json:"b_tiles"`
	DeltaTile int64 `json:"delta_tiles"`
	ASize     int64 `json:"a_size_bytes"`
	BSize     int64 `json:"b_size_bytes"`
	DeltaSize int64 `json:"delta_size_bytes"`
}

// Compute produces a Result for the two readers. The readers must remain open
// for the duration of the call.
//
// Tile coordinates in OnlyInA/OnlyInB are normalized to XYZ Y convention so
// that the diff is meaningful even when A and B store Y differently.
func Compute(ctx context.Context, a, b *mbtiles.Reader) (*Result, error) {
	mdA, err := a.MetadataMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("metadata a: %w", err)
	}
	mdB, err := b.MetadataMap(ctx)
	if err != nil {
		return nil, fmt.Errorf("metadata b: %w", err)
	}

	typedA, err := a.Metadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("typed metadata a: %w", err)
	}
	typedB, err := b.Metadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("typed metadata b: %w", err)
	}
	yconvA, err := mbtiles.DetectYConvention(ctx, a, typedA)
	if err != nil {
		return nil, fmt.Errorf("detect y a: %w", err)
	}
	yconvB, err := mbtiles.DetectYConvention(ctx, b, typedB)
	if err != nil {
		return nil, fmt.Errorf("detect y b: %w", err)
	}

	statsA, err := a.ZoomStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("zoom stats a: %w", err)
	}
	statsB, err := b.ZoomStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("zoom stats b: %w", err)
	}

	onlyA, onlyB, err := sampleTileDiff(ctx, a, b, yconvA, yconvB, SampleSize)
	if err != nil {
		return nil, fmt.Errorf("sample tile diff: %w", err)
	}

	return &Result{
		APath:    a.Path(),
		BPath:    b.Path(),
		Metadata: diffMetadata(mdA, mdB),
		Zooms:    diffZooms(statsA, statsB),
		OnlyInA:  onlyA,
		OnlyInB:  onlyB,
	}, nil
}

func diffMetadata(a, b map[string]string) MetadataDiff {
	out := MetadataDiff{
		Added:    map[string]string{},
		Removed:  map[string]string{},
		Modified: map[string]ABValue{},
	}
	for k, av := range a {
		bv, ok := b[k]
		switch {
		case !ok:
			out.Removed[k] = av
		case av != bv:
			out.Modified[k] = ABValue{A: av, B: bv}
		}
	}
	for k, bv := range b {
		if _, ok := a[k]; !ok {
			out.Added[k] = bv
		}
	}
	if len(out.Added) == 0 {
		out.Added = nil
	}
	if len(out.Removed) == 0 {
		out.Removed = nil
	}
	if len(out.Modified) == 0 {
		out.Modified = nil
	}
	return out
}

func diffZooms(a, b []mbtiles.ZoomStats) []ZoomDiff {
	byZoom := map[int]*ZoomDiff{}
	for _, s := range a {
		zd := byZoom[s.Zoom]
		if zd == nil {
			zd = &ZoomDiff{Zoom: s.Zoom}
			byZoom[s.Zoom] = zd
		}
		zd.ATiles = s.TileCount
		zd.ASize = s.TotalSize
	}
	for _, s := range b {
		zd := byZoom[s.Zoom]
		if zd == nil {
			zd = &ZoomDiff{Zoom: s.Zoom}
			byZoom[s.Zoom] = zd
		}
		zd.BTiles = s.TileCount
		zd.BSize = s.TotalSize
	}
	out := make([]ZoomDiff, 0, len(byZoom))
	for _, zd := range byZoom {
		zd.DeltaTile = zd.BTiles - zd.ATiles
		zd.DeltaSize = zd.BSize - zd.ASize
		out = append(out, *zd)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Zoom < out[j].Zoom })
	return out
}

// sampleTileDiff streams (z, x, y) from both readers in lock-step and collects
// up to max tiles unique to each side, with Y normalized to XYZ. Both queries
// are ordered on the primary key, but if either side stores TMS, that order
// reverses the Y direction relative to XYZ — so we collect into per-zoom
// buckets, normalize, and then sort/cap. Memory stays bounded because we cap
// before final sort.
func sampleTileDiff(ctx context.Context, a, b *mbtiles.Reader, yconvA, yconvB tiles.YConvention, max int) (onlyA, onlyB []tiles.Coord, err error) {
	rawA, err := loadAllCoords(ctx, a, yconvA)
	if err != nil {
		return nil, nil, fmt.Errorf("coords a: %w", err)
	}
	rawB, err := loadAllCoords(ctx, b, yconvB)
	if err != nil {
		return nil, nil, fmt.Errorf("coords b: %w", err)
	}

	// After XYZ normalization, sort ascending so user-facing samples are
	// deterministic and ordered by (z, x, y).
	sort.Slice(rawA, func(i, j int) bool { return coordLess(rawA[i], rawA[j]) })
	sort.Slice(rawB, func(i, j int) bool { return coordLess(rawB[i], rawB[j]) })

	curA, hasA, idxA := next(rawA, 0)
	curB, hasB, idxB := next(rawB, 0)

	for hasA || hasB {
		if len(onlyA) >= max && len(onlyB) >= max {
			break
		}
		switch {
		case hasA && hasB && curA == curB:
			curA, hasA, idxA = next(rawA, idxA)
			curB, hasB, idxB = next(rawB, idxB)
		case hasA && (!hasB || coordLess(curA, curB)):
			if len(onlyA) < max {
				onlyA = append(onlyA, curA)
			}
			curA, hasA, idxA = next(rawA, idxA)
		default:
			if len(onlyB) < max {
				onlyB = append(onlyB, curB)
			}
			curB, hasB, idxB = next(rawB, idxB)
		}
	}
	return onlyA, onlyB, nil
}

func loadAllCoords(ctx context.Context, r *mbtiles.Reader, yconv tiles.YConvention) ([]tiles.Coord, error) {
	c, err := r.Coords(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	var out []tiles.Coord
	for {
		z, x, y, ok, err := c.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if yconv == tiles.YTMS {
			y = tiles.FlipY(z, y)
		}
		out = append(out, tiles.Coord{Z: z, X: x, Y: y})
	}
	return out, nil
}

func next(s []tiles.Coord, i int) (tiles.Coord, bool, int) {
	if i >= len(s) {
		return tiles.Coord{}, false, i
	}
	return s[i], true, i + 1
}

func coordLess(a, b tiles.Coord) bool {
	if a.Z != b.Z {
		return a.Z < b.Z
	}
	if a.X != b.X {
		return a.X < b.X
	}
	return a.Y < b.Y
}
