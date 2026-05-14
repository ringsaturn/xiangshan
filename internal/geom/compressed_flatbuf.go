package geom

import (
	"encoding/binary"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
)

const compressedCoordScale = 100000.0

func CompressedDivisionContainsPointFB(div *xs.CompressedDivision, lng, lat float64) bool {
	if div == nil {
		return false
	}
	var fbBBox xs.BBox
	bbox := div.Bbox(&fbBBox)
	if bbox == nil || !BBoxContains(bbox.Xmin(), bbox.Xmax(), bbox.Ymin(), bbox.Ymax(), lng, lat) {
		return false
	}
	p := Point{X: lng, Y: lat}
	var poly xs.CompressedPolygon
	for i := 0; i < div.PolygonsLength(); i++ {
		if div.Polygons(&poly, i) && compressedPolygonContainsPointFB(&poly, p) {
			return true
		}
	}
	return false
}

func CompressedRingContainsPointFB(r *xs.CompressedRing, lng, lat float64) bool {
	if r == nil {
		return false
	}
	return compressedRingContainsPointFB(r, Point{X: lng, Y: lat})
}

func compressedPolygonContainsPointFB(poly *xs.CompressedPolygon, p Point) bool {
	var ext xs.CompressedRing
	if poly.Exterior(&ext) == nil || !compressedRingContainsPointFB(&ext, p) {
		return false
	}
	var hole xs.CompressedRing
	for i := 0; i < poly.HolesLength(); i++ {
		if poly.Holes(&hole, i) && compressedRingContainsPointFB(&hole, p) {
			return false
		}
	}
	return true
}

func compressedRingContainsPointFB(r *xs.CompressedRing, p Point) bool {
	n := int(r.PointCount())
	if n < 3 {
		return false
	}

	dec := compressedRingDecoder{data: r.DataBytes()}
	first, ok := dec.nextPoint()
	if !ok {
		return false
	}

	prev := first
	inside := false
	for i := 1; i < n; i++ {
		curr, ok := dec.nextPoint()
		if !ok {
			return false
		}
		res := raycastSeg(prev, curr, p)
		if res.on {
			return true
		}
		if res.inside {
			inside = !inside
		}
		prev = curr
	}

	res := raycastSeg(prev, first, p)
	if res.on {
		return true
	}
	if res.inside {
		inside = !inside
	}
	return inside
}

type compressedRingDecoder struct {
	data []byte
	pos  int
	lng  int32
	lat  int32
}

func (d *compressedRingDecoder) nextPoint() (Point, bool) {
	dlng, ok := d.nextDelta()
	if !ok {
		return Point{}, false
	}
	dlat, ok := d.nextDelta()
	if !ok {
		return Point{}, false
	}
	d.lng += dlng
	d.lat += dlat
	return Point{
		X: float64(d.lng) / compressedCoordScale,
		Y: float64(d.lat) / compressedCoordScale,
	}, true
}

func (d *compressedRingDecoder) nextDelta() (int32, bool) {
	if d.pos >= len(d.data) {
		return 0, false
	}
	u, n := binary.Uvarint(d.data[d.pos:])
	if n <= 0 || u > uint64(^uint32(0)) {
		return 0, false
	}
	d.pos += n
	v := uint32(u)
	return int32(v>>1) ^ -int32(v&1), true
}
