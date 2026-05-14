package xiangshan

import (
	"errors"
	"os"
	"testing"
	"time"
)

var integrationFinder *Finder

func TestMain(m *testing.M) {
	const binPath = "build/divisions.bin"
	if _, err := os.Stat(binPath); err == nil {
		integrationFinder, err = NewFinder(binPath)
		if err != nil {
			panic(err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}
	code := m.Run()
	if integrationFinder != nil {
		_ = integrationFinder.Close()
	}
	os.Exit(code)
}

func requireIntegrationFinder(t *testing.T) *Finder {
	t.Helper()
	if integrationFinder == nil {
		t.Skip("build/divisions.bin not found, run xs-encode first")
	}
	return integrationFinder
}

func TestQueryGolden(t *testing.T) {
	f := requireIntegrationFinder(t)
	cases := []struct {
		name        string
		lng, lat    float64
		wantCountry string
		wantRegion  string
		wantCounty  string
		wantLocal   string
	}{
		{"ParisEiffelTower", 2.2945, 48.8584, "France", "Île-de-France", "Paris", "Paris"},
		{"BeijingTiananmen", 116.3974, 39.9087, "中国", "北京市", "", "东城区"},
		{"NewYorkEmpireState", -73.9857, 40.7484, "United States", "New York", "New York County", ""},
		{"TokyoShinjuku", 139.6917, 35.6895, "日本", "東京都", "Shinjuku", ""},
		{"SydneyOperaHouse", 151.2153, -33.8568, "Australia", "New South Wales", "Council of the City of Sydney", ""},
		{"NullIsland", 0.0, 0.0, "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := f.Query(c.lng, c.lat)
			if got.Country != c.wantCountry {
				t.Errorf("Country = %q, want %q", got.Country, c.wantCountry)
			}
			if got.Region != c.wantRegion {
				t.Errorf("Region = %q, want %q", got.Region, c.wantRegion)
			}
			if got.County != c.wantCounty {
				t.Errorf("County = %q, want %q", got.County, c.wantCounty)
			}
			if got.LocalAdmin != c.wantLocal {
				t.Errorf("LocalAdmin = %q, want %q", got.LocalAdmin, c.wantLocal)
			}
		})
	}
}

func TestQueryEdgeCases(t *testing.T) {
	f := requireIntegrationFinder(t)
	cases := []struct {
		name     string
		lng, lat float64
	}{
		{"FijiAntimeridian", 179.9, -16.5},
		{"MaxLng", 180.0, 0.0},
		{"MinLat", 0.0, -90.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_ = f.Query(c.lng, c.lat)
		})
	}

	start := time.Now()
	for i := 0; i < 10000; i++ {
		f.Query(16.0, 49.0)
	}
	if avg := time.Since(start) / 10000; avg > time.Millisecond {
		t.Errorf("avg query time = %v, want less than 1ms", avg)
	}
}

func TestQueryAllocsZero(t *testing.T) {
	f := requireIntegrationFinder(t)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = f.Query(2.2945, 48.8584)
	})
	if allocs != 0 {
		t.Fatalf("Query allocations = %v, want 0", allocs)
	}
}
