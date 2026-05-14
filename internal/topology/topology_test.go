package topology

import "testing"

func TestDoWithStatsSimplifiesSharedBoundaryOnce(t *testing.T) {
	input := &Dataset{Divisions: []*Division{
		{
			Name: "left",
			Polygons: []*Polygon{{
				Points: ring(
					pt(0, 0), pt(1, 0), pt(1, 0.25), pt(1, 0.5), pt(1, 0.75), pt(1, 1), pt(0, 1), pt(0, 0),
				),
			}},
		},
		{
			Name: "right",
			Polygons: []*Polygon{{
				Points: ring(
					pt(1, 0), pt(2, 0), pt(2, 1), pt(1, 1), pt(1, 0.75), pt(1, 0.5), pt(1, 0.25), pt(1, 0),
				),
			}},
		},
	}}

	output, stats := DoWithStats(input, 0.2)
	if err := ValidateWithOptions(output, ReductionValidateOptions()); err != nil {
		t.Fatal(err)
	}
	if stats.SharedCacheMisses == 0 || stats.SharedCacheHits == 0 {
		t.Fatalf("shared cache misses=%d hits=%d, want both non-zero", stats.SharedCacheMisses, stats.SharedCacheHits)
	}

	left := output.Divisions[0].Polygons[0].Points
	right := output.Divisions[1].Polygons[0].Points
	leftShared := xEdge(left, 1)
	rightShared := xEdge(right, 1)
	if len(leftShared) != 2 {
		t.Fatalf("left shared edge points = %v, want two endpoints", leftShared)
	}
	if len(rightShared) != 2 {
		t.Fatalf("right shared edge points = %v, want two endpoints", rightShared)
	}
	if !samePoint(leftShared[0], rightShared[1]) || !samePoint(leftShared[1], rightShared[0]) {
		t.Fatalf("shared edge mismatch: left=%v right=%v", leftShared, rightShared)
	}
}

func TestDoWithOptionsSkipsSmallAreaRings(t *testing.T) {
	input := &Dataset{Divisions: []*Division{{
		Name: "small",
		Polygons: []*Polygon{{
			Points: ring(
				pt(0, 0), pt(0.1, 0), pt(0.1, 0.05), pt(0.1, 0.1), pt(0, 0.1), pt(0, 0),
			),
		}},
	}}}

	output, stats := DoWithOptions(input, Options{Epsilon: 0.2, MinRingArea: 1})
	if stats.RingsSkippedSmall != 1 {
		t.Fatalf("RingsSkippedSmall = %d, want 1", stats.RingsSkippedSmall)
	}
	got := output.Divisions[0].Polygons[0].Points
	if len(got) != len(input.Divisions[0].Polygons[0].Points) {
		t.Fatalf("points = %d, want %d", len(got), len(input.Divisions[0].Polygons[0].Points))
	}
}

func TestDoWithOptionsPreservesSharedEdgeWithSmallNeighbor(t *testing.T) {
	input := &Dataset{Divisions: []*Division{
		{
			Name: "large",
			Polygons: []*Polygon{{
				Points: ring(
					pt(0, 0), pt(1, 0), pt(1, 0.25), pt(1, 0.5), pt(1, 0.75), pt(1, 1), pt(0, 1), pt(0, 0),
				),
			}},
		},
		{
			Name: "small-neighbor",
			Polygons: []*Polygon{{
				Points: ring(
					pt(1, 0), pt(1.1, 0), pt(1.1, 1), pt(1, 1), pt(1, 0.75), pt(1, 0.5), pt(1, 0.25), pt(1, 0),
				),
			}},
		},
	}}

	output, stats := DoWithOptions(input, Options{Epsilon: 0.2, MinRingArea: 0.5})
	if stats.RingsSkippedSmall != 1 {
		t.Fatalf("RingsSkippedSmall = %d, want 1", stats.RingsSkippedSmall)
	}
	gotSharedPoints := countPointsOnX(output.Divisions[0].Polygons[0].Points, 1)
	if gotSharedPoints < 5 {
		t.Fatalf("large shared edge points on x=1 = %d, want at least 5", gotSharedPoints)
	}
}

func ring(points ...*Point) []*Point {
	return points
}

func pt(lng, lat float32) *Point {
	return &Point{Lng: lng, Lat: lat}
}

func xEdge(points []*Point, x float32) []*Point {
	out := make([]*Point, 0, 2)
	for i, p := range points {
		next := points[(i+1)%len(points)]
		if p.Lng == x && next.Lng == x && p.Lat != next.Lat {
			out = append(out, p, next)
			return out
		}
	}
	return nil
}

func countPointsOnX(points []*Point, x float32) int {
	n := 0
	for _, p := range points {
		if p.Lng == x {
			n++
		}
	}
	return n
}
