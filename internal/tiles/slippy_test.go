package tiles

import "testing"

func TestLonLatToTile(t *testing.T) {
	cases := []struct {
		name         string
		lon, lat     float64
		z            int
		wantX, wantY int
	}{
		{"origin z0", 0, 0, 0, 0, 0},
		{"null island z2", 0, 0, 2, 2, 2},
		// Dhaka ~ (90.40, 23.81) — standard OSM slippy formula at z=14.
		{"dhaka z14", 90.40, 23.81, 14, 12306, 7075},
		{"sw extreme z3", -179.9999, -85.0, 3, 0, 7},
		{"ne extreme z3", 179.9999, 85.0, 3, 7, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y := LonLatToTile(tc.lon, tc.lat, tc.z)
			if x != tc.wantX || y != tc.wantY {
				t.Fatalf("LonLatToTile(%v,%v,%d) = (%d,%d), want (%d,%d)",
					tc.lon, tc.lat, tc.z, x, y, tc.wantX, tc.wantY)
			}
		})
	}
}

func TestBBoxToRange(t *testing.T) {
	// Dhaka-ish bbox from the spec example.
	bbox := BBox{LonMin: 90.30, LatMin: 23.70, LonMax: 90.50, LatMax: 23.90}
	r := BBoxToRange(bbox, 14)
	if r.Z != 14 {
		t.Fatalf("Z = %d, want 14", r.Z)
	}
	if r.XMin > r.XMax || r.YMin > r.YMax {
		t.Fatalf("invalid range: %+v", r)
	}
	// The bbox should contain the (12306, 7075) tile we tested above.
	xIn := 12306 >= r.XMin && 12306 <= r.XMax
	yIn := 7075 >= r.YMin && 7075 <= r.YMax
	if !xIn || !yIn {
		t.Fatalf("expected (12306,7075) inside %+v", r)
	}
}

func TestFlipY(t *testing.T) {
	cases := []struct {
		z, in, want int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{1, 1, 0},
		{14, 7075, 9308}, // 2^14 - 1 - 7075
	}
	for _, tc := range cases {
		got := FlipY(tc.z, tc.in)
		if got != tc.want {
			t.Errorf("FlipY(%d,%d) = %d, want %d", tc.z, tc.in, got, tc.want)
		}
	}
}

func TestRangeCount(t *testing.T) {
	r := Range{Z: 0, XMin: 0, YMin: 0, XMax: 0, YMax: 0}
	if r.Count() != 1 {
		t.Errorf("z0 single tile count = %d, want 1", r.Count())
	}
	r = Range{Z: 2, XMin: 0, YMin: 0, XMax: 3, YMax: 3}
	if r.Count() != 16 {
		t.Errorf("z2 full count = %d, want 16", r.Count())
	}
	r = Range{Z: 1, XMin: 1, YMin: 0, XMax: 0, YMax: 0}
	if r.Count() != 0 {
		t.Errorf("inverted range count = %d, want 0", r.Count())
	}
}
