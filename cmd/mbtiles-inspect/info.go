package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/report"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

func newInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <file.mbtiles>",
		Short: "Print metadata and per-zoom summary for an mbtiles file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runInfo(cmd.Context(), args[0], asJSON)
		},
	}
}

func runInfo(ctx context.Context, path string, asJSON bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	r, err := mbtiles.Open(path)
	if err != nil {
		return err
	}
	defer r.Close()

	md, err := r.Metadata(ctx)
	if err != nil {
		return fmt.Errorf("read metadata: %w", err)
	}

	stats, err := r.ZoomStats(ctx)
	if err != nil {
		return fmt.Errorf("read zoom stats: %w", err)
	}

	yconv, err := mbtiles.DetectYConvention(ctx, r, md)
	if err != nil {
		return fmt.Errorf("detect y convention: %w", err)
	}

	zooms := make([]report.ZoomSummary, 0, len(stats))
	for _, s := range stats {
		zs := report.ZoomSummary{Stats: s}
		if md != nil && md.Bounds != nil {
			rng := tiles.BBoxToRange(*md.Bounds, s.Zoom)
			zs.Expected = rng.Count()
			if zs.Expected > 0 {
				zs.CoveragePct = 100.0 * float64(s.TileCount) / float64(zs.Expected)
				if zs.CoveragePct > 100 {
					// Tiles outside declared bounds are possible; cap display.
					zs.CoveragePct = 100
				}
			}
		}
		zooms = append(zooms, zs)
	}

	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	d := &report.InfoData{
		Path:      r.Path(),
		SizeBytes: fi.Size(),
		Metadata:  md,
		YConv:     yconv,
		Zooms:     zooms,
	}

	if asJSON {
		return report.JSON(os.Stdout, d)
	}
	return report.Human(os.Stdout, d)
}
