package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rilus/mbtiles-inspect/internal/gaps"
	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

type gapsTab struct {
	reader *mbtiles.Reader
	md     *mbtiles.Metadata
	yconv  tiles.YConvention
	styles Styles

	zooms   []int
	zoomIdx int
	current *gaps.Result
	loadErr error

	width, height int
	status        string
}

func newGapsTab(ctx context.Context, r *mbtiles.Reader, md *mbtiles.Metadata, yconv tiles.YConvention, s Styles) *gapsTab {
	stats, _ := r.ZoomStats(ctx)
	zlist := make([]int, 0, len(stats))
	for _, st := range stats {
		zlist = append(zlist, st.Zoom)
	}
	t := &gapsTab{reader: r, md: md, yconv: yconv, styles: s, zooms: zlist}
	t.loadCurrent(ctx)
	return t
}

func (t *gapsTab) loadCurrent(ctx context.Context) {
	t.current = nil
	t.loadErr = nil
	if len(t.zooms) == 0 {
		return
	}
	if t.md == nil || t.md.Bounds == nil {
		t.loadErr = fmt.Errorf("file has no bounds metadata; gaps unavailable")
		return
	}
	res, err := gaps.Detect(ctx, t.reader, t.md, t.zooms[t.zoomIdx], t.yconv)
	if err != nil {
		t.loadErr = err
		return
	}
	t.current = res
}

func (t *gapsTab) Title() string { return "Gaps" }
func (t *gapsTab) Init() tea.Cmd { return nil }

func (t *gapsTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = m.Width, m.Height
	case tea.KeyMsg:
		switch m.String() {
		case "[":
			if len(t.zooms) > 0 {
				t.zoomIdx = (t.zoomIdx - 1 + len(t.zooms)) % len(t.zooms)
				t.loadCurrent(context.Background())
			}
		case "]":
			if len(t.zooms) > 0 {
				t.zoomIdx = (t.zoomIdx + 1) % len(t.zooms)
				t.loadCurrent(context.Background())
			}
		case "e":
			if t.current != nil && len(t.current.Missing) > 0 {
				path, err := exportGapsGeoJSON(t.current)
				if err != nil {
					t.status = "export failed: " + err.Error()
				} else {
					t.status = "exported " + path
				}
			} else {
				t.status = "nothing to export"
			}
		}
	}
	return t, nil
}

func (t *gapsTab) View() string {
	if t.loadErr != nil {
		return t.styles.StatusWarn.Render(t.loadErr.Error())
	}
	if t.current == nil {
		return t.styles.Subtle.Render("no zoom levels")
	}
	r := t.current
	header := fmt.Sprintf("zoom %d  ·  %d/%d present (%.1f%% coverage)  ·  [/] switch · e export geojson",
		r.Zoom, r.Present, r.Expected, r.CoveragePct())
	body := renderGapsGrid(r, t.width, t.height-8)
	out := t.styles.Subtle.Render(header) + "\n" + body
	if t.status != "" {
		out += "\n" + t.styles.StatusOK.Render(t.status)
	}
	return out
}

// renderGapsGrid downsamples the missing-tile rectangle into the available
// terminal cells. Each cell shows a glyph based on the fraction of tiles
// missing inside it.
func renderGapsGrid(r *gaps.Result, w, h int) string {
	if r == nil {
		return ""
	}
	rng := tiles.BBoxToRange(r.BBox, r.Zoom)
	cols := rng.XMax - rng.XMin + 1
	rows := rng.YMax - rng.YMin + 1
	if cols <= 0 || rows <= 0 {
		return ""
	}

	maxCols := w
	if maxCols < 10 {
		maxCols = 10
	}
	if maxCols > cols {
		maxCols = cols
	}
	maxRows := h
	if maxRows < 5 {
		maxRows = 5
	}
	if maxRows > rows {
		maxRows = rows
	}

	cellW := float64(cols) / float64(maxCols)
	cellH := float64(rows) / float64(maxRows)
	missing := make(map[[2]int]struct{}, len(r.Missing))
	for _, c := range r.Missing {
		missing[[2]int{c.X, c.Y}] = struct{}{}
	}

	var b strings.Builder
	for cy := 0; cy < maxRows; cy++ {
		for cx := 0; cx < maxCols; cx++ {
			x0 := rng.XMin + int(float64(cx)*cellW)
			x1 := rng.XMin + int(float64(cx+1)*cellW)
			if x1 == x0 {
				x1 = x0 + 1
			}
			y0 := rng.YMin + int(float64(cy)*cellH)
			y1 := rng.YMin + int(float64(cy+1)*cellH)
			if y1 == y0 {
				y1 = y0 + 1
			}
			total := 0
			miss := 0
			for x := x0; x < x1; x++ {
				for y := y0; y < y1; y++ {
					total++
					if _, ok := missing[[2]int{x, y}]; ok {
						miss++
					}
				}
			}
			b.WriteString(glyphFor(miss, total))
		}
		b.WriteByte('\n')
	}
	b.WriteString("legend: █ all missing  ▓ mostly  ▒ half  ░ few  · all present\n")
	return b.String()
}

func glyphFor(miss, total int) string {
	if total == 0 {
		return "·"
	}
	frac := float64(miss) / float64(total)
	switch {
	case frac == 0:
		return "·"
	case frac < 0.25:
		return "░"
	case frac < 0.50:
		return "▒"
	case frac < 0.99:
		return "▓"
	default:
		return "█"
	}
}

func exportGapsGeoJSON(r *gaps.Result) (string, error) {
	dir := os.TempDir()
	name := fmt.Sprintf("mbtiles-gaps-z%d-%d.geojson", r.Zoom, time.Now().Unix())
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := gaps.WriteGeoJSON(f, r); err != nil {
		return "", err
	}
	return path, nil
}
