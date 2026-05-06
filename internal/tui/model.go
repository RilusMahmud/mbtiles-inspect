package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// Tab is implemented by each tab sub-model. It extends tea.Model with a
// short title for the tab strip.
type Tab interface {
	tea.Model
	Title() string
}

// Model is the top-level bubbletea model. Tabs are kept as a flat slice and
// indexed by active.
type Model struct {
	reader *mbtiles.Reader
	md     *mbtiles.Metadata
	yconv  tiles.YConvention
	styles Styles

	tabs   []Tab
	active int

	width, height int
	help          bool
	statusMsg     string
}

// New constructs the top-level model. The reader is borrowed; callers retain
// ownership and must Close it after the program exits.
func New(ctx context.Context, r *mbtiles.Reader) (*Model, error) {
	md, err := r.Metadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	yconv, err := mbtiles.DetectYConvention(ctx, r, md)
	if err != nil {
		return nil, fmt.Errorf("detect y convention: %w", err)
	}

	styles := NewStyles()
	m := &Model{
		reader: r,
		md:     md,
		yconv:  yconv,
		styles: styles,
	}
	m.tabs = []Tab{
		newMetadataTab(md, yconv, r.Path(), styles),
		newZoomsTab(ctx, r, md, yconv, styles),
		newTilesTab(ctx, r, md, yconv, styles),
		newGapsTab(ctx, r, md, yconv, styles),
		newPreviewTab(r, md, yconv, styles),
	}
	return m, nil
}

// Init kicks off any per-tab initial commands.
func (m *Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.tabs))
	for _, t := range m.tabs {
		if c := t.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// Update handles global keys and routes everything else to the active tab.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Forward to every tab so they can size themselves whether visible
		// or not. Each tab returns its own updated model.
		for i, t := range m.tabs {
			updated, _ := t.Update(msg)
			m.tabs[i] = updated.(Tab)
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.help = !m.help
			return m, nil
		case "tab":
			m.active = (m.active + 1) % len(m.tabs)
			return m, nil
		case "shift+tab":
			m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
			return m, nil
		case "m":
			m.active = 0
			return m, nil
		case "z":
			m.active = 1
			return m, nil
		case "t":
			m.active = 2
			return m, nil
		case "g":
			m.active = 3
			return m, nil
		case "p":
			m.active = 4
			return m, nil
		}
	case statusMsg:
		m.statusMsg = string(msg)
		return m, nil
	}

	updated, cmd := m.tabs[m.active].Update(msg)
	m.tabs[m.active] = updated.(Tab)
	return m, cmd
}

// View renders the title bar, tab strip, active tab body, and help footer.
func (m *Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	var b strings.Builder

	header := m.md.Name
	if header == "" {
		header = m.reader.Path()
	}
	b.WriteString(m.styles.Title.Render("mbtiles-inspect — "+header) + "\n")

	tabs := make([]string, len(m.tabs))
	for i, t := range m.tabs {
		label := t.Title()
		if i == m.active {
			tabs[i] = m.styles.ActiveTab.Render(label)
		} else {
			tabs[i] = m.styles.Tab.Render(label)
		}
	}
	b.WriteString(m.styles.TabRow.Render(lipgloss.JoinHorizontal(lipgloss.Top, tabs...)) + "\n")

	body := m.tabs[m.active].View()
	b.WriteString(body)

	if m.statusMsg != "" {
		b.WriteString("\n" + m.styles.Subtle.Render(m.statusMsg))
	}

	helpLine := "tab/shift-tab cycle · m/z/t/g/p jump · q quit · ? help"
	if m.help {
		helpLine = m.helpText()
	}
	b.WriteString("\n" + m.styles.Help.Render(helpLine))
	return b.String()
}

func (m *Model) helpText() string {
	return strings.Join([]string{
		"keys:",
		"  m  metadata    z  zooms    t  tiles    g  gaps    p  preview",
		"  tab / shift-tab   next / previous tab",
		"  ↑ ↓ / pgup pgdn   navigate within a tab",
		"  q                 quit       ?  toggle this help",
	}, "\n")
}

// statusMsg sets the small status line under the tab body. Tabs emit this
// to acknowledge actions like "exported gaps to /tmp/...".
type statusMsg string
