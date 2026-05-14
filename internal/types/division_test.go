package types

import "testing"

func TestBBoxContains(t *testing.T) {
	bb := BBox{Xmin: 0.0, Xmax: 10.0, Ymin: 0.0, Ymax: 10.0}
	cases := []struct {
		lng, lat float64
		want     bool
	}{
		{5.0, 5.0, true},
		{0.0, 0.0, true},
		{10.0, 10.0, true},
		{10.001, 5.0, false},
		{-0.001, 5.0, false},
		{5.0, -0.001, false},
	}
	for _, c := range cases {
		if got := bb.Contains(c.lng, c.lat); got != c.want {
			t.Errorf("BBox.Contains(%v, %v) = %v, want %v", c.lng, c.lat, got, c.want)
		}
	}
}
