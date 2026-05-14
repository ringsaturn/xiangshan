package topology

import (
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/simplify"
)

func simplifyOpenPath(points []*Point, epsilon float64) []*Point {
	if len(points) == 0 {
		return nil
	}
	if len(points) <= minSegmentSimplifyPoints {
		return clonePoints(points)
	}

	original := make(orb.LineString, 0, len(points))
	for _, point := range points {
		original = append(original, orb.Point{float64(point.Lng), float64(point.Lat)})
	}
	reduced := simplify.DouglasPeucker(epsilon).Simplify(original.Clone()).(orb.LineString)
	if len(reduced) < 2 {
		return clonePoints(points)
	}

	output := make([]*Point, 0, len(reduced))
	for _, point := range reduced {
		output = append(output, &Point{
			Lng: float32(point.Lon()),
			Lat: float32(point.Lat()),
		})
	}
	return output
}
