package mbtiles

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rilus/mbtiles-inspect/internal/tiles"
)

// Metadata is the parsed key-value content of an mbtiles `metadata` table.
//
// Unrecognized keys are preserved in Extra so callers can surface them
// (e.g. in the TUI's Metadata tab).
type Metadata struct {
	Name        string
	Format      string // pbf, png, jpg, webp
	Bounds      *tiles.BBox
	Center      *Center
	MinZoom     *int
	MaxZoom     *int
	Attribution string
	Description string
	Type        string // overlay, baselayer
	Version     string
	Scheme      string // tms, xyz (rarely set)
	JSON        *VectorLayersJSON

	Extra map[string]string
}

// Center is the metadata `center` key: lon, lat, zoom.
type Center struct {
	Lon, Lat float64
	Zoom     int
}

// VectorLayersJSON mirrors the `json` metadata key for vector tilesets.
type VectorLayersJSON struct {
	VectorLayers []VectorLayer `json:"vector_layers"`
}

// VectorLayer is one entry in the vector_layers metadata array.
type VectorLayer struct {
	ID          string            `json:"id"`
	Description string            `json:"description,omitempty"`
	MinZoom     *int              `json:"minzoom,omitempty"`
	MaxZoom     *int              `json:"maxzoom,omitempty"`
	Fields      map[string]string `json:"fields,omitempty"`
}

// ParseMetadata converts a raw key-value map (as stored in the metadata table)
// into a typed Metadata struct. Unknown fields go into Extra.
//
// Errors are returned for malformed values of well-known keys (e.g. bounds
// with the wrong number of components). Missing keys are not errors at this
// layer — `verify` enforces required-key policy separately.
func ParseMetadata(kv map[string]string) (*Metadata, error) {
	m := &Metadata{Extra: map[string]string{}}
	for k, v := range kv {
		switch k {
		case "name":
			m.Name = v
		case "format":
			m.Format = v
		case "bounds":
			b, err := parseBounds(v)
			if err != nil {
				return nil, fmt.Errorf("metadata.bounds: %w", err)
			}
			m.Bounds = b
		case "center":
			c, err := parseCenter(v)
			if err != nil {
				return nil, fmt.Errorf("metadata.center: %w", err)
			}
			m.Center = c
		case "minzoom":
			z, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("metadata.minzoom: %w", err)
			}
			m.MinZoom = &z
		case "maxzoom":
			z, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return nil, fmt.Errorf("metadata.maxzoom: %w", err)
			}
			m.MaxZoom = &z
		case "attribution":
			m.Attribution = v
		case "description":
			m.Description = v
		case "type":
			m.Type = v
		case "version":
			m.Version = v
		case "scheme":
			m.Scheme = strings.ToLower(strings.TrimSpace(v))
		case "json":
			var j VectorLayersJSON
			if err := json.Unmarshal([]byte(v), &j); err != nil {
				return nil, fmt.Errorf("metadata.json: %w", err)
			}
			m.JSON = &j
		default:
			m.Extra[k] = v
		}
	}
	return m, nil
}

func parseBounds(s string) (*tiles.BBox, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("expected 4 comma-separated values, got %d", len(parts))
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, fmt.Errorf("component %d: %w", i, err)
		}
		vals[i] = f
	}
	return &tiles.BBox{
		LonMin: vals[0], LatMin: vals[1],
		LonMax: vals[2], LatMax: vals[3],
	}, nil
}

func parseCenter(s string) (*Center, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 comma-separated values, got %d", len(parts))
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return nil, fmt.Errorf("lon: %w", err)
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return nil, fmt.Errorf("lat: %w", err)
	}
	z, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return nil, fmt.Errorf("zoom: %w", err)
	}
	return &Center{Lon: lon, Lat: lat, Zoom: z}, nil
}
