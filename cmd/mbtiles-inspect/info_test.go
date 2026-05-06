package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
)

// fixturePath resolves a testdata fixture relative to the project root.
//
// The test binary runs from the package directory; walk up until we find
// the testdata folder.
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
	t.Skipf("fixture %s not found; run `go run ./testdata/gen.go` first", name)
	return ""
}

// captureStdout runs fn with os.Stdout redirected to a buffer.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	orig := os.Stdout
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = wr

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, rd)
		close(done)
	}()

	runErr := fn()
	wr.Close()
	<-done
	os.Stdout = orig
	return buf.String(), runErr
}

func TestInfoComplete(t *testing.T) {
	path := fixturePath(t, "complete.mbtiles")
	out, err := captureStdout(t, func() error {
		return runInfo(context.Background(), path, false)
	})
	if err != nil {
		t.Fatalf("runInfo: %v", err)
	}
	for _, want := range []string{"Format:  pbf", "Bounds:", "Zoom coverage", "tiles", "coverage", "100.0%"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestInfoJSON(t *testing.T) {
	path := fixturePath(t, "complete.mbtiles")
	out, err := captureStdout(t, func() error {
		return runInfo(context.Background(), path, true)
	})
	if err != nil {
		t.Fatalf("runInfo: %v", err)
	}
	for _, want := range []string{`"file":`, `"size_bytes":`, `"tile_y_convention":`, `"zoom_levels":`} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestInfoNotMBTiles(t *testing.T) {
	path := fixturePath(t, "notmbtiles.sqlite")
	_, err := captureStdout(t, func() error {
		return runInfo(context.Background(), path, false)
	})
	if !errors.Is(err, mbtiles.ErrNotMBTiles) {
		t.Fatalf("expected ErrNotMBTiles, got %v", err)
	}
	if exitCodeFor(err) != 2 {
		t.Errorf("expected exit code 2, got %d", exitCodeFor(err))
	}
}

func TestVerifyGappyFails(t *testing.T) {
	path := fixturePath(t, "gappy.mbtiles")
	_, err := captureStdout(t, func() error {
		return runVerify(context.Background(), path, 95.0, 0)
	})
	if err == nil {
		t.Fatal("expected verify to fail on gappy fixture")
	}
	if exitCodeFor(err) != 1 {
		t.Errorf("expected exit code 1, got %d (err=%v)", exitCodeFor(err), err)
	}
}

func TestVerifyCompleteOK(t *testing.T) {
	path := fixturePath(t, "complete.mbtiles")
	_, err := captureStdout(t, func() error {
		return runVerify(context.Background(), path, 95.0, 8)
	})
	if err != nil {
		t.Fatalf("verify failed on complete fixture: %v", err)
	}
}

func TestDiffComplete(t *testing.T) {
	a := fixturePath(t, "complete.mbtiles")
	b := fixturePath(t, "gappy.mbtiles")
	out, err := captureStdout(t, func() error {
		return runDiff(context.Background(), a, b, false)
	})
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	// Coords are reported in XYZ Y. The gappy fixture is stored TMS, so the
	// kept-in-A samples appear at XYZ y values like 2/0/1.
	for _, want := range []string{"Per-zoom:", "Δ tiles", "-8", "Only in A", "2/0/1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestDiffJSON(t *testing.T) {
	a := fixturePath(t, "complete.mbtiles")
	b := fixturePath(t, "gappy.mbtiles")
	out, err := captureStdout(t, func() error {
		return runDiff(context.Background(), a, b, true)
	})
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	for _, want := range []string{`"a":`, `"zooms":`, `"delta_tiles": -8`, `"only_in_a":`} {
		if !strings.Contains(out, want) {
			t.Errorf("json output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestDiffIdentical(t *testing.T) {
	a := fixturePath(t, "complete.mbtiles")
	out, err := captureStdout(t, func() error {
		return runDiff(context.Background(), a, a, false)
	})
	if err != nil {
		t.Fatalf("runDiff: %v", err)
	}
	if !strings.Contains(out, "Tile sets match") {
		t.Errorf("expected 'Tile sets match' in output\n%s", out)
	}
	if !strings.Contains(out, "Metadata: identical") {
		t.Errorf("expected 'Metadata: identical' in output\n%s", out)
	}
}

func TestVerifyCorruptFails(t *testing.T) {
	path := fixturePath(t, "corrupt.mbtiles")
	_, err := captureStdout(t, func() error {
		return runVerify(context.Background(), path, 0.0, 8)
	})
	if err == nil {
		t.Fatal("expected verify to fail on corrupt fixture")
	}
	if exitCodeFor(err) != 1 {
		t.Errorf("expected exit code 1, got %d (err=%v)", exitCodeFor(err), err)
	}
}
