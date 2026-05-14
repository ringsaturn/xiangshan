package xiangshan

import (
	"os"
	"path/filepath"
	"testing"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/flatbuf"
	"github.com/ringsaturn/xiangshan/internal/grid"
	"github.com/ringsaturn/xiangshan/internal/types"
)

func TestFinderCountryPreindexKeepsCoarseRegion(t *testing.T) {
	divs := []types.Division{
		{
			ID:         "country",
			Name:       "Country",
			Subtype:    types.SubtypeCountry,
			AdminLevel: -1,
			BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
			}},
		},
		{
			ID:         "region",
			Name:       "Region",
			Subtype:    types.SubtypeRegion,
			AdminLevel: 4,
			BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
			}},
		},
	}
	f := newTestFinder(t, divs)
	defer f.Close()

	if got := len(f.countryPreindex); got != 1 {
		t.Fatalf("country preindex cells = %d, want 1", got)
	}
	got := f.Query(0.5, 0.5)
	if got.Country != "Country" {
		t.Fatalf("Country = %q, want Country", got.Country)
	}
	if got.Region != "Region" {
		t.Fatalf("Region = %q, want Region", got.Region)
	}
}

func TestCanShortCircuit(t *testing.T) {
	if !canShortCircuit(0, 0) {
		t.Fatal("expected ordinary coordinate to allow short circuit")
	}
	if canShortCircuit(179, 0) {
		t.Fatal("expected antimeridian edge to reject short circuit")
	}
	if canShortCircuit(0, -89) {
		t.Fatal("expected polar edge to reject short circuit")
	}
}

func TestCompressedFinderMatchesFinder(t *testing.T) {
	divs := []types.Division{
		{
			ID:         "country",
			Name:       "Country",
			Subtype:    types.SubtypeCountry,
			AdminLevel: -1,
			BBox:       types.BBox{Xmin: 0, Xmax: 10, Ymin: 0, Ymax: 10},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{0, 0, 10, 0, 10, 10, 0, 10}},
				Holes: []types.Ring{{
					Coords: []float32{3, 3, 7, 3, 7, 7, 3, 7},
				}},
			}},
		},
		{
			ID:         "local",
			Name:       "Local",
			Subtype:    types.SubtypeLocalAdmin,
			AdminLevel: 8,
			BBox:       types.BBox{Xmin: 1, Xmax: 2, Ymin: 1, Ymax: 2},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{1, 1, 2, 1, 2, 2, 1, 2}},
			}},
		},
	}
	plain := newTestFinder(t, divs)
	defer plain.Close()
	compressed := newTestCompressedFinder(t, divs)
	defer compressed.Close()

	points := [][2]float64{
		{1.5, 1.5},
		{5, 5},
		{12, 5},
		{0, 5},
	}
	for _, p := range points {
		if got, want := compressed.Query(p[0], p[1]), plain.Query(p[0], p[1]); got != want {
			t.Fatalf("compressed Query(%v, %v) = %+v, want %+v", p[0], p[1], got, want)
		}
	}
}

func newTestFinder(t *testing.T, divs []types.Division) *Finder {
	t.Helper()
	coarse := grid.Build(divs, grid.CoarseTierSubtypes, 1.0)
	fine := grid.Build(divs, grid.FineTierSubtypes, 4.0)
	buf, err := flatbuf.EncodeDivisions(divs, coarse, fine, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "divisions.bin")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := NewFinder(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func newTestCompressedFinder(t *testing.T, divs []types.Division) *Finder {
	t.Helper()
	coarse := grid.Build(divs, grid.CoarseTierSubtypes, 1.0)
	fine := grid.Build(divs, grid.FineTierSubtypes, 4.0)
	buf, err := flatbuf.EncodeDivisions(divs, coarse, fine, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	cbuf, _, err := flatbuf.EncodeCompressedDivisionsFromXSFB(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "divisions.cf.bin")
	if err := os.WriteFile(path, cbuf, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := NewCompressedFinder(path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNewFinderAutoDetectsCompressedFormat(t *testing.T) {
	divs := []types.Division{{
		ID:         "country",
		Name:       "Country",
		Subtype:    types.SubtypeCountry,
		AdminLevel: -1,
		BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
		Polygons: []types.Polygon{{
			Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
		}},
	}}
	coarse := grid.Build(divs, grid.CoarseTierSubtypes, 1.0)
	fine := grid.Build(divs, grid.FineTierSubtypes, 4.0)
	buf, err := flatbuf.EncodeDivisions(divs, coarse, fine, "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	cbuf, _, err := flatbuf.EncodeCompressedDivisionsFromXSFB(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "divisions.cf.bin")
	if err := os.WriteFile(path, cbuf, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := NewFinder(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := f.Query(0.5, 0.5).Country; got != "Country" {
		t.Fatalf("Country = %q, want Country", got)
	}
}
