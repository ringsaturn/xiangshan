package geom

import (
	"math"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
)

// fbPolyIndex holds pre-built ring indexes for one FlatBuffers polygon.
type fbPolyIndex struct {
	ext   *fbRingIndex
	holes []*fbRingIndex
}

// DivisionIndex holds pre-built YStripes indexes for one FlatBuffers Division.
// Coordinates remain in the mmap-backed FlatBuffers buffer and are read on demand.
type DivisionIndex struct {
	polys []fbPolyIndex
}

// NewDivisionIndex pre-builds YStripes metadata for a FlatBuffers Division.
func NewDivisionIndex(div *xs.Division) DivisionIndex {
	n := div.PolygonsLength()
	polys := make([]fbPolyIndex, n)
	var fbPoly xs.Polygon
	var fbExt xs.Ring
	var fbHole xs.Ring
	for i := range n {
		if !div.Polygons(&fbPoly, i) {
			continue
		}
		pi := fbPolyIndex{}
		if ext := fbPoly.Exterior(&fbExt); ext != nil {
			pi.ext = buildFBRingIndex(ext)
		}
		if h := fbPoly.HolesLength(); h > 0 {
			pi.holes = make([]*fbRingIndex, h)
			for j := range h {
				if fbPoly.Holes(&fbHole, j) {
					pi.holes[j] = buildFBRingIndex(&fbHole)
				}
			}
		}
		polys[i] = pi
	}
	return DivisionIndex{polys: polys}
}

// ContainsPoint reports whether (lng, lat) lies inside any polygon of div.
func (idx DivisionIndex) ContainsPoint(div *xs.Division, lng, lat float64) bool {
	var fbBBox xs.BBox
	bbox := div.Bbox(&fbBBox)
	if bbox == nil || !BBoxContains(bbox.Xmin(), bbox.Xmax(), bbox.Ymin(), bbox.Ymax(), lng, lat) {
		return false
	}

	p := Point{X: lng, Y: lat}
	var fbPoly xs.Polygon
	for i, pi := range idx.polys {
		if div.Polygons(&fbPoly, i) && polygonContainsPointFB(&fbPoly, pi, p) {
			return true
		}
	}
	return false
}

// BBoxContains reports whether point (lng, lat) lies inside or on a bbox.
func BBoxContains(xmin, xmax, ymin, ymax float32, lng, lat float64) bool {
	return lng >= float64(xmin) && lng <= float64(xmax) &&
		lat >= float64(ymin) && lat <= float64(ymax)
}

// RingContainsPointFB reads coordinates directly from a FlatBuffers Ring.
func RingContainsPointFB(r *xs.Ring, lng, lat float64) bool {
	if r == nil {
		return false
	}
	return ringContainsPointFB(r, nil, Point{X: lng, Y: lat})
}

// PolygonContainsPointFB reads coordinates directly from a FlatBuffers Polygon.
func PolygonContainsPointFB(poly *xs.Polygon, lng, lat float64) bool {
	if poly == nil {
		return false
	}
	return polygonContainsPointFB(poly, fbPolyIndex{}, Point{X: lng, Y: lat})
}

// DivisionContainsPointFB reads coordinates directly from a FlatBuffers Division.
func DivisionContainsPointFB(div *xs.Division, lng, lat float64) bool {
	if div == nil {
		return false
	}
	var fbBBox xs.BBox
	bbox := div.Bbox(&fbBBox)
	if bbox == nil || !BBoxContains(bbox.Xmin(), bbox.Xmax(), bbox.Ymin(), bbox.Ymax(), lng, lat) {
		return false
	}
	p := Point{X: lng, Y: lat}
	var poly xs.Polygon
	for i := 0; i < div.PolygonsLength(); i++ {
		if div.Polygons(&poly, i) && polygonContainsPointFB(&poly, fbPolyIndex{}, p) {
			return true
		}
	}
	return false
}

func polygonContainsPointFB(fbPoly *xs.Polygon, pi fbPolyIndex, p Point) bool {
	var fbExt xs.Ring
	ext := fbPoly.Exterior(&fbExt)
	if ext == nil {
		return false
	}
	if !ringContainsPointFB(ext, pi.ext, p) {
		return false
	}
	var fbHole xs.Ring
	for h := 0; h < fbPoly.HolesLength(); h++ {
		var hIdx *fbRingIndex
		if h < len(pi.holes) {
			hIdx = pi.holes[h]
		}
		if fbPoly.Holes(&fbHole, h) && ringContainsPointFB(&fbHole, hIdx, p) {
			return false
		}
	}
	return true
}

