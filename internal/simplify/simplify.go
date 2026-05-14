package simplify

import (
	"github.com/paulmach/orb"
	orbsimplify "github.com/paulmach/orb/simplify"
)

var DefaultTolerances = map[string]float64{
	"country":     0.01,
	"dependency":  0.01,
	"macroregion": 0.005,
	"region":      0.005,
	"macrocounty": 0.001,
	"county":      0.001,
	"localadmin":  0.0005,
	"locality":    0.0005,
}

const minSimplifyPoints = 100

func Ring(r orb.Ring, tolerance float64) orb.Ring {
	got, _ := RingWithFallback(r, tolerance)
	return got
}

func RingWithFallback(r orb.Ring, tolerance float64) (orb.Ring, bool) {
	if len(r) < 4 {
		return r, false
	}
	if len(r) <= minSimplifyPoints {
		return r, false
	}
	got := orbsimplify.DouglasPeucker(tolerance).Ring(r.Clone())
	if len(got) < 4 {
		return r, true
	}
	if got[0] != got[len(got)-1] {
		got = append(got, got[0])
	}
	if uniquePoints(got) < 3 {
		return r, true
	}
	return got, false
}

func Polygon(p orb.Polygon, tolerance float64) orb.Polygon {
	got, _ := PolygonWithFallback(p, tolerance)
	return got
}

func PolygonWithFallback(p orb.Polygon, tolerance float64) (orb.Polygon, bool) {
	out := make(orb.Polygon, len(p))
	var fallback bool
	for i, r := range p {
		var ringFallback bool
		out[i], ringFallback = RingWithFallback(r, tolerance)
		if i == 0 {
			fallback = fallback || ringFallback
		}
	}
	return out, fallback
}

func MultiPolygon(mp orb.MultiPolygon, tolerance float64) orb.MultiPolygon {
	got, _ := MultiPolygonWithFallback(mp, tolerance)
	return got
}

func MultiPolygonWithFallback(mp orb.MultiPolygon, tolerance float64) (orb.MultiPolygon, bool) {
	out := make(orb.MultiPolygon, len(mp))
	var fallback bool
	for i, p := range mp {
		var polyFallback bool
		out[i], polyFallback = PolygonWithFallback(p, tolerance)
		fallback = fallback || polyFallback
	}
	return out, fallback
}

func uniquePoints(r orb.Ring) int {
	seen := make(map[orb.Point]struct{}, len(r))
	for i, p := range r {
		if i == len(r)-1 && p == r[0] {
			continue
		}
		seen[p] = struct{}{}
	}
	return len(seen)
}
