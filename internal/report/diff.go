package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/rilus/mbtiles-inspect/internal/diff"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// DiffJSON encodes a diff Result as pretty-printed JSON.
func DiffJSON(w io.Writer, r *diff.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// DiffHuman renders a diff Result for terminal display.
func DiffHuman(w io.Writer, r *diff.Result) error {
	fmt.Fprintf(w, "A: %s\nB: %s\n\n", r.APath, r.BPath)

	writeMetadataDiff(w, r.Metadata)

	if len(r.Zooms) > 0 {
		fmt.Fprintln(w, "\nPer-zoom:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  z\ta tiles\tb tiles\tΔ tiles\ta size\tb size\tΔ size")
		for _, z := range r.Zooms {
			fmt.Fprintf(tw, "  %d\t%d\t%d\t%+d\t%s\t%s\t%+d B\n",
				z.Zoom, z.ATiles, z.BTiles, z.DeltaTile,
				humanBytes(z.ASize), humanBytes(z.BSize), z.DeltaSize,
			)
		}
		tw.Flush()
	}

	writeSample(w, "\nOnly in A (sample)", r.OnlyInA)
	writeSample(w, "\nOnly in B (sample)", r.OnlyInB)
	if len(r.OnlyInA) == 0 && len(r.OnlyInB) == 0 {
		fmt.Fprintln(w, "\nTile sets match.")
	}
	return nil
}

func writeMetadataDiff(w io.Writer, m diff.MetadataDiff) {
	any := false
	if len(m.Modified) > 0 {
		fmt.Fprintln(w, "Metadata changed:")
		keys := sortedKeys(m.Modified)
		for _, k := range keys {
			fmt.Fprintf(w, "  ~ %s: %q → %q\n", k, m.Modified[k].A, m.Modified[k].B)
		}
		any = true
	}
	if len(m.Added) > 0 {
		if !any {
			fmt.Fprintln(w, "Metadata changed:")
		}
		for _, k := range sortedStrKeys(m.Added) {
			fmt.Fprintf(w, "  + %s: %q\n", k, m.Added[k])
		}
		any = true
	}
	if len(m.Removed) > 0 {
		if !any {
			fmt.Fprintln(w, "Metadata changed:")
		}
		for _, k := range sortedStrKeys(m.Removed) {
			fmt.Fprintf(w, "  - %s: %q\n", k, m.Removed[k])
		}
		any = true
	}
	if !any {
		fmt.Fprintln(w, "Metadata: identical")
	}
}

func writeSample(w io.Writer, header string, coords []tiles.Coord) {
	if len(coords) == 0 {
		return
	}
	fmt.Fprintln(w, header+":")
	for _, c := range coords {
		fmt.Fprintf(w, "  %d/%d/%d\n", c.Z, c.X, c.Y)
	}
}

func sortedKeys(m map[string]diff.ABValue) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedStrKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
