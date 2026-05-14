package grid

import (
	"math"
	"slices"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/types"
)

var CoarseTierSubtypes = map[types.Subtype]bool{
	types.SubtypeCountry:     true,
	types.SubtypeDependency:  true,
	types.SubtypeMacroRegion: true,
	types.SubtypeRegion:      true,
	types.SubtypeMacroCounty: true,
}

var FineTierSubtypes = map[types.Subtype]bool{
	types.SubtypeCounty:     true,
	types.SubtypeLocalAdmin: true,
	types.SubtypeLocality:   true,
}

func Build(divs []types.Division, subtypes map[types.Subtype]bool, scale float64) map[[2]int16][]uint32 {
	cells := make(map[[2]int16][]uint32, len(divs)*2)
	for i, d := range divs {
		if !subtypes[d.Subtype] {
			continue
		}
		lngMin := int16(math.Floor(float64(d.BBox.Xmin) * scale))
		lngMax := int16(math.Floor(float64(d.BBox.Xmax) * scale))
		latMin := int16(math.Floor(float64(d.BBox.Ymin) * scale))
		latMax := int16(math.Floor(float64(d.BBox.Ymax) * scale))
		for lng := lngMin; lng <= lngMax; lng++ {
			for lat := latMin; lat <= latMax; lat++ {
				key := [2]int16{lng, lat}
				cells[key] = append(cells[key], uint32(i))
			}
		}
	}
	for k := range cells {
		slices.Sort(cells[k])
	}
	return cells
}

func CoarseKey(lng, lat float64) [2]int16 {
	return [2]int16{int16(math.Floor(lng)), int16(math.Floor(lat))}
}

func FineKey(lng, lat float64) [2]int16 {
	return [2]int16{int16(math.Floor(lng * 4)), int16(math.Floor(lat * 4))}
}

func DecodeFromFlatbuf(gi *xs.GridIndex) map[[2]int16][]uint32 {
	if gi == nil {
		return nil
	}
	out := make(map[[2]int16][]uint32, gi.CellsLength())
	var cell xs.GridCell
	for i := 0; i < gi.CellsLength(); i++ {
		if !gi.Cells(&cell, i) {
			continue
		}
		indices := make([]uint32, cell.IndicesLength())
		for j := range indices {
			indices[j] = cell.Indices(j)
		}
		out[[2]int16{cell.Lng(), cell.Lat()}] = indices
	}
	return out
}
