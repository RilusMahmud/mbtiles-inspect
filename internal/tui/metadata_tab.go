package tui

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

type metadataTab struct {
	md     *mbtiles.Metadata
	yconv  tiles.YConvention
	path   string
	styles Styles
	vp     viewport.Model
	ready  bool
}

func newMetadataTab(md *mbtiles.Metadata, yconv tiles.YConvention, path string, s Styles) *metadataTab {
	return &metadataTab{md: md, yconv: yconv, path: path, styles: s}
}

func (t *metadataTab) Title() string { return "Metadata" }

func (t *metadataTab) Init() tea.Cmd { return nil }

func (t *metadataTab) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// reserve 5 lines for title + tabs + help
		h := msg.Height - 5
		if h < 5 {
			h = 5
		}
		if !t.ready {
			t.vp = viewport.New(msg.Width, h)
			t.vp.SetContent(t.render())
			t.ready = true
		} else {
			t.vp.Width, t.vp.Height = msg.Width, h
			t.vp.SetContent(t.render())
		}
	case tea.KeyMsg:
		// Let viewport handle scroll keys.
	}
	if t.ready {
		var cmd tea.Cmd
		t.vp, cmd = t.vp.Update(msg)
		return t, cmd
	}
	return t, nil
}

func (t *metadataTab) View() string {
	if !t.ready {
		return "loading…"
	}
	return t.vp.View()
}

func (t *metadataTab) render() string {
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Path:\t%s\n", t.path)
	if t.md != nil {
		md := t.md
		if md.Name != "" {
			fmt.Fprintf(tw, "Name:\t%s\n", md.Name)
		}
		if md.Format != "" {
			fmt.Fprintf(tw, "Format:\t%s\n", md.Format)
		}
		if md.Bounds != nil {
			fmt.Fprintf(tw, "Bounds:\t%.4f, %.4f, %.4f, %.4f\n",
				md.Bounds.LonMin, md.Bounds.LatMin, md.Bounds.LonMax, md.Bounds.LatMax)
		}
		if md.Center != nil {
			fmt.Fprintf(tw, "Center:\t%.4f, %.4f, %d\n", md.Center.Lon, md.Center.Lat, md.Center.Zoom)
		}
		if md.MinZoom != nil && md.MaxZoom != nil {
			fmt.Fprintf(tw, "Zooms:\t%d — %d\n", *md.MinZoom, *md.MaxZoom)
		}
		fmt.Fprintf(tw, "Tile y:\t%s\n", t.yconv)
		if md.Attribution != "" {
			fmt.Fprintf(tw, "Attribution:\t%s\n", md.Attribution)
		}
		if md.Description != "" {
			fmt.Fprintf(tw, "Description:\t%s\n", md.Description)
		}
		if md.Type != "" {
			fmt.Fprintf(tw, "Type:\t%s\n", md.Type)
		}
		if md.Version != "" {
			fmt.Fprintf(tw, "Version:\t%s\n", md.Version)
		}
		if md.Scheme != "" {
			fmt.Fprintf(tw, "Scheme:\t%s\n", md.Scheme)
		}
		for k, v := range md.Extra {
			fmt.Fprintf(tw, "%s:\t%s\n", k, v)
		}
	}
	tw.Flush()

	if t.md != nil && t.md.JSON != nil && len(t.md.JSON.VectorLayers) > 0 {
		fmt.Fprintf(&b, "\n%s\n", t.styles.Header.Render(fmt.Sprintf("Vector layers (%d)", len(t.md.JSON.VectorLayers))))
		ltw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		for _, vl := range t.md.JSON.VectorLayers {
			zr := ""
			if vl.MinZoom != nil && vl.MaxZoom != nil {
				zr = fmt.Sprintf("minzoom %d, maxzoom %d", *vl.MinZoom, *vl.MaxZoom)
			}
			fmt.Fprintf(ltw, "  %s\t%s\n", vl.ID, zr)
			if vl.Description != "" {
				fmt.Fprintf(ltw, "    %s\t\n", t.styles.Subtle.Render(vl.Description))
			}
		}
		ltw.Flush()
	}
	return b.String()
}
