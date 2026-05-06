// Package tui implements the bubbletea-based interactive view of an mbtiles
// file. The top-level Model holds a list of per-tab sub-models; each tab is
// its own tea.Model.
package tui

import "github.com/charmbracelet/lipgloss"

// Styles bundles lipgloss styles shared across tabs. Constructed once at
// startup so palette decisions live in one place.
type Styles struct {
	Title      lipgloss.Style
	Tab        lipgloss.Style
	ActiveTab  lipgloss.Style
	TabRow     lipgloss.Style
	Subtle     lipgloss.Style
	Help       lipgloss.Style
	StatusOK   lipgloss.Style
	StatusWarn lipgloss.Style
	Header     lipgloss.Style
}

// NewStyles returns the default style set.
func NewStyles() Styles {
	return Styles{
		Title:      lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
		Tab:        lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("245")),
		ActiveTab:  lipgloss.NewStyle().Padding(0, 2).Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("63")),
		TabRow:     lipgloss.NewStyle().Padding(0, 0, 1, 0),
		Subtle:     lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
		Help:       lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1),
		StatusOK:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		StatusWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("214")),
		Header:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("213")),
	}
}
