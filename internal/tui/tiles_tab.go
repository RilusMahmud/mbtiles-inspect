package tui

import (
	"context"
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

const tilesPageMax = 10000

type tilesTab struct {
	reader *mbtiles.Reader
	md     *mbtiles.Metadata
	yconv  tiles.YConvention
	styles Styles

	zooms     []int
	zoomIdx   int
	allRows   []mbtiles.TileRow
	truncated bool

	sortBySize bool
	tbl        table.Model
}

func newTilesTab(ctx context.Context, r *mbtiles.Reader, md *mbtiles.Metadata, yconv tiles.YConvention, s Styles) *tilesTab {
	stats, _ := r.ZoomStats(ctx)
	zlist := make([]int, 0, len(stats))
	for _, st := range stats {
		zlist = append(zlist, st.Zoom)
	}
	t := &tilesTab{reader: r, md: md, yconv: yconv, styles: s, zooms: zlist}
	if len(zlist) > 0 {
		t.loadZoom(ctx, zlist[0])
	}
	return t
}

func (t *tilesTab) loadZoom(ctx context.Context, z int) {
	rows, err := t.reader.TilesAtZoom(ctx, z)
	if err != nil {
		t.allRows = nil
	} else {
		t.allRows = rows
	}
	t.truncated = len(t.allRows) > tilesPageMax
	t.refreshTable()
}

func (t *tilesTab) refreshTable() {
	rows := t.allRows
	if t.sortBySize {
		sorted := make([]mbtiles.TileRow, len(rows))
		copy(sorted, rows)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Size > sorted[j].Size })
		rows = sorted
	}
	if len(rows) > tilesPageMax {
		rows = rows[:tilesPageMax]
	}

	cols := []table.Column{
		{Title: "z", Width: 4},
		{Title: "x", Width: 8},
		{Title: "y (storage)", Width: 12},
		{Title: "y (xyz)", Width: 9},
		{Title: "size", Width: 10},
	}
	trows := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		yXYZ := r.Y
		if t.yconv == tiles.YTMS {
			yXYZ = tiles.FlipY(r.Z, r.Y)
		}
		trows = append(trows, table.Row{
			fmt.Sprintf("%d", r.Z),
			fmt.Sprintf("%d", r.X),
			fmt.Sprintf("%d", r.Y),
			fmt.Sprintf("%d", yXYZ),
			humanBytes(r.Size),
		})
	}
	tbl := table.New(table.WithColumns(cols), table.WithRows(trows), table.WithFocused(true), table.WithHeight(15))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true)
	tbl.SetStyles(st)
	t.tbl = tbl
}

func (t *tilesTab) Title() string { return "Tiles" }
func (t *tilesTab) Init() tea.Cmd { return nil }

func (t *tilesTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		h := m.Height - 8
		if h < 5 {
			h = 5
		}
		t.tbl.SetHeight(h)
	case tea.KeyMsg:
		switch m.String() {
		case "[":
			if len(t.zooms) > 0 {
				t.zoomIdx = (t.zoomIdx - 1 + len(t.zooms)) % len(t.zooms)
				t.loadZoom(context.Background(), t.zooms[t.zoomIdx])
				return t, nil
			}
		case "]":
			if len(t.zooms) > 0 {
				t.zoomIdx = (t.zoomIdx + 1) % len(t.zooms)
				t.loadZoom(context.Background(), t.zooms[t.zoomIdx])
				return t, nil
			}
		case "s":
			t.sortBySize = !t.sortBySize
			t.refreshTable()
			return t, nil
		}
	}
	var cmd tea.Cmd
	t.tbl, cmd = t.tbl.Update(msg)
	return t, cmd
}

func (t *tilesTab) View() string {
	if len(t.zooms) == 0 {
		return t.styles.Subtle.Render("no tiles in file")
	}
	header := fmt.Sprintf("zoom: %d   (use [/] to switch · s to toggle sort by size)", t.zooms[t.zoomIdx])
	if t.sortBySize {
		header += "  · sort=size"
	}
	if t.truncated {
		header += t.styles.StatusWarn.Render(fmt.Sprintf("  · showing first %d of %d", tilesPageMax, len(t.allRows)))
	}
	return t.styles.Subtle.Render(header) + "\n" + t.tbl.View()
}
