package mbtiles

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

// ErrCorruptTile signals that a tile blob failed integrity checks (bad gzip,
// raster magic with truncated body, or unrecognised content for the declared
// format).
var ErrCorruptTile = errors.New("tile blob is corrupt")

// IsGzip reports whether b looks like a gzip-compressed payload.
//
// Vector tiles in mbtiles are commonly gzipped inside tile_data; the spec
// recommends it but does not require it. Detection is by magic bytes.
func IsGzip(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

// MaybeUngzip returns b decompressed if it's gzipped, or b unchanged otherwise.
func MaybeUngzip(b []byte) ([]byte, error) {
	if !IsGzip(b) {
		return b, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("gzip header: %w", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("gzip body: %w", err)
	}
	return out, nil
}

// DetectFormat sniffs raster magic bytes and returns "png", "jpg", "webp",
// "pbf", or "unknown".
//
// PBF detection is heuristic: after gzip-stripping, if the byte does not
// match a known raster magic and looks like a protobuf wire-tag, we report
// pbf. Callers that have authoritative format from metadata should prefer it.
func DetectFormat(b []byte) string {
	raw, err := MaybeUngzip(b)
	if err != nil {
		raw = b
	}
	switch {
	case len(raw) >= 8 && bytes.Equal(raw[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "png"
	case len(raw) >= 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff:
		return "jpg"
	case len(raw) >= 12 && bytes.Equal(raw[:4], []byte("RIFF")) && bytes.Equal(raw[8:12], []byte("WEBP")):
		return "webp"
	case len(raw) > 0 && (raw[0]&0x07) <= 5:
		// Protobuf wire types are 0..5 in the low 3 bits of the first byte.
		// This is loose but works for MVT in practice.
		return "pbf"
	default:
		return "unknown"
	}
}

// ValidateTile applies a cheap integrity check to a tile blob. It is meant for
// `verify` sampling: false positives would block CI, so the bar is "obviously
// broken", not "decodes to a valid image". Returns ErrCorruptTile (wrapped)
// when the blob fails a check.
//
// Checks performed:
//   - Empty blob → corrupt.
//   - Gzip magic present but decompression fails → corrupt.
//   - Declared format pbf → after gzip strip, first byte's protobuf wire-type
//     bits must be 0..5.
//   - Declared format png/jpg/webp → after gzip strip, the corresponding magic
//     bytes must be present.
func ValidateTile(format string, blob []byte) error {
	if len(blob) == 0 {
		return fmt.Errorf("%w: empty blob", ErrCorruptTile)
	}
	raw, err := MaybeUngzip(blob)
	if err != nil {
		return fmt.Errorf("%w: gzip: %v", ErrCorruptTile, err)
	}
	switch format {
	case "pbf", "mvt":
		if len(raw) == 0 || (raw[0]&0x07) > 5 {
			return fmt.Errorf("%w: not a protobuf payload", ErrCorruptTile)
		}
	case "png":
		if len(raw) < 8 || !bytes.Equal(raw[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
			return fmt.Errorf("%w: bad PNG magic", ErrCorruptTile)
		}
	case "jpg", "jpeg":
		if len(raw) < 3 || raw[0] != 0xff || raw[1] != 0xd8 || raw[2] != 0xff {
			return fmt.Errorf("%w: bad JPEG magic", ErrCorruptTile)
		}
	case "webp":
		if len(raw) < 12 || !bytes.Equal(raw[:4], []byte("RIFF")) || !bytes.Equal(raw[8:12], []byte("WEBP")) {
			return fmt.Errorf("%w: bad WEBP magic", ErrCorruptTile)
		}
	}
	return nil
}
