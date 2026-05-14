package geom

import (
	"testing"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/flatbuf"
	"github.com/ringsaturn/xiangshan/internal/types"
)

func TestPolygonContainsPoint(t *testing.T) {
	poly := NewPolygon(
		[]Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}},
		[][]Point{{{X: 3, Y: 3}, {X: 7, Y: 3}, {X: 7, Y: 7}, {X: 3, Y: 7}}},
	)
	cases := []struct {
		name string
		p    Point
		want bool
	}{
		{"inside", Point{X: 1, Y: 1}, true},
		{"insideHole", Point{X: 5, Y: 5}, false},
		{"outside", Point{X: 15, Y: 5}, false},
		{"boundary", Point{X: 0, Y: 5}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := poly.ContainsPoint(c.p); got != c.want {
				t.Fatalf("ContainsPoint(%v) = %v, want %v", c.p, got, c.want)
			}
		})
	}
}

func TestDivisionContainsPointFB(t *testing.T) {
	div := testFlatbufDivision(t)
	cases := []struct {
		name     string
		lng, lat float64
		want     bool
	}{
		{"inside", 1, 1, true},
		{"hole", 5, 5, false},
		{"outside", 15, 5, false},
		{"boundary", 0, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DivisionContainsPointFB(div, c.lng, c.lat); got != c.want {
				t.Fatalf("DivisionContainsPointFB(%v, %v) = %v, want %v", c.lng, c.lat, got, c.want)
			}
		})
	}
}

func TestDivisionIndexContainsPoint(t *testing.T) {
	div := testFlatbufDivision(t)
	idx := NewDivisionIndex(div)
	if !idx.ContainsPoint(div, 1, 1) {
		t.Fatal("indexed division should contain inner point")
	}
	if idx.ContainsPoint(div, 5, 5) {
		t.Fatal("indexed division should exclude hole point")
	}
	allocs := testing.AllocsPerRun(1000, func() {
		_ = idx.ContainsPoint(div, 1, 1)
	})
	if allocs != 0 {
		t.Fatalf("indexed ContainsPoint allocations = %v, want 0", allocs)
	}
}

func TestCompressedDivisionContainsPointFB(t *testing.T) {
	div := testCompressedFlatbufDivision(t)
	cases := []struct {
		name     string
		lng, lat float64
		want     bool
	}{
		{"inside", 1, 1, true},
		{"hole", 5, 5, false},
		{"outside", 15, 5, false},
		{"boundary", 0, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CompressedDivisionContainsPointFB(div, c.lng, c.lat); got != c.want {
				t.Fatalf("CompressedDivisionContainsPointFB(%v, %v) = %v, want %v", c.lng, c.lat, got, c.want)
			}
		})
	}
}

func TestCompressedDivisionContainsPointFBAllocs(t *testing.T) {
	div := testCompressedFlatbufDivision(t)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = CompressedDivisionContainsPointFB(div, 1, 1)
	})
	if allocs != 0 {
		t.Fatalf("compressed ContainsPoint allocations = %v, want 0", allocs)
	}
}

func testFlatbufDivision(t *testing.T) *xs.Division {
	t.Helper()
	divs := []types.Division{{
		ID:      "area-1",
		Name:    "Test",
		Subtype: types.SubtypeCountry,
		BBox:    types.BBox{Xmin: 0, Xmax: 10, Ymin: 0, Ymax: 10},
		Polygons: []types.Polygon{{
			Exterior: types.Ring{Coords: []float32{0, 0, 10, 0, 10, 10, 0, 10}},
			Holes: []types.Ring{{
				Coords: []float32{3, 3, 7, 3, 7, 7, 3, 7},
			}},
		}},
	}}
	buf, err := flatbuf.EncodeDivisions(divs, nil, nil, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	var div xs.Division
	if !root.Items(&div, 0) {
		t.Fatal("missing test division")
	}
	return &div
}

func testCompressedFlatbufDivision(t *testing.T) *xs.CompressedDivision {
	t.Helper()
	divs := []types.Division{{
		ID:      "area-1",
		Name:    "Test",
		Subtype: types.SubtypeCountry,
		BBox:    types.BBox{Xmin: 0, Xmax: 10, Ymin: 0, Ymax: 10},
		Polygons: []types.Polygon{{
			Exterior: types.Ring{Coords: []float32{0, 0, 10, 0, 10, 10, 0, 10}},
			Holes: []types.Ring{{
				Coords: []float32{3, 3, 7, 3, 7, 7, 3, 7},
			}},
		}},
	}}
	buf, err := flatbuf.EncodeDivisions(divs, nil, nil, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	cbuf, _, err := flatbuf.EncodeCompressedDivisionsFromXSFB(root)
	if err != nil {
		t.Fatal(err)
	}
	croot := xs.GetSizePrefixedRootAsCompressedDivisions(cbuf, 0)
	var div xs.CompressedDivision
	if !croot.Items(&div, 0) {
		t.Fatal("missing compressed test division")
	}
	return &div
}
