package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/ringsaturn/xiangshan"
)

type jsonResult struct {
	Country      string `json:"country,omitempty"`
	Region       string `json:"region,omitempty"`
	County       string `json:"county,omitempty"`
	LocalAdmin   string `json:"local_admin,omitempty"`
	Locality     string `json:"locality,omitempty"`
	CountryID    string `json:"country_id,omitempty"`
	RegionID     string `json:"region_id,omitempty"`
	CountyID     string `json:"county_id,omitempty"`
	LocalAdminID string `json:"local_admin_id,omitempty"`
	LocalityID   string `json:"locality_id,omitempty"`
}

func main() {
	data := flag.String("data", "", "divisions.bin path")
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	if *data == "" {
		fmt.Fprintln(os.Stderr, "data path is required")
		os.Exit(2)
	}
	finder, err := xiangshan.NewFinder(*data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer finder.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/query", queryHandler(finder))

	log.Printf("xs-serve listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}

func queryHandler(finder *xiangshan.Finder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		lng, lat, lang, err := parseQueryPoint(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println("lang", lang)
		res := finder.QueryI18n(lng, lat, lang)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(jsonResult{
			Country:      res.Country,
			Region:       res.Region,
			County:       res.County,
			LocalAdmin:   res.LocalAdmin,
			Locality:     res.Locality,
			CountryID:    res.CountryID,
			RegionID:     res.RegionID,
			CountyID:     res.CountyID,
			LocalAdminID: res.LocalAdminID,
			LocalityID:   res.LocalityID,
		}); err != nil {
			log.Printf("write response: %v", err)
		}
	}
}

func parseQueryPoint(r *http.Request) (float64, float64, string, error) {
	q := r.URL.Query()
	lngText := q.Get("lng")
	latText := q.Get("lat")
	lang := q.Get("lang")
	if lngText == "" || latText == "" {
		return 0, 0, "", fmt.Errorf("lng and lat are required")
	}
	lng, err := strconv.ParseFloat(lngText, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid lng")
	}
	lat, err := strconv.ParseFloat(latText, 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("invalid lat")
	}
	if lng < -180 || lng > 180 {
		return 0, 0, "", fmt.Errorf("lng must be in [-180, 180]")
	}
	if lat < -90 || lat > 90 {
		return 0, 0, "", fmt.Errorf("lat must be in [-90, 90]")
	}
	return lng, lat, lang, nil
}
