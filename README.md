# mbtiles-inspect

Terminal tool for inspecting `.mbtiles` files: metadata, zoom coverage, gaps,
and tile previews. CLI subcommands for scripting and CI; TUI for interactive
exploration.

See [SPEC.md](./SPEC.md) for the full design and [CLAUDE.md](./CLAUDE.md) for
contribution rules.

## Status

- M1 (core read + `info`) — done
- M2 (`gaps`, `verify`, `extract`, `diff`) — done
- M3 (TUI shell, Metadata/Zooms tabs) — done
- M4 (TUI Tiles/Gaps/Preview tabs, vector layer summary) — done

## Quick start

```sh
make fixtures        # generate test mbtiles
make build
./bin/mbtiles-inspect info testdata/complete.mbtiles
./bin/mbtiles-inspect gaps testdata/gappy.mbtiles --zoom 2
./bin/mbtiles-inspect verify testdata/complete.mbtiles
./bin/mbtiles-inspect testdata/complete.mbtiles    # launches TUI
```

## Commands

```
mbtiles-inspect info <file>              metadata + per-zoom summary
mbtiles-inspect gaps <file> [--zoom N]   list missing tiles in declared bounds
mbtiles-inspect verify <file>            CI mode (non-zero on issues)
mbtiles-inspect diff <a> <b>             compare two mbtiles files
mbtiles-inspect extract <file> z/x/y     write tile blob to stdout
mbtiles-inspect tui <file>               launch interactive TUI
mbtiles-inspect <file>                   shorthand for `tui <file>`
```

`--json` switches `info`, `gaps`, and `diff` to machine-readable output.
`--geojson` (on `gaps`) emits a FeatureCollection of missing-tile polygons.
`verify --sample N` spot-checks N tiles per zoom for corruption (default 8).

### TUI keys

`m`/`z`/`t`/`g`/`p` jump to Metadata/Zooms/Tiles/Gaps/Preview tabs;
`tab` / `shift+tab` cycle. In Tiles and Gaps, `[` / `]` switch zoom.
In Gaps, `e` exports the current zoom's missing-tile GeoJSON to a temp file.
In Preview, `i` opens an entry prompt for a `z/x/y` (XYZ).
`q` quits, `?` toggles help.

## Exit codes

- 0 — success
- 1 — validation failure (gaps, corruption)
- 2 — file unreadable / not an mbtiles
- 3 — invocation error (bad args/flags)

## License

MIT (add LICENSE file before publishing).
