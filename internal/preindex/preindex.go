package preindex

import (
	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/types"
)

const cornerInset = 0.01

// BuildCountry builds a coarse-cell preindex for cells fully covered by a
// single country or dependency candidate.
func BuildCountry(coarseGrid map[[2]int16][]uint32, divs []types.Division) map[[2]int16]uint32 {
	out := make(map[[2]int16]uint32)
	for key, indices := range coarseGrid {
		countryIdx, ok := singleCountryCandidate(indices, divs)
		if !ok {
			continue
		}
		if cellCornersInDivision(key, divs[countryIdx]) {
			out[key] = uint32(countryIdx)
		}
	}
	return out
}

func singleCountryCandidate(indices []uint32, divs []types.Division) (int, bool) {
	countryIdx := -1
	for _, idx := range indices {
		if int(idx) >= len(divs) {
			continue
		}
		switch divs[idx].Subtype {
		case types.SubtypeCountry, types.SubtypeDependency:
			if countryIdx != -1 {
				return 0, false
			}
			countryIdx = int(idx)
		}
	}
	return countryIdx, countryIdx != -1
}

func cellCornersInDivision(key [2]int16, div types.Division) bool {
	lng0 := float64(key[0])
	lat0 := float64(key[1])
	corners := [4][2]float64{
		{lng0 + cornerInset, lat0 + cornerInset},
		{lng0 + 1 - cornerInset, lat0 + cornerInset},
		{lng0 + cornerInset, lat0 + 1 - cornerInset},
		{lng0 + 1 - cornerInset, lat0 + 1 - cornerInset},
	}
	for _, c := range corners {
		if !divisionContainsPoint(div, c[0], c[1]) {
			return false
		}
	}
	return true
}

func divisionContainsPoint(div types.Division, lng, lat float64) bool {
	if !div.BBox.Contains(lng, lat) {
		return false
	}
	for _, poly := range div.Polygons {
		if polygonContainsPoint(poly, lng, lat) {
			return true
		}
	}
	return false
}

func polygonContainsPoint(poly types.Polygon, lng, lat float64) bool {
	if !ringContainsPoint(poly.Exterior, lng, lat) {
		return false
	}
	for _, hole := range poly.Holes {
		if ringContainsPoint(hole, lng, lat) {
			return false
		}
	}
	return true
}

func ringContainsPoint(r types.Ring, lng, lat float64) bool {
	n := len(r.Coords) / 2
	if n < 3 {
		return false
	}
	inside := false
	for i := range n {
		j := (i + n - 1) % n
		xi, yi := ringPoint(r, i)
		xj, yj := ringPoint(r, j)
		if pointOnSegment(lng, lat, xi, yi, xj, yj) {
			return true
		}
		if (yi > lat) != (yj > lat) && lng < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

func ringPoint(r types.Ring, i int) (float64, float64) {
	return float64(r.Coords[i*2]), float64(r.Coords[i*2+1])
}

func pointOnSegment(px, py, ax, ay, bx, by float64) bool {
	const eps = 1e-12
	cross := (px-ax)*(by-ay) - (py-ay)*(bx-ax)
	if cross < -eps || cross > eps {
		return false
	}
	dot := (px-ax)*(px-bx) + (py-ay)*(py-by)
	return dot <= eps
}

// Decode reads a FlatBuffers preindex into the runtime map format.
func Decode(pi *xs.Preindex) map[[2]int16]uint32 {
	if pi == nil {
		return nil
	}
	out := make(map[[2]int16]uint32, pi.CellsLength())
	var cell xs.PreindexCell
	for i := 0; i < pi.CellsLength(); i++ {
		if !pi.Cells(&cell, i) {
			continue
		}
		out[[2]int16{cell.Lng(), cell.Lat()}] = cell.Index()
	}
	return out
}
