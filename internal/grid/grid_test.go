package grid

import (
	"slices"
	"testing"

	"github.com/ringsaturn/xiangshan/internal/types"
)

func TestCoarseKey(t *testing.T) {
	cases := []struct {
		lng, lat float64
		want     [2]int16
	}{
		{2.2945, 48.8584, [2]int16{2, 48}},
		{116.3974, 39.9087, [2]int16{116, 39}},
		{-73.9857, 40.7484, [2]int16{-74, 40}},
		{-74.006, 40.712, [2]int16{-75, 40}},
		{179.9, -16.5, [2]int16{179, -17}},
		{0.0, 0.0, [2]int16{0, 0}},
		{-180.0, -90.0, [2]int16{-180, -90}},
	}
	for _, c := range cases {
		if got := CoarseKey(c.lng, c.lat); got != c.want {
			t.Errorf("CoarseKey(%v, %v) = %v, want %v", c.lng, c.lat, got, c.want)
		}
	}
}

func TestFineKey(t *testing.T) {
	cases := []struct {
		lng, lat float64
		want     [2]int16
	}{
		{2.2945, 48.8584, [2]int16{9, 195}},
		{116.3974, 39.9087, [2]int16{465, 159}},
		{-73.9857, 40.7484, [2]int16{-296, 162}},
		{0.0, 0.0, [2]int16{0, 0}},
	}
	for _, c := range cases {
		if got := FineKey(c.lng, c.lat); got != c.want {
			t.Errorf("FineKey(%v, %v) = %v, want %v", c.lng, c.lat, got, c.want)
		}
	}
}

func TestBuildSingleDivision(t *testing.T) {
	divs := []types.Division{{
		Subtype: types.SubtypeCounty,
		BBox:    types.BBox{Xmin: 2.0, Xmax: 2.5, Ymin: 48.0, Ymax: 48.5},
	}}
	cells := Build(divs, FineTierSubtypes, 4.0)
	if got := len(cells); got != 9 {
		t.Errorf("cell count = %d, want 9", got)
	}
	for k, v := range cells {
		if len(v) != 1 || v[0] != 0 {
			t.Errorf("cell %v: got %v, want [0]", k, v)
		}
	}
}

func TestBuildIndicesSorted(t *testing.T) {
	divs := []types.Division{
		{Subtype: types.SubtypeCounty, BBox: types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1}},
		{Subtype: types.SubtypeCounty, BBox: types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1}},
	}
	cells := Build(divs, FineTierSubtypes, 4.0)
	for _, v := range cells {
		if !slices.IsSorted(v) {
			t.Errorf("indices not sorted: %v", v)
		}
	}
}

func TestBuildSubtypeFilter(t *testing.T) {
	divs := []types.Division{
		{Subtype: types.SubtypeLocalAdmin, BBox: types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1}},
		{Subtype: types.SubtypeCountry, BBox: types.BBox{Xmin: 0, Xmax: 1, Ymin: 0, Ymax: 1}},
	}
	cells := Build(divs, CoarseTierSubtypes, 1.0)
	for _, v := range cells {
		for _, idx := range v {
			if idx == 0 {
				t.Errorf("localadmin idx 0 appeared in coarse grid")
			}
		}
	}
}
