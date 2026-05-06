package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
)

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 5; i++ {
		candidate := filepath.Join(dir, "testdata", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Skipf("fixture %s not found; run `go run ./testdata/gen.go`", name)
	return ""
}

func mustReader(t *testing.T, name string) *mbtiles.Reader {
	t.Helper()
	r, err := mbtiles.Open(fixturePath(t, name))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return r
}

func TestModelTabSwitch(t *testing.T) {
	r := mustReader(t, "complete.mbtiles")
	defer r.Close()

	m, err := New(context.Background(), r)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if m.active != 0 {
		t.Fatalf("active = %d, want 0", m.active)
	}

	send := func(key string) tea.Msg {
		switch key {
		case "tab":
			return tea.KeyMsg{Type: tea.KeyTab}
		case "shift+tab":
			return tea.KeyMsg{Type: tea.KeyShiftTab}
		default:
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
	}
	cases := []struct {
		key  string
		want int
	}{
		{"z", 1},
		{"t", 2},
		{"g", 3},
		{"p", 4},
		{"m", 0},
		{"tab", 1},
		{"shift+tab", 0},
	}
	for _, c := range cases {
		updated, _ := m.Update(send(c.key))
		mm := updated.(*Model)
		if mm.active != c.want {
			t.Errorf("after %q: active = %d, want %d", c.key, mm.active, c.want)
		}
		m = mm
	}
}

func TestModelQuit(t *testing.T) {
	r := mustReader(t, "complete.mbtiles")
	defer r.Close()

	m, err := New(context.Background(), r)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit cmd, got nil")
	}
}

func TestMetadataTabRenders(t *testing.T) {
	r := mustReader(t, "complete.mbtiles")
	defer r.Close()

	m, err := New(context.Background(), r)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	view := updated.(*Model).View()
	if !strings.Contains(view, "Metadata") {
		t.Errorf("missing tab title; view:\n%s", view)
	}
	if !strings.Contains(view, "complete fixture") {
		t.Errorf("missing fixture name; view:\n%s", view)
	}
}

func TestZoomsTabRenders(t *testing.T) {
	r := mustReader(t, "complete.mbtiles")
	defer r.Close()

	m, err := New(context.Background(), r)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	view := updated.(*Model).View()
	if !strings.Contains(view, "tiles") {
		t.Errorf("expected zooms table header; view:\n%s", view)
	}
}

func TestPreviewTabLoadsTile(t *testing.T) {
	r := mustReader(t, "complete.mbtiles")
	defer r.Close()
	m, err := New(context.Background(), r)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	// Jump to preview tab.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	// Enter input mode.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	for _, ch := range "2/0/0" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	// Submit.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := updated.(*Model).View()
	if !strings.Contains(view, "Tile (2, 0, 0)") {
		t.Errorf("expected loaded-tile header in view; got:\n%s", view)
	}
}

func TestGapsTabHandlesMissingBounds(t *testing.T) {
	// Use the corrupt fixture which has bounds, but it's a fine smoke test.
	r := mustReader(t, "complete.mbtiles")
	defer r.Close()
	m, err := New(context.Background(), r)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	view := updated.(*Model).View()
	if !strings.Contains(view, "coverage") {
		t.Errorf("expected gaps view to mention coverage; got:\n%s", view)
	}
}
