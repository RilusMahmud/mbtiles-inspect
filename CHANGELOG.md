# Changelog

All notable changes to `mbtiles-inspect` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versioning follows [Semantic Versioning](https://semver.org/).

The `--json` output of `info`, `gaps`, and `diff` is part of the public API.
Backwards-incompatible changes to JSON shape require a major version bump.

## [Unreleased]

## [0.1.0] — 2026-05-07

### Added
- MIT `LICENSE` file.
- GoReleaser-driven release pipeline (`.goreleaser.yaml` + GitHub Actions
  workflow) producing Linux and macOS binaries (amd64 + arm64) with
  SHA-256 checksums on every `v*` tag push. Manual `workflow_dispatch`
  produces a snapshot dry-run uploaded as a workflow artifact.
- `ci` GitHub Actions workflow running `gofmt -s`, `go vet`, and
  `go test ./...` on push to `main` and on pull requests.
- `--version` now reports the source commit and build date alongside
  the version string (e.g. `v0.1.0 (abc1234, 2026-05-07T12:34:56Z)`),
  populated via `-ldflags` at release time.
- `diff` subcommand comparing two mbtiles files: metadata add/remove/modify,
  per-zoom tile-count and size deltas, and up to 10 sample tiles unique to
  each side. Tile coordinates in `only_in_a` / `only_in_b` are normalized to
  XYZ Y convention (matching `gaps` and `extract`), so diffs are meaningful
  even when the two files store Y differently. Human and JSON output.
- `verify --sample N` flag: spot-checks N tiles per zoom for corruption
  (gzip integrity + format magic). Default 8.
- `mbtiles.ValidateTile` and `mbtiles.SampleTilesAtZoom` helpers.
- `mbtiles.CoordCursor` — streaming `(z, x, y)` iterator used by `diff`.
- TUI (`mbtiles-inspect <file>` or `tui <file>`): five tabs — Metadata,
  Zooms, Tiles, Gaps, Preview. Zoom switcher `[`/`]` in Tiles and Gaps,
  GeoJSON export `e` in Gaps, MVT layer summary in Preview.
- `corrupt.mbtiles` test fixture and `TestVerifyCorruptFails` integration
  test.

### Fixed
- Slippy-map test reference for Dhaka at z=14 corrected from `(12055, 7642)`
  to the OSM-formula value `(12306, 7075)`. The math was correct; only the
  test expectation and SPEC examples were wrong.
- SPEC `gaps` example: `4,210 of 32,768` was bogus arithmetic for the Dhaka
  bbox at z=14 (the real expected count is ~80). Updated example.
- `info` integration test asserted on a literal tab character that
  `tabwriter` always expands to spaces. Now asserts on stable substrings.

### Dependencies
- Added: `charmbracelet/bubbletea`, `bubbles`, `lipgloss`, `muesli/termenv`,
  `paulmach/orb` (for MVT decode in Preview tab).

## [0.0.0] — initial scaffold

- `info`, `gaps`, `verify`, `extract` subcommands (M1 + M2).
- Pure-Go SQLite via `modernc.org/sqlite`.
- Read-only mbtiles reader, metadata parser, slippy-map math, gap detection,
  TMS/XYZ Y-convention auto-detect, GeoJSON export.
