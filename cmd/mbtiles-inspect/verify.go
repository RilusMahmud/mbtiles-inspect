package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rilus/mbtiles-inspect/internal/gaps"
	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
)

func newVerifyCmd() *cobra.Command {
	var minCoverage float64
	var sampleN int
	cmd := &cobra.Command{
		Use:   "verify <file.mbtiles>",
		Short: "Validate an mbtiles file for CI use; non-zero exit on issues.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVerify(cmd.Context(), args[0], minCoverage, sampleN)
		},
	}
	cmd.Flags().Float64Var(&minCoverage, "min-coverage", 95.0, "minimum acceptable coverage percentage per zoom")
	cmd.Flags().IntVar(&sampleN, "sample", 8, "number of tiles per zoom to spot-check for corruption (0 disables)")
	return cmd
}

func runVerify(ctx context.Context, path string, minCoverage float64, sampleN int) error {
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

	// Required-key policy.
	missing := []string{}
	if md.Name == "" {
		missing = append(missing, "name")
	}
	if md.Format == "" {
		missing = append(missing, "format")
	}
	if md.Bounds == nil {
		missing = append(missing, "bounds")
	}
	if md.MinZoom == nil {
		missing = append(missing, "minzoom")
	}
	if md.MaxZoom == nil {
		missing = append(missing, "maxzoom")
	}
	if len(missing) > 0 {
		return verifyError{msg: fmt.Sprintf("missing required metadata keys: %v", missing)}
	}

	yconv, err := mbtiles.DetectYConvention(ctx, r, md)
	if err != nil {
		return err
	}

	stats, err := r.ZoomStats(ctx)
	if err != nil {
		return err
	}

	// Every declared zoom must have tiles.
	have := map[int]bool{}
	for _, s := range stats {
		have[s.Zoom] = true
	}
	for z := *md.MinZoom; z <= *md.MaxZoom; z++ {
		if !have[z] {
			return verifyError{msg: fmt.Sprintf("zoom %d declared but has 0 tiles", z)}
		}
	}

	// Coverage per zoom.
	for _, s := range stats {
		res, err := gaps.Detect(ctx, r, md, s.Zoom, yconv)
		if err != nil {
			return fmt.Errorf("gap detect z=%d: %w", s.Zoom, err)
		}
		if res.CoveragePct() < minCoverage {
			return verifyError{msg: fmt.Sprintf(
				"zoom %d coverage %.2f%% below threshold %.2f%% (%d/%d tiles)",
				s.Zoom, res.CoveragePct(), minCoverage, res.Present, res.Expected,
			)}
		}
	}

	// Sample tiles per zoom for corruption.
	if sampleN > 0 {
		for _, s := range stats {
			blobs, err := r.SampleTilesAtZoom(ctx, s.Zoom, sampleN)
			if err != nil {
				return fmt.Errorf("sample z=%d: %w", s.Zoom, err)
			}
			for i, b := range blobs {
				if err := mbtiles.ValidateTile(md.Format, b); err != nil {
					return verifyError{msg: fmt.Sprintf(
						"zoom %d sample %d/%d: %v",
						s.Zoom, i+1, len(blobs), err,
					)}
				}
			}
		}
	}

	fmt.Println("ok")
	return nil
}
