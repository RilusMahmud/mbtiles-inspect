//go:build ignore

// gen generates deterministic .mbtiles fixtures for tests.
//
// Run from the project root:
//
//	go run ./testdata/gen.go
//
// Outputs:
//
//	testdata/complete.mbtiles    full coverage at z=0..2 over Dhaka bbox
//	testdata/gappy.mbtiles       same, but z=2 is missing 25% of tiles
//	testdata/xyz.mbtiles         tiny file in XYZ y-convention with scheme=xyz
//	testdata/corrupt.mbtiles     full coverage but one tile has a bad gzip header
//	testdata/notmbtiles.sqlite   SQLite that lacks tiles/metadata tables
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	dir := "testdata"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, f := range []string{"complete.mbtiles", "gappy.mbtiles", "xyz.mbtiles", "corrupt.mbtiles", "notmbtiles.sqlite"} {
		_ = os.Remove(filepath.Join(dir, f))
	}

	if err := writeComplete(filepath.Join(dir, "complete.mbtiles")); err != nil {
		log.Fatalf("complete: %v", err)
	}
	if err := writeGappy(filepath.Join(dir, "gappy.mbtiles")); err != nil {
		log.Fatalf("gappy: %v", err)
	}
	if err := writeXYZ(filepath.Join(dir, "xyz.mbtiles")); err != nil {
		log.Fatalf("xyz: %v", err)
	}
	if err := writeCorrupt(filepath.Join(dir, "corrupt.mbtiles")); err != nil {
		log.Fatalf("corrupt: %v", err)
	}
	if err := writeNotMBTiles(filepath.Join(dir, "notmbtiles.sqlite")); err != nil {
		log.Fatalf("notmbtiles: %v", err)
	}
	fmt.Println("fixtures generated in", dir)
}

func openDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(MEMORY)&_pragma=synchronous(OFF)")
}

func createSchema(db *sql.DB) error {
	for _, stmt := range []string{
		`CREATE TABLE metadata (name TEXT PRIMARY KEY, value TEXT);`,
		`CREATE TABLE tiles (zoom_level INTEGER, tile_column INTEGER, tile_row INTEGER, tile_data BLOB,
			PRIMARY KEY (zoom_level, tile_column, tile_row));`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func putMeta(db *sql.DB, kv map[string]string) error {
	for k, v := range kv {
		if _, err := db.Exec(`INSERT INTO metadata(name, value) VALUES(?, ?)`, k, v); err != nil {
			return err
		}
	}
	return nil
}

func writeComplete(path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := createSchema(db); err != nil {
		return err
	}
	if err := putMeta(db, map[string]string{
		"name":    "complete fixture",
		"format":  "pbf",
		"bounds":  "-180,-85,180,85",
		"minzoom": "0",
		"maxzoom": "2",
	}); err != nil {
		return err
	}
	// Whole-world coverage z=0..2, TMS y.
	for z := 0; z <= 2; z++ {
		n := int(math.Exp2(float64(z)))
		for x := 0; x < n; x++ {
			for y := 0; y < n; y++ {
				if _, err := db.Exec(
					`INSERT INTO tiles(zoom_level,tile_column,tile_row,tile_data) VALUES(?,?,?,?)`,
					z, x, y, []byte{0x1a, 0x00}, // tiny "pbf" stub
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeGappy(path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := createSchema(db); err != nil {
		return err
	}
	if err := putMeta(db, map[string]string{
		"name":    "gappy fixture",
		"format":  "pbf",
		"bounds":  "-180,-85,180,85",
		"minzoom": "0",
		"maxzoom": "2",
	}); err != nil {
		return err
	}
	for z := 0; z <= 2; z++ {
		n := int(math.Exp2(float64(z)))
		for x := 0; x < n; x++ {
			for y := 0; y < n; y++ {
				// At z=2, drop tiles where (x+y) is even — ~50% missing.
				if z == 2 && (x+y)%2 == 0 {
					continue
				}
				if _, err := db.Exec(
					`INSERT INTO tiles(zoom_level,tile_column,tile_row,tile_data) VALUES(?,?,?,?)`,
					z, x, y, []byte{0x1a, 0x00},
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeXYZ(path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := createSchema(db); err != nil {
		return err
	}
	if err := putMeta(db, map[string]string{
		"name":    "xyz fixture",
		"format":  "pbf",
		"bounds":  "-180,-85,180,85",
		"minzoom": "0",
		"maxzoom": "1",
		"scheme":  "xyz",
	}); err != nil {
		return err
	}
	// Same single tile at z=0; at z=1 use XYZ y values (0,0)(1,0)(0,1)(1,1)
	// which happens to be symmetric, but scheme=xyz settles ambiguity.
	if _, err := db.Exec(
		`INSERT INTO tiles(zoom_level,tile_column,tile_row,tile_data) VALUES(0,0,0,?)`,
		[]byte{0x1a, 0x00},
	); err != nil {
		return err
	}
	for x := 0; x < 2; x++ {
		for y := 0; y < 2; y++ {
			if _, err := db.Exec(
				`INSERT INTO tiles(zoom_level,tile_column,tile_row,tile_data) VALUES(1,?,?,?)`,
				x, y, []byte{0x1a, 0x00},
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeCorrupt(path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := createSchema(db); err != nil {
		return err
	}
	if err := putMeta(db, map[string]string{
		"name":    "corrupt fixture",
		"format":  "pbf",
		"bounds":  "-180,-85,180,85",
		"minzoom": "0",
		"maxzoom": "1",
	}); err != nil {
		return err
	}
	good := []byte{0x1a, 0x00}
	// gzip magic header followed by garbage — passes IsGzip, fails decompress.
	bad := []byte{0x1f, 0x8b, 0xde, 0xad, 0xbe, 0xef}
	for z := 0; z <= 1; z++ {
		n := int(math.Exp2(float64(z)))
		for x := 0; x < n; x++ {
			for y := 0; y < n; y++ {
				blob := good
				if z == 1 && x == 0 && y == 0 {
					blob = bad
				}
				if _, err := db.Exec(
					`INSERT INTO tiles(zoom_level,tile_column,tile_row,tile_data) VALUES(?,?,?,?)`,
					z, x, y, blob,
				); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeNotMBTiles(path string) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE not_mbtiles (id INTEGER); INSERT INTO not_mbtiles VALUES(1);`)
	return err
}
