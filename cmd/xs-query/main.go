package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/ringsaturn/xiangshan"
)

type config struct {
	data   string
	lng    float64
	lat    float64
	stdin  bool
	format string
	lang   string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.data, "data", "", "divisions.bin path")
	flag.Float64Var(&cfg.lng, "lng", math.NaN(), "longitude")
	flag.Float64Var(&cfg.lat, "lat", math.NaN(), "latitude")
	flag.BoolVar(&cfg.stdin, "stdin", false, `read "lng,lat" lines from stdin`)
	flag.StringVar(&cfg.format, "format", "text", "output format: text or json")
	flag.StringVar(&cfg.lang, "lang", "en", "language for output")
	flag.Parse()

	if err := run(os.Stdin, os.Stdout, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer, cfg config) error {
	if cfg.data == "" {
		return fmt.Errorf("data path is required")
	}
	if cfg.format != "text" && cfg.format != "json" {
		return fmt.Errorf("format must be text or json")
	}
	finder, err := xiangshan.NewFinder(cfg.data)
	if err != nil {
		return err
	}
	defer finder.Close()

	if cfg.stdin {
		return queryLines(in, out, finder, cfg.format)
	}
	if math.IsNaN(cfg.lng) || math.IsNaN(cfg.lat) {
		return fmt.Errorf("lng and lat are required unless -stdin is set")
	}
	if err := validatePoint(cfg.lng, cfg.lat); err != nil {
		return err
	}
	return writeResult(out, finder.QueryI18n(cfg.lng, cfg.lat, cfg.lang), cfg.format)
}

func queryLines(in io.Reader, out io.Writer, finder *xiangshan.Finder, format string) error {
	scanner := bufio.NewScanner(in)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lng, lat, err := parsePoint(line)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		if err := validatePoint(lng, lat); err != nil {
			return fmt.Errorf("line %d: %w", lineNo, err)
		}
		if err := writeResult(out, finder.Query(lng, lat), format); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func parsePoint(line string) (float64, float64, error) {
	left, right, ok := strings.Cut(line, ",")
	if !ok {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return 0, 0, fmt.Errorf(`expected "lng,lat"`)
		}
		left, right = fields[0], fields[1]
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(left), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %w", err)
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %w", err)
	}
	return lng, lat, nil
}

func validatePoint(lng, lat float64) error {
	if lng < -180 || lng > 180 {
		return fmt.Errorf("lng must be in [-180, 180]")
	}
	if lat < -90 || lat > 90 {
		return fmt.Errorf("lat must be in [-90, 90]")
	}
	return nil
}

func writeResult(out io.Writer, r xiangshan.Result, format string) error {
	if format == "json" {
		return json.NewEncoder(out).Encode(r)
	}
	_, err := fmt.Fprintf(out, "Country: %s  Region: %s  County: %s  LocalAdmin: %s\n",
		r.Country, r.Region, r.County, r.LocalAdmin)
	return err
}