func ringContainsPointFB(fbRing *xs.Ring, idx *fbRingIndex, p Point) bool {
	n := fbRing.CoordsLength() / 2
	if n < 3 {
		return false
	}

	inside := false
	onBoundary := false

	if idx != nil {
		idx.forEachCandidate(p.Y, func(i int) bool {
			j := (i + 1) % n
			res := raycastSeg(fbRingPoint(fbRing, i), fbRingPoint(fbRing, j), p)
			if res.on {
				onBoundary = true
				return false
			}
			if res.inside {
				inside = !inside
			}
			return true
		})
		if onBoundary {
			return true
		}
		return inside
	}

	for i := range n {
		j := (i + 1) % n
		res := raycastSeg(fbRingPoint(fbRing, i), fbRingPoint(fbRing, j), p)
		if res.on {
			return true
		}
		if res.inside {
			inside = !inside
		}
	}
	return inside
}

func fbRingPoint(r *xs.Ring, i int) Point {
	return Point{
		X: float64(r.Coords(i * 2)),
		Y: float64(r.Coords(i*2 + 1)),
	}
}

type fbRingIndex struct {
	minY    float64
	height  float64
	stripes []yStripe
	indexes []int
	yRanges [][2]float64
}

func buildFBRingIndex(r *xs.Ring) *fbRingIndex {
	n := r.CoordsLength() / 2
	if n < minIndexSegments {
		return nil
	}

	yRanges := make([][2]float64, n)
	minY := math.Inf(1)
	maxY := math.Inf(-1)
	var area, perim float64

	for i := range n {
		j := (i + 1) % n
		a := fbRingPoint(r, i)
		b := fbRingPoint(r, j)

		area += a.X*b.Y - b.X*a.Y
		dx, dy := b.X-a.X, b.Y-a.Y
		perim += math.Sqrt(dx*dx + dy*dy)

		if a.Y <= b.Y {
			yRanges[i] = [2]float64{a.Y, b.Y}
		} else {
			yRanges[i] = [2]float64{b.Y, a.Y}
		}
		if yRanges[i][0] < minY {
			minY = yRanges[i][0]
		}
		if yRanges[i][1] > maxY {
			maxY = yRanges[i][1]
		}
	}

	height := maxY - minY
	if height == 0 {
		return nil
	}

	area = math.Abs(area) * 0.5
	score := 0.0
	if perim > 0 {
		score = (area * math.Pi * 4) / (perim * perim)
	}
	stripeCount := max(int(math.Floor(float64(n)*score)), minIndexSegments)

	stripes := make([]yStripe, stripeCount)
	for i := range n {
		lo, hi := segStripeRange(yRanges[i][0], yRanges[i][1], minY, height, stripeCount)
		for s := lo; s <= hi; s++ {
			stripes[s].count++
		}
	}

	total := 0
	starts := make([]int, stripeCount)
	for s := range stripes {
		starts[s] = total
		stripes[s].start = total
		total += stripes[s].count
		stripes[s].count = 0
	}

	indexes := make([]int, total)
	for i := range n {
		lo, hi := segStripeRange(yRanges[i][0], yRanges[i][1], minY, height, stripeCount)
		for s := lo; s <= hi; s++ {
			pos := starts[s] + stripes[s].count
			indexes[pos] = i
			stripes[s].count++
		}
	}

	return &fbRingIndex{
		minY:    minY,
		height:  height,
		stripes: stripes,
		indexes: indexes,
		yRanges: yRanges,
	}
}

func (idx *fbRingIndex) forEachCandidate(y float64, fn func(int) bool) {
	if y < idx.minY || y > idx.minY+idx.height {
		return
	}
	s := pointStripe(y, idx.minY, idx.height, len(idx.stripes))
	stripe := idx.stripes[s]
	for k := stripe.start; k < stripe.start+stripe.count; k++ {
		seg := idx.indexes[k]
		if y >= idx.yRanges[seg][0] && y <= idx.yRanges[seg][1] {
			if !fn(seg) {
				return
			}
		}
	}
}
