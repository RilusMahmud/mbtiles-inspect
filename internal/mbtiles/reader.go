// Package mbtiles reads .mbtiles files (SQLite) and exposes typed access to
// metadata, per-zoom aggregates, and individual tile blobs.
//
// The package is read-only by design. All connections open with mode=ro.
package mbtiles

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Reader is a read-only handle to an mbtiles file.
type Reader struct {
	db   *sql.DB
	path string
}

// Open opens an mbtiles file in read-only mode. The caller must Close it.
func Open(path string) (*Reader, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=query_only(1)", url.PathEscape(abs))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Verify it actually opens and looks like an mbtiles file.
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	r := &Reader{db: db, path: abs}
	if err := r.checkSchema(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return r, nil
}

// Close releases the underlying database handle.
func (r *Reader) Close() error { return r.db.Close() }

// Path returns the absolute path to the open file.
func (r *Reader) Path() string { return r.path }

// ErrNotMBTiles is returned when the opened SQLite file lacks the required
// `metadata` and `tiles` schema.
var ErrNotMBTiles = errors.New("file is not a valid mbtiles (missing metadata/tiles)")

func (r *Reader) checkSchema(ctx context.Context) error {
	required := []string{"metadata", "tiles"}
	for _, t := range required {
		var name string
		err := r.db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?`, t,
		).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: missing %q", ErrNotMBTiles, t)
		}
		if err != nil {
			return fmt.Errorf("schema probe %q: %w", t, err)
		}
	}
	return nil
}

// MetadataMap returns the raw key-value pairs from the metadata table.
// Use ParseMetadata to convert to a typed struct.
func (r *Reader) MetadataMap(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name, value FROM metadata`)
	if err != nil {
		return nil, fmt.Errorf("query metadata: %w", err)
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan metadata row: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metadata: %w", err)
	}
	return out, nil
}

// Metadata returns the typed metadata for the file.
func (r *Reader) Metadata(ctx context.Context) (*Metadata, error) {
	kv, err := r.MetadataMap(ctx)
	if err != nil {
		return nil, err
	}
	return ParseMetadata(kv)
}

// ZoomStats summarizes tile data for one zoom level.
type ZoomStats struct {
	Zoom      int
	TileCount int64
	TotalSize int64 // sum of LENGTH(tile_data)
	MinSize   int64
	MaxSize   int64
	AvgSize   int64 // computed: TotalSize / TileCount
	XMin      int
	XMax      int
	YMin      int // as stored (may be TMS)
	YMax      int // as stored
}

// ZoomStats returns aggregate stats per zoom level, ordered ascending.
//
// One query, no per-tile iteration, so this is fast even on large files.
func (r *Reader) ZoomStats(ctx context.Context) ([]ZoomStats, error) {
	const q = `
		SELECT
			zoom_level,
			COUNT(*),
			COALESCE(SUM(LENGTH(tile_data)), 0),
			COALESCE(MIN(LENGTH(tile_data)), 0),
			COALESCE(MAX(LENGTH(tile_data)), 0),
			MIN(tile_column), MAX(tile_column),
			MIN(tile_row), MAX(tile_row)
		FROM tiles
		GROUP BY zoom_level
		ORDER BY zoom_level ASC
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("zoom stats query: %w", err)
	}
	defer rows.Close()

	var out []ZoomStats
	for rows.Next() {
		var s ZoomStats
		if err := rows.Scan(
			&s.Zoom, &s.TileCount, &s.TotalSize, &s.MinSize, &s.MaxSize,
			&s.XMin, &s.XMax, &s.YMin, &s.YMax,
		); err != nil {
			return nil, fmt.Errorf("scan zoom stats: %w", err)
		}
		if s.TileCount > 0 {
			s.AvgSize = s.TotalSize / s.TileCount
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate zoom stats: %w", err)
	}
	return out, nil
}

// TileRow is a single tile row's coordinates and size, used by gap detection
// and the tiles tab.
type TileRow struct {
	Z, X, Y int // Y as stored
	Size    int64
}

// TilesAtZoom returns coordinates (and sizes) of all tiles at a given zoom.
//
// On large files this can return millions of rows. Callers needing a bounded
// view should use ZoomStats or paginate via TilesPage.
func (r *Reader) TilesAtZoom(ctx context.Context, zoom int) ([]TileRow, error) {
	const q = `
		SELECT zoom_level, tile_column, tile_row, LENGTH(tile_data)
		FROM tiles WHERE zoom_level = ?
	`
	rows, err := r.db.QueryContext(ctx, q, zoom)
	if err != nil {
		return nil, fmt.Errorf("tiles at zoom: %w", err)
	}
	defer rows.Close()
	var out []TileRow
	for rows.Next() {
		var t TileRow
		if err := rows.Scan(&t.Z, &t.X, &t.Y, &t.Size); err != nil {
			return nil, fmt.Errorf("scan tile row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CoordCursor streams tile coordinates ordered by (zoom_level, tile_column,
// tile_row). It is the building block for diffing two files in lock-step
// without loading every coordinate into memory.
type CoordCursor struct {
	rows *sql.Rows
}

// Next advances the cursor and returns the next coordinate. The bool reports
// whether a value was scanned; when false, iteration is done (check err).
func (c *CoordCursor) Next() (z, x, y int, ok bool, err error) {
	if !c.rows.Next() {
		if e := c.rows.Err(); e != nil {
			return 0, 0, 0, false, fmt.Errorf("iterate coords: %w", e)
		}
		return 0, 0, 0, false, nil
	}
	if err := c.rows.Scan(&z, &x, &y); err != nil {
		return 0, 0, 0, false, fmt.Errorf("scan coord: %w", err)
	}
	return z, x, y, true, nil
}

// Close releases the underlying iterator.
func (c *CoordCursor) Close() error { return c.rows.Close() }

// Coords returns a streaming cursor over every (z, x, y) in the file, ordered
// by the primary key. The caller must Close it.
func (r *Reader) Coords(ctx context.Context) (*CoordCursor, error) {
	const q = `
		SELECT zoom_level, tile_column, tile_row FROM tiles
		ORDER BY zoom_level, tile_column, tile_row
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("coords query: %w", err)
	}
	return &CoordCursor{rows: rows}, nil
}

// SampleTilesAtZoom returns up to n tile blobs at the given zoom, ordered by
// (tile_column, tile_row) for deterministic sampling. Used by verify's
// corruption check.
func (r *Reader) SampleTilesAtZoom(ctx context.Context, zoom, n int) ([][]byte, error) {
	const q = `
		SELECT tile_data FROM tiles
		WHERE zoom_level = ?
		ORDER BY tile_column, tile_row
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, q, zoom, n)
	if err != nil {
		return nil, fmt.Errorf("sample tiles: %w", err)
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, fmt.Errorf("scan sample blob: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Tile returns the raw blob for one (z, x, y). The y argument follows the
// **storage** convention of the file (typically TMS).
//
// Returns (nil, nil) if no row exists.
func (r *Reader) Tile(ctx context.Context, z, x, y int) ([]byte, error) {
	const q = `
		SELECT tile_data FROM tiles
		WHERE zoom_level = ? AND tile_column = ? AND tile_row = ?
		LIMIT 1
	`
	var blob []byte
	err := r.db.QueryRowContext(ctx, q, z, x, y).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fetch tile: %w", err)
	}
	return blob, nil
}
