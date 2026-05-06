package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

func newExtractCmd() *cobra.Command {
	var tms bool
	cmd := &cobra.Command{
		Use:   "extract <file.mbtiles> <z/x/y>",
		Short: "Write a single tile blob to stdout (raw bytes, no decoding).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtract(cmd.Context(), args[0], args[1], tms)
		},
	}
	cmd.Flags().BoolVar(&tms, "tms", false, "the y argument is in TMS convention")
	return cmd
}

func runExtract(ctx context.Context, path, coordStr string, asTMS bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	z, x, y, err := parseCoord(coordStr)
	if err != nil {
		return invocationError{msg: err.Error()}
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
	yconv, err := mbtiles.DetectYConvention(ctx, r, md)
	if err != nil {
		return err
	}

	// User input is XYZ unless --tms; convert to storage convention.
	storageY := y
	if !asTMS && yconv == tiles.YTMS {
		storageY = tiles.FlipY(z, y)
	}
	if asTMS && yconv == tiles.YXYZ {
		storageY = tiles.FlipY(z, y)
	}

	blob, err := r.Tile(ctx, z, x, storageY)
	if err != nil {
		return err
	}
	if blob == nil {
		return verifyError{msg: fmt.Sprintf("no tile at %d/%d/%d", z, x, y)}
	}
	_, err = os.Stdout.Write(blob)
	return err
}

func parseCoord(s string) (z, x, y int, err error) {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("expected z/x/y, got %q", s)
	}
	if z, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, 0, fmt.Errorf("z: %w", err)
	}
	if x, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, 0, fmt.Errorf("x: %w", err)
	}
	if y, err = strconv.Atoi(parts[2]); err != nil {
		return 0, 0, 0, fmt.Errorf("y: %w", err)
	}
	return z, x, y, nil
}
