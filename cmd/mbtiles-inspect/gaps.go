package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rilus/mbtiles-inspect/internal/gaps"
	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
)

func newGapsCmd() *cobra.Command {
	var zoom int
	var geojson bool
	cmd := &cobra.Command{
		Use:   "gaps <file.mbtiles>",
		Short: "List missing tiles within the file's declared bounds.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runGaps(cmd.Context(), args[0], zoom, asJSON, geojson)
		},
	}
	cmd.Flags().IntVar(&zoom, "zoom", -1, "zoom level (default: all populated zooms)")
	cmd.Flags().BoolVar(&geojson, "geojson", false, "emit GeoJSON FeatureCollection of missing tiles")
	return cmd
}

func runGaps(ctx context.Context, path string, zoom int, asJSON, asGeoJSON bool) error {
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
	if md == nil || md.Bounds == nil {
		return invocationError{msg: "file has no bounds metadata; cannot compute expected tiles"}
	}

	yconv, err := mbtiles.DetectYConvention(ctx, r, md)
	if err != nil {
		return err
	}

	stats, err := r.ZoomStats(ctx)
	if err != nil {
		return err
	}

	zooms := []int{}
	if zoom >= 0 {
		zooms = append(zooms, zoom)
	} else {
		for _, s := range stats {
			zooms = append(zooms, s.Zoom)
		}
	}

	var results []*gaps.Result
	for _, z := range zooms {
		res, err := gaps.Detect(ctx, r, md, z, yconv)
		if err != nil {
			return fmt.Errorf("zoom %d: %w", z, err)
		}
		results = append(results, res)
	}

	switch {
	case asGeoJSON:
		// Combined FeatureCollection across all requested zooms.
		merged := &gaps.Result{}
		for _, res := range results {
			merged.Missing = append(merged.Missing, res.Missing...)
		}
		return gaps.WriteGeoJSON(os.Stdout, merged)
	case asJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	default:
		for _, res := range results {
			fmt.Printf("z=%d missing tiles (%d of %d expected, %.1f%% coverage):\n",
				res.Zoom, len(res.Missing), res.Expected, res.CoveragePct())
			for _, c := range res.Missing {
				fmt.Printf("  %d,%d\n", c.X, c.Y)
			}
		}
		return nil
	}
}
