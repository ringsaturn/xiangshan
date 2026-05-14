package preindex

import (
	"testing"

	"github.com/ringsaturn/xiangshan/internal/types"
)

func TestBuildCountry(t *testing.T) {
	divs := []types.Division{
		{
			ID:         "country",
			Name:       "Country",
			Subtype:    types.SubtypeCountry,
			AdminLevel: -1,
			BBox:       types.BBox{Xmin: 0, Xmax: 2, Ymin: 0, Ymax: 1},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{0, 0, 2, 0, 2, 1, 0, 1}},
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
	got := BuildCountry(map[[2]int16][]uint32{
		{0, 0}: {0, 1},
		{1, 0}: {0},
	}, divs)
	if got[[2]int16{0, 0}] != 0 {
		t.Fatalf("preindex[0,0] = %d, want 0", got[[2]int16{0, 0}])
	}
	if got[[2]int16{1, 0}] != 0 {
		t.Fatalf("preindex[1,0] = %d, want 0", got[[2]int16{1, 0}])
	}
	if len(got) != 2 {
		t.Fatalf("preindex cells = %d, want 2", len(got))
	}
}

func TestBuildCountryRejectsMultipleCountries(t *testing.T) {
	divs := []types.Division{
		{
			ID:         "a",
			Name:       "A",
			Subtype:    types.SubtypeCountry,
			AdminLevel: -1,
			BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
			}},
		},
		{
			ID:         "b",
			Name:       "B",
			Subtype:    types.SubtypeDependency,
			AdminLevel: -1,
			BBox:       types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1},
			Polygons: []types.Polygon{{
				Exterior: types.Ring{Coords: []float32{0, 0, 1, 0, 1, 1, 0, 1}},
			}},
		},
	}
	got := BuildCountry(map[[2]int16][]uint32{{0, 0}: {0, 1}}, divs)
	if len(got) != 0 {
		t.Fatalf("preindex cells = %d, want 0", len(got))
	}
}
