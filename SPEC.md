# mbtiles-inspect

A terminal tool for inspecting `.mbtiles` files. CLI subcommands for scripting
and CI, plus a TUI for interactive exploration.

## Goals

- Answer common questions about an mbtiles file in under 5 seconds without writing SQL.
- Detect coverage gaps geographically, not just numerically.
- Be usable in CI (`verify` exits non-zero on problems).
- Single static Go binary, no runtime dependencies.

## Non-goals

- Editing or writing mbtiles files. Read-only.
- Serving tiles. Use tileserver-gl.
- Format conversion (mbtiles → pmtiles, etc).
- GUI. Terminal only.

## Background: mbtiles format

`.mbtiles` is a SQLite file. Two relevant tables:

```sql
CREATE TABLE metadata (name TEXT, value TEXT);
CREATE TABLE tiles (
  zoom_level INTEGER,
  tile_column INTEGER,
  tile_row INTEGER,
  tile_data BLOB
);
```

`metadata` holds keys like `name`, `format` (pbf/png/jpg/webp), `bounds`
(comma-separated lon_min,lat_min,lon_max,lat_max), `minzoom`, `maxzoom`,
`center`, `attribution`, `vector_layers` (JSON).

`tiles` is keyed by `(zoom_level, tile_column, tile_row)`. The spec stores
`tile_row` in **TMS** convention (y flipped from XYZ). Many real files do not
follow the spec — we must auto-detect.

Reference: https://github.com/mapbox/mbtiles-spec/blob/master/1.3/spec.md

## Commands

### Subcommands (non-TUI, scripting-friendly)

```
mbtiles-inspect info <file>              # metadata + zoom summary
mbtiles-inspect gaps <file> [--zoom N]   # list missing tiles
mbtiles-inspect verify <file>            # CI mode, exit non-zero on issues
mbtiles-inspect diff <a> <b>             # compare two files
mbtiles-inspect extract <file> z/x/y     # write a single tile to stdout
mbtiles-inspect tui <file>               # launch the TUI (also: no subcommand)
```

Global flags:

- `--json` — JSON output for `info`, `gaps`, `diff`. Default human-readable.
- `--quiet` / `-q` — suppress non-essential output.
- `--no-color` — disable ANSI color.

### TUI

`mbtiles-inspect <file>` (no subcommand) launches the TUI. Five tabs:

1. **Metadata** — pretty-printed metadata table, `vector_layers` JSON expanded.
2. **Zooms** — table of zoom level, tile count, total size, min/avg/max tile size, coverage %.
3. **Tiles** — sortable list of tiles. Filter by zoom, sort by size. Enter → Preview.
4. **Gaps** — per-zoom missing-tile visualization as ASCII grid. `e` to export GeoJSON.
5. **Preview** — render a selected tile as ASCII. Raster as halfblocks, vector as layer summary + geometry wireframe.

Keys: `m/z/t/g/p` to switch tabs, `q` to quit, `?` for help, arrow keys + enter
within tabs.

## Output specifications

### `info` (human-readable)

```
File:    dhaka.mbtiles (2.1 GB)
Name:    Dhaka Vector Tiles
Format:  pbf
Bounds:  90.30, 23.70, 90.50, 23.90
Center:  90.40, 23.80, 12
Zooms:   8 — 18
Tile y:  TMS (auto-detected)

Vector layers (5):
  road       (line)     minzoom 8,  maxzoom 18
  building   (polygon)  minzoom 14, maxzoom 18
  poi        (point)    minzoom 12, maxzoom 18
  water      (polygon)  minzoom 0,  maxzoom 18
  landuse    (polygon)  minzoom 8,  maxzoom 18

Zoom coverage:
  z    tiles      size       avg      max     coverage
  8    64         1.2 MB     19 KB    34 KB   100%
  ...
```

### `info --json`

```json
{
  "file": "dhaka.mbtiles",
  "size_bytes": 2147483648,
  "metadata": { "name": "...", "format": "pbf", "bounds": [...], ... },
  "tile_y_convention": "tms",
  "zoom_levels": [
    {
      "zoom": 8,
      "tile_count": 64,
      "expected_count": 64,
      "coverage_pct": 100.0,
      "size_bytes": 1258291,
      "size_avg": 19660,
      "size_min": 2048,
      "size_max": 34816
    }
  ]
}
```

