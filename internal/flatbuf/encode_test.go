package flatbuf

import (
	"testing"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/types"
)

func TestEncodeDivisions(t *testing.T) {
	divs := []types.Division{{
		ID:         "area-1",
		Name:       "Test",
		Subtype:    types.SubtypeCountry,
		AdminLevel: -1,
		Country:    "XX",
		BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
		Polygons: []types.Polygon{{
			Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
		}},
	}}
	buf, err := EncodeDivisions(divs, map[[2]int16][]uint32{{0, 0}: {0}}, nil, "v", "s")
	if err != nil {
		t.Fatal(err)
	}
	if !xs.SizePrefixedDivisionsBufferHasIdentifier(buf) {
		t.Fatal("missing size-prefixed identifier")
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	if root.ItemsLength() != 1 {
		t.Fatalf("ItemsLength = %d, want 1", root.ItemsLength())
	}
	if root.GridCoarse(nil).CellsLength() != 1 {
		t.Fatalf("GridCoarse cells = %d, want 1", root.GridCoarse(nil).CellsLength())
	}
	if root.CountryPreindex(nil).CellsLength() != 1 {
		t.Fatalf("CountryPreindex cells = %d, want 1", root.CountryPreindex(nil).CellsLength())
	}
}

func TestEncodeCompressedDivisionsFromXSFB(t *testing.T) {
	divs := []types.Division{{
		ID:         "area-1",
		Name:       "Test",
		Subtype:    types.SubtypeCountry,
		AdminLevel: -1,
		Country:    "XX",
		BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
		Polygons: []types.Polygon{{
			Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
		}},
	}}
	buf, err := EncodeDivisions(divs, map[[2]int16][]uint32{{0, 0}: {0}}, nil, "v", "s")
	if err != nil {
		t.Fatal(err)
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	cbuf, stats, err := EncodeCompressedDivisionsFromXSFB(root)
	if err != nil {
		t.Fatal(err)
	}
	if !xs.SizePrefixedCompressedDivisionsBufferHasIdentifier(cbuf) {
		t.Fatal("missing compressed size-prefixed identifier")
	}
	if stats.Divisions != 1 || stats.Rings != 1 || stats.Points != 4 {
		t.Fatalf("stats = %+v, want one division, one ring, four points", stats)
	}
	croot := xs.GetSizePrefixedRootAsCompressedDivisions(cbuf, 0)
	if croot.ItemsLength() != 1 {
		t.Fatalf("ItemsLength = %d, want 1", croot.ItemsLength())
	}
	if croot.GridCoarse(nil).CellsLength() != 1 {
		t.Fatalf("GridCoarse cells = %d, want 1", croot.GridCoarse(nil).CellsLength())
	}
	if croot.CountryPreindex(nil).CellsLength() != 1 {
		t.Fatalf("CountryPreindex cells = %d, want 1", croot.CountryPreindex(nil).CellsLength())
	}
}
