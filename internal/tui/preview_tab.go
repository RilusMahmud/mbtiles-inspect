package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/mvt"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

type previewTab struct {
	reader *mbtiles.Reader
	md     *mbtiles.Metadata
	yconv  tiles.YConvention
	styles Styles

	z, x, y  int // user-facing XYZ coords
	hasCoord bool
	body     string
	input    string
	editing  bool
}

func newPreviewTab(r *mbtiles.Reader, md *mbtiles.Metadata, yconv tiles.YConvention, s Styles) *previewTab {
	return &previewTab{reader: r, md: md, yconv: yconv, styles: s}
}

func (t *previewTab) Title() string { return "Preview" }
func (t *previewTab) Init() tea.Cmd { return nil }

func (t *previewTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		if t.editing {
			switch k.String() {
			case "enter":
				t.editing = false
				t.parseAndLoad(t.input)
			case "esc":
				t.editing = false
				t.input = ""
			case "backspace":
				if len(t.input) > 0 {
					t.input = t.input[:len(t.input)-1]
				}
			default:
				if len(k.String()) == 1 {
					t.input += k.String()
				}
			}
			return t, nil
		}
		if k.String() == "i" {
			t.editing = true
			t.input = ""
			return t, nil
		}
	}
	return t, nil
}

func (t *previewTab) parseAndLoad(s string) {
	var z, x, y int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d/%d/%d", &z, &x, &y); err != nil {
		t.body = t.styles.StatusWarn.Render("expected z/x/y, got " + s)
		return
	}
	t.z, t.x, t.y, t.hasCoord = z, x, y, true
	t.body = t.load(context.Background(), z, x, y)
}

func (t *previewTab) load(ctx context.Context, z, x, y int) string {
	storageY := y
	if t.yconv == tiles.YTMS {
		storageY = tiles.FlipY(z, y)
	}
	blob, err := t.reader.Tile(ctx, z, x, storageY)
	if err != nil {
		return t.styles.StatusWarn.Render("read tile: " + err.Error())
	}
	if blob == nil {
		return t.styles.StatusWarn.Render(fmt.Sprintf("no tile at %d/%d/%d", z, x, y))
	}

	declared := ""
	if t.md != nil {
		declared = t.md.Format
	}
	sniffed := mbtiles.DetectFormat(blob)
	gz := ""
	if mbtiles.IsGzip(blob) {
		gz = " · gzip-compressed"
	}

	header := fmt.Sprintf("Tile (%d, %d, %d)  ·  %s  ·  %s%s",
		z, x, y, sniffed, humanBytes(int64(len(blob))), gz)

	var body strings.Builder
	body.WriteString(t.styles.Header.Render(header) + "\n")
	if declared != "" && declared != sniffed {
		body.WriteString(t.styles.StatusWarn.Render(
			fmt.Sprintf("declared format %q ≠ sniffed %q", declared, sniffed)) + "\n")
	}

	switch sniffed {
	case "pbf":
		body.WriteString(renderVectorSummary(blob))
	case "png", "jpg", "webp":
		body.WriteString(t.styles.Subtle.Render(
			fmt.Sprintf("(raster preview not rendered in v1; pipe `mbtiles-inspect extract %s %d/%d/%d` to a viewer)",
				t.reader.Path(), z, x, y)))
	default:
		body.WriteString(t.styles.Subtle.Render("unknown format; nothing to preview"))
	}
	return body.String()
}

func (t *previewTab) View() string {
	var b strings.Builder
	if t.editing {
		b.WriteString(t.styles.Subtle.Render("enter z/x/y (XYZ), e.g. 2/0/0:") + "\n")
		b.WriteString("> " + t.input + "_")
		return b.String()
	}
	b.WriteString(t.styles.Subtle.Render("press i to load a tile by z/x/y (XYZ)") + "\n\n")
	if !t.hasCoord {
		return b.String() + t.styles.Subtle.Render("(no tile selected)")
	}
	b.WriteString(t.body)
	return b.String()
}

// renderVectorSummary decodes the MVT and lists layers with feature counts
// and the dominant geometry type per SPEC.md preview output.
func renderVectorSummary(blob []byte) string {
	raw, err := mbtiles.MaybeUngzip(blob)
	if err != nil {
		return "decompress: " + err.Error()
	}
	layers, err := mvt.Unmarshal(raw)
	if err != nil {
		return fmt.Sprintf("vector decode failed: %v\n(blob: %d bytes uncompressed)", err, len(raw))
	}
	if len(layers) == 0 {
		return "vector tile decoded; no layers"
	}
	type entry struct {
		name     string
		features int
		geomType string
	}
	rows := make([]entry, 0, len(layers))
	for _, l := range layers {
		rows = append(rows, entry{name: l.Name, features: len(l.Features), geomType: dominantGeomType(l)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].features > rows[j].features })

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Layers (%d):\n", len(rows)))
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %-20s %6d features  (%s)\n", r.name, r.features, r.geomType))
	}
	return b.String()
}

func dominantGeomType(l *mvt.Layer) string {
	counts := map[string]int{}
	for _, f := range l.Features {
		counts[geomTypeOf(f.Geometry)]++
	}
	best := ""
	bestN := -1
	for k, v := range counts {
		if v > bestN {
			best, bestN = k, v
		}
	}
	if best == "" {
		return "unknown"
	}
	return best
}

func geomTypeOf(g orb.Geometry) string {
	if g == nil {
		return "unknown"
	}
	return g.GeoJSONType()
}