### `gaps <file> --zoom 14`
```
z=14 missing tiles (12 of 80 expected, 85.0% coverage):
  12306,7075
  12306,7076
  12307,7075
  ...
```

`gaps --json` outputs `{"zoom": 14, "missing": [[x,y], ...], "expected": N, "actual": M}`.

`gaps --geojson` outputs a FeatureCollection of polygon features, one per missing tile, with `(z,x,y)` properties. Useful for visualizing on a map.

### `verify` (CI mode)

Exits 0 only if all of:

- File opens as valid SQLite
- Required metadata keys present (`name`, `format`, `bounds`, `minzoom`, `maxzoom`)
- No corrupt tiles (sample N tiles per zoom, attempt to decode)
- Coverage above threshold per zoom (default 95%, configurable with `--min-coverage`)
- No zoom levels declared in `minzoom..maxzoom` range that have zero tiles

Exit codes:
- 0: all good
- 1: validation failure (gaps, corruption)
- 2: file unreadable / not an mbtiles
- 3: invocation error (missing args, bad flags)

Output is single-line summary unless `-v`. JSON with `--json`.

### `diff a.mbtiles b.mbtiles`

Compares two files:

- Metadata changes (added/removed/modified keys)
- Per-zoom tile count delta
- Per-zoom size delta
- Sample of tiles added/removed (first 10 of each)

Useful for "did my regenerated tile set actually change anything."

### `extract <file> z/x/y`

Writes raw `tile_data` blob to stdout. No decoding. Pipe to `file` to identify
format, or to `protoc` for vector tiles, or to a file for raster.

Accepts both XYZ and TMS y. Default XYZ. `--tms` flag for explicit TMS.

## Gap detection algorithm

For each zoom level `z`:

1. Read declared `bounds` from metadata. Fall back to computing bounds from
   actual tile extent if metadata is absent.
2. Convert `bounds` to tile range using slippy-map math:
   ```
   n = 2^z
   x_min = floor((lon_min + 180) / 360 * n)
   x_max = floor((lon_max + 180) / 360 * n)
   y_min_xyz = floor((1 - ln(tan(lat_max) + sec(lat_max)) / π) / 2 * n)
   y_max_xyz = floor((1 - ln(tan(lat_min) + sec(lat_min)) / π) / 2 * n)
   ```
3. Detect TMS vs XYZ y-convention (see below).
4. Build expected set: all `(x, y)` in the rectangle.
5. Query `SELECT tile_column, tile_row FROM tiles WHERE zoom_level = ?`.
6. Diff. Report missing tiles in **XYZ convention** (user-friendly), regardless
   of how the file stores them.

### TMS detection

At the lowest populated zoom, check if observed `tile_row` values fall in the
range expected for TMS or XYZ given the bounds. They differ by `y_tms = (n - 1) - y_xyz`.

If ambiguous (single tile, or symmetric bounds), check the `metadata` table for
a `scheme` key (`tms` or `xyz`). Fall back to TMS (per spec). Display the
detected convention prominently so the user can sanity-check.

## Preview rendering

### Raster (png/jpg/webp)

Decode with stdlib `image/png`, `image/jpeg`, and `golang.org/x/image/webp`.
Render to terminal using halfblock characters (▀) — each character cell shows
two pixels (top + bottom) using fg + bg ANSI 24-bit color.

Target size: fit in current terminal. Resize the decoded image preserving
aspect ratio. Use nearest-neighbor for speed; the user is just sanity-checking.

### Vector (pbf/mvt)

Decode with `github.com/paulmach/orb/encoding/mvt`. Output:

```
Tile (14, 12306, 7075)  ·  pbf  ·  18.4 KB  ·  gzip-compressed

Layers:
  road           247 features  (LineString)
  building     1,820 features  (Polygon)
  poi             89 features  (Point)
  water           12 features  (Polygon)
  landuse         34 features  (Polygon)

Press [g] to render geometry preview, [j] to dump first feature as JSON
```

Geometry preview is a stretch goal: project all geometries to tile-local
coordinates (0..4096), downsample to terminal grid, render lines/polygons as
ASCII characters. Skip for v1 if time is tight.

Tiles are commonly gzip-compressed inside the blob. Detect via magic bytes
(`1f 8b`) and decompress before parsing.

## TUI implementation

