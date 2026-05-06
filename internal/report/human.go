// Package report renders mbtiles inspection results in human and JSON forms.
package report

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// InfoData is the payload produced by `info`, shared between human and JSON
// renderers.
type InfoData struct {
	Path      string
	SizeBytes int64
	Metadata  *mbtiles.Metadata
	YConv     tiles.YConvention
	Zooms     []ZoomSummary
}

// ZoomSummary combines raw tile stats with derived coverage figures.
type ZoomSummary struct {
	Stats       mbtiles.ZoomStats
	Expected    int
	CoveragePct float64
}

// Human renders an InfoData payload as human-readable text to w.
func Human(w io.Writer, d *InfoData) error {
	fmt.Fprintf(w, "File:    %s (%s)\n", d.Path, humanBytes(d.SizeBytes))
	if d.Metadata != nil {
		md := d.Metadata
		if md.Name != "" {
			fmt.Fprintf(w, "Name:    %s\n", md.Name)
		}
		if md.Format != "" {
			fmt.Fprintf(w, "Format:  %s\n", md.Format)
		}
		if md.Bounds != nil {
			fmt.Fprintf(w, "Bounds:  %.4f, %.4f, %.4f, %.4f\n",
				md.Bounds.LonMin, md.Bounds.LatMin, md.Bounds.LonMax, md.Bounds.LatMax)
		}
		if md.Center != nil {
			fmt.Fprintf(w, "Center:  %.4f, %.4f, %d\n",
				md.Center.Lon, md.Center.Lat, md.Center.Zoom)
		}
		if md.MinZoom != nil && md.MaxZoom != nil {
			fmt.Fprintf(w, "Zooms:   %d — %d\n", *md.MinZoom, *md.MaxZoom)
		}
		fmt.Fprintf(w, "Tile y:  %s\n", d.YConv)
		if md.JSON != nil && len(md.JSON.VectorLayers) > 0 {
			fmt.Fprintf(w, "\nVector layers (%d):\n", len(md.JSON.VectorLayers))
			tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
			for _, vl := range md.JSON.VectorLayers {
				zoomRange := ""
				if vl.MinZoom != nil && vl.MaxZoom != nil {
					zoomRange = fmt.Sprintf("minzoom %d, maxzoom %d", *vl.MinZoom, *vl.MaxZoom)
				}
				fmt.Fprintf(tw, "  %s\t%s\n", vl.ID, zoomRange)
			}
			tw.Flush()
		}
	}

	if len(d.Zooms) > 0 {
		fmt.Fprintf(w, "\nZoom coverage:\n")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  z\ttiles\tsize\tavg\tmax\tcoverage")
		for _, z := range d.Zooms {
			cov := "—"
			if z.Expected > 0 {
				cov = fmt.Sprintf("%.1f%%", z.CoveragePct)
			}
			fmt.Fprintf(tw, "  %d\t%d\t%s\t%s\t%s\t%s\n",
				z.Stats.Zoom,
				z.Stats.TileCount,
				humanBytes(z.Stats.TotalSize),
				humanBytes(z.Stats.AvgSize),
				humanBytes(z.Stats.MaxSize),
				cov,
			)
		}
		tw.Flush()
	}
	return nil
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n2 := n / unit; n2 >= unit; n2 /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// HumanStdout is a convenience for printing to os.Stdout, dropping any error.
func HumanStdout(d *InfoData) {
	_ = Human(os.Stdout, d)
}

// IsTerminal is intentionally not implemented here; color decisions live in
// the cmd layer where the user can pass --no-color.
var _ = strings.Builder{}
