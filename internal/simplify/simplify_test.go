package simplify

import (
	"math"
	"testing"

	"github.com/paulmach/orb"
)

func TestSimplifyRingNoDegeneracy(t *testing.T) {
	n := 100
	ring := make(orb.Ring, n+1)
	for i := 0; i < n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		ring[i] = orb.Point{math.Cos(angle), math.Sin(angle)}
	}
	ring[n] = ring[0]

	got := Ring(ring, 0.01)
	if len(got) < 4 {
		t.Errorf("simplified ring has %d points, want at least 4", len(got))
	}
}

func TestSimplifyRingFallbackOnDegeneracy(t *testing.T) {
	ring := orb.Ring{
		{0, 0}, {0.00001, 0}, {0.00001, 0.00001}, {0, 0},
	}
	got := Ring(ring, 1.0)
	if len(got) != len(ring) {
		t.Errorf("expected fallback to original %d points, got %d points", len(ring), len(got))
	}
}
