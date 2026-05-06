package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rilus/mbtiles-inspect/internal/diff"
	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/report"
)

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <a.mbtiles> <b.mbtiles>",
		Short: "Compare two mbtiles files: metadata, per-zoom counts and sizes, sample tiles.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runDiff(cmd.Context(), args[0], args[1], asJSON)
		},
	}
}

func runDiff(ctx context.Context, pathA, pathB string, asJSON bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	a, err := mbtiles.Open(pathA)
	if err != nil {
		return fmt.Errorf("open a: %w", err)
	}
	defer a.Close()
	b, err := mbtiles.Open(pathB)
	if err != nil {
		return fmt.Errorf("open b: %w", err)
	}
	defer b.Close()

	res, err := diff.Compute(ctx, a, b)
	if err != nil {
		return err
	}

	if asJSON {
		return report.DiffJSON(os.Stdout, res)
	}
	return report.DiffHuman(os.Stdout, res)
}
