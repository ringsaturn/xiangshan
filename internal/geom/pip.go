// Derived from github.com/tidwall/geojson; see LICENSE_GEOJSON.

package geom

import "math"

// raycastResult is the outcome of testing one segment against a horizontal ray.
type raycastResult struct {
	inside bool
	on     bool
}

// raycastSeg tests whether a leftward horizontal ray from p crosses segment (a, b).
func raycastSeg(a, b, p Point) raycastResult {
	py := p.Y

	if a.Y < b.Y {
		if py < a.Y || py > b.Y {
			return raycastResult{}
		}
	} else if a.Y > b.Y {
		if py < b.Y || py > a.Y {
			return raycastResult{}
		}
	}

	if a.Y == b.Y {
		if a.X == b.X {
			if p.X == a.X && py == a.Y {
				return raycastResult{on: true}
			}
			return raycastResult{}
		}
		if py == b.Y {
			if a.X < b.X {
				if p.X >= a.X && p.X <= b.X {
					return raycastResult{on: true}
				}
			} else if p.X >= b.X && p.X <= a.X {
				return raycastResult{on: true}
			}
		}
	}
	if a.X == b.X && p.X == b.X {
		if a.Y < b.Y {
			if py >= a.Y && py <= b.Y {
				return raycastResult{on: true}
			}
		} else if py >= b.Y && py <= a.Y {
			return raycastResult{on: true}
		}
	}
	if (p.X-a.X)/(b.X-a.X) == (py-a.Y)/(b.Y-a.Y) {
		return raycastResult{on: true}
	}

	for py == a.Y || py == b.Y {
		py = math.Nextafter(py, math.Inf(1))
	}

	if a.Y < b.Y {
		if py < a.Y || py > b.Y {
			return raycastResult{}
		}
	} else if py < b.Y || py > a.Y {
		return raycastResult{}
	}

	if a.X > b.X {
		if p.X >= a.X {
			return raycastResult{}
		}
		if p.X <= b.X {
			return raycastResult{inside: true}
		}
	} else {
		if p.X >= b.X {
			return raycastResult{}
		}
		if p.X <= a.X {
			return raycastResult{inside: true}
		}
	}

	if a.Y < b.Y {
		if (py-a.Y)/(p.X-a.X) >= (b.Y-a.Y)/(b.X-a.X) {
			return raycastResult{inside: true}
		}
	} else if (py-b.Y)/(p.X-b.X) >= (a.Y-b.Y)/(a.X-b.X) {
		return raycastResult{inside: true}
	}
	return raycastResult{}
}

// RaycastResult is the exported outcome of RaycastSeg.
type RaycastResult struct{ Inside, On bool }

// RaycastSeg is the exported entry point for the ray-casting segment test,
// for use by callers that drive their own ring iteration (e.g. FlatBuffers rings).
func RaycastSeg(a, b, p Point) RaycastResult {
	r := raycastSeg(a, b, p)
	return RaycastResult{Inside: r.inside, On: r.on}
}

// ringContainsPoint reports whether p is strictly inside ring r using the
// even-odd ray-casting rule. Points on the ring boundary return false.
func ringContainsPoint(r Ring, idx *yStripesIndex, p Point) bool {
	n := len(r)
	if n < 3 {
		return false
	}

	inside := false

	if idx != nil {
		idx.forEachCandidate(p.Y, func(i int) bool {
			j := (i + 1) % n
			res := raycastSeg(r[i], r[j], p)
			if res.on {
				inside = false
				return false
			}
			if res.inside {
				inside = !inside
			}
			return true
		})
		return inside
	}

	for i := range n {
		j := (i + 1) % n
		res := raycastSeg(r[i], r[j], p)
		if res.on {
			return false
		}
		if res.inside {
			inside = !inside
		}
	}
	return inside
}
