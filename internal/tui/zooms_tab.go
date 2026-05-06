package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rilus/mbtiles-inspect/internal/gaps"
	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

type zoomsTab struct {
	reader *mbtiles.Reader
	md     *mbtiles.Metadata
	yconv  tiles.YConvention
	styles Styles
	tbl    table.Model
}

func newZoomsTab(ctx context.Context, r *mbtiles.Reader, md *mbtiles.Metadata, yconv tiles.YConvention, s Styles) *zoomsTab {
	cols := []table.Column{
		{Title: "z", Width: 4},
		{Title: "tiles", Width: 10},
		{Title: "size", Width: 10},
		{Title: "min", Width: 8},
		{Title: "avg", Width: 8},
		{Title: "max", Width: 8},
		{Title: "coverage", Width: 10},
	}
	rows := loadZoomRows(ctx, r, md, yconv)

	tbl := table.New(table.WithColumns(cols), table.WithRows(rows), table.WithFocused(true), table.WithHeight(12))
	st := table.DefaultStyles()
	st.Header = st.Header.Bold(true)
	tbl.SetStyles(st)

	return &zoomsTab{reader: r, md: md, yconv: yconv, styles: s, tbl: tbl}
}

func loadZoomRows(ctx context.Context, r *mbtiles.Reader, md *mbtiles.Metadata, yconv tiles.YConvention) []table.Row {
	stats, err := r.ZoomStats(ctx)
	if err != nil {
		return []table.Row{{"err", err.Error(), "", "", "", "", ""}}
	}
	rows := make([]table.Row, 0, len(stats))
	for _, s := range stats {
		coverage := "—"
		if md != nil && md.Bounds != nil {
			res, err := gaps.Detect(ctx, r, md, s.Zoom, yconv)
			if err == nil {
				cov := res.CoveragePct()
				if cov > 100 {
					cov = 100
				}
				coverage = fmt.Sprintf("%.1f%%", cov)
			}
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", s.Zoom),
			fmt.Sprintf("%d", s.TileCount),
			humanBytes(s.TotalSize),
			humanBytes(s.MinSize),
			humanBytes(s.AvgSize),
			humanBytes(s.MaxSize),
			coverage,
		})
	}
	return rows
}

func (t *zoomsTab) Title() string { return "Zooms" }
func (t *zoomsTab) Init() tea.Cmd { return nil }

func (t *zoomsTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		h := m.Height - 7
		if h < 5 {
			h = 5
		}
		t.tbl.SetHeight(h)
	}
	var cmd tea.Cmd
	t.tbl, cmd = t.tbl.Update(msg)
	return t, cmd
}

func (t *zoomsTab) View() string { return t.tbl.View() }

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