bubbletea + lipgloss + bubbles (table, viewport, help).

Top-level `Model` holds:

- File handle and parsed metadata
- Active tab
- Per-tab sub-models (each tab is its own bubbletea component)
- Window size

Tab switching is just changing the active sub-model. Each sub-model handles
its own keys when active. Global keys (`q`, `?`, tab-switch keys) handled at
top level.

Lazy loading: don't read all tile rows on startup. The Tiles tab queries on
demand with `LIMIT/OFFSET`. The Zooms tab uses aggregate queries
(`SELECT zoom_level, COUNT(*), SUM(LENGTH(tile_data)), ...`).

## Architecture

```
cmd/mbtiles-inspect/
  main.go                      # cobra root, version, dispatches subcommands

internal/mbtiles/
  reader.go                    # opens SQLite, queries metadata + tiles
  metadata.go                  # parses metadata key-value pairs into struct
  tile.go                      # tile blob handling, gzip detection

internal/tiles/
  slippy.go                    # lat/lon ↔ tile, XYZ ↔ TMS conversion
  bbox.go                      # bbox → tile range at zoom

internal/gaps/
  detect.go                    # expected vs actual diff
  geojson.go                   # GeoJSON output for missing tiles

internal/preview/
  raster.go                    # image decode → halfblock terminal output
  vector.go                    # mvt decode → layer summary
  ascii.go                     # halfblock renderer

internal/report/
  human.go                     # human-readable text output
  json.go                      # JSON output
  diff.go                      # diff formatting

internal/tui/
  model.go                     # top-level bubbletea model
  metadata_tab.go
  zooms_tab.go
  tiles_tab.go
  gaps_tab.go
  preview_tab.go
  styles.go                    # lipgloss styles, shared

cmd/mbtiles-inspect/
  info.go
  gaps.go
  verify.go
  diff.go
  extract.go
  tui.go
```

## Dependencies

- `github.com/spf13/cobra` — CLI
- `github.com/mattn/go-sqlite3` — SQLite (CGO) — OR `modernc.org/sqlite` (pure Go, no CGO). Prefer pure Go for easier cross-compilation.
- `github.com/paulmach/orb` + `github.com/paulmach/orb/encoding/mvt` — vector tiles
- `golang.org/x/image/webp` — webp decode
- `github.com/charmbracelet/bubbletea` — TUI
- `github.com/charmbracelet/bubbles` — TUI components
- `github.com/charmbracelet/lipgloss` — styling

Pure-Go SQLite means a static binary cross-compiles to Linux easily for CI use.

## Testing

- `internal/tiles` — unit tests for slippy math against known reference points
  (e.g., `(0,0)` at z=0 = whole world; Dhaka at z=14 = known tile).
- `internal/gaps` — unit tests with synthetic small mbtiles fixtures.
- Fixtures: a tiny generated mbtiles (1KB) committed to `testdata/`. Generation
  script in `testdata/gen.go` so it's reproducible.
- `verify` command tested against:
  - valid file → exit 0
  - file with gaps → exit 1
  - non-mbtiles file → exit 2
- Snapshot tests for human-readable output (golden files in `testdata/golden/`).

No tests for the TUI rendering itself (low ROI). Test the underlying tab
sub-models' update functions with synthetic key events.

## Out of scope for v1

- Geometry rendering for vector tile preview (stretch).
- `pmtiles` support.
- Network mbtiles (URL inputs).
- Modifying files.
- Authoring new mbtiles from sources.

## Milestones

**M1 — Core read & info** (Saturday morning)
- mbtiles reader, metadata parsing, zoom summary aggregate query
- `info` command (human + JSON)
- TMS/XYZ detection
- Slippy-map math + tests

**M2 — Gaps & verify** (Saturday afternoon)
- Gap detection
- `gaps` command (human, JSON, GeoJSON)
- `verify` command with proper exit codes
- `extract` and `diff`

**M3 — TUI shell** (Sunday morning)
- bubbletea scaffolding, tab switching
- Metadata tab, Zooms tab (read-only views of M1 data)

**M4 — TUI interactive tabs** (Sunday afternoon)
- Tiles tab with sort/filter
- Gaps tab with ASCII grid
- Preview tab with raster rendering
- Vector layer summary in Preview

**Stretch** — vector geometry rendering, `diff` command polish, README with screencasts.
