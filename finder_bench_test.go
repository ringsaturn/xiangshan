package xiangshan

import (
	"errors"
	"os"
	"testing"

	gocitiesjson "github.com/ringsaturn/go-cities.json"
)

var worldCities = gocitiesjson.Cities

var europeCities = func() []*gocitiesjson.City {
	var cities []*gocitiesjson.City
	for _, city := range worldCities {
		if city.Lng >= -10.0 && city.Lng <= 30.0 && city.Lat >= 35.0 && city.Lat <= 70.0 {
			cities = append(cities, city)
		}
	}
	return cities
}()

var benchResult Result

func benchmarkFinder(b *testing.B) *Finder {
	const binPath = "build/divisions.bin"
	if _, err := os.Stat(binPath); errors.Is(err, os.ErrNotExist) {
		b.Skip("build/divisions.bin not found, run xs-encode first")
	}
	finder, err := NewFinder(binPath)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = finder.Close()
	})
	return finder
}

func BenchmarkQuery_WorldCities(b *testing.B) {
	finder := benchmarkFinder(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		city := worldCities[i%len(worldCities)]
		benchResult = finder.Query(city.Lng, city.Lat)
	}
}

func BenchmarkQuery_DenseEurope(b *testing.B) {
	finder := benchmarkFinder(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		city := europeCities[i%len(europeCities)]
		benchResult = finder.Query(city.Lng, city.Lat)
	}
}

func BenchmarkQuery_Parallel(b *testing.B) {
	finder := benchmarkFinder(b)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		var local Result
		for pb.Next() {
			city := worldCities[i%len(worldCities)]
			local = finder.Query(city.Lng, city.Lat)
			i++
		}
		_ = local
	})
}

// func BenchmarkNewFinder(b *testing.B) {
// 	const binPath = "build/divisions.bin"
// 	if _, err := os.Stat(binPath); errors.Is(err, os.ErrNotExist) {
// 		b.Skip("build/divisions.bin not found, run xs-encode first")
// 	}
// 	b.ReportAllocs()
// 	for i := 0; i < b.N; i++ {
// 		finder, err := NewFinder(binPath)
// 		if err != nil {
// 			b.Fatal(err)
// 		}
// 		if err := finder.Close(); err != nil {
// 			b.Fatal(err)
// 		}
// 	}
// }
