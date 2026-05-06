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

## Install

Prebuilt binaries for Linux and macOS (amd64 + arm64) are published on the
[Releases page](https://github.com/RilusMahmud/mbtiles-inspect/releases).

```sh
# Linux (adjust version + arch as needed)
curl -L -o mbtiles-inspect.tar.gz \
  https://github.com/RilusMahmud/mbtiles-inspect/releases/latest/download/mbtiles-inspect_<version>_linux_amd64.tar.gz
tar -xzf mbtiles-inspect.tar.gz
./mbtiles-inspect --version
```

macOS binaries are not signed; after extracting, run
`xattr -d com.apple.quarantine ./mbtiles-inspect` once to clear the
quarantine attribute.

To build from source instead:

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

## Releasing (maintainers)

Releases are produced by GoReleaser via GitHub Actions. To cut a release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `release` workflow runs tests, then builds Linux + macOS binaries
(amd64 + arm64) and publishes them to a new GitHub Release with a SHA-256
checksum file. A manual `workflow_dispatch` run on the Actions tab produces
a snapshot dry-run (binaries uploaded as a workflow artifact, no GitHub
Release created) and is safe for verifying the pipeline.

## License

MIT (add LICENSE file before publishing).
