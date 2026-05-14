package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/flatbuf"
)

type config struct {
	input  string
	output string
}

func main() {
	var cfg config
	flag.StringVar(&cfg.input, "input", "build/divisions.bin", "input XSFB divisions path")
	flag.StringVar(&cfg.output, "output", "build/divisions.cf.bin", "output XSCF divisions path")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	if cfg.input == "" || cfg.output == "" {
		return fmt.Errorf("input and output are required")
	}
	in, err := os.ReadFile(cfg.input)
	if err != nil {
		return err
	}
	if !xs.SizePrefixedDivisionsBufferHasIdentifier(in) {
		return fmt.Errorf("input is not a valid XSFB divisions file")
	}
	root := xs.GetSizePrefixedRootAsDivisions(in, 0)
	out, stats, err := flatbuf.EncodeCompressedDivisionsFromXSFB(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.output), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cfg.output, out, 0o644); err != nil {
		return err
	}

	ratio := 0.0
	if len(in) > 0 {
		ratio = float64(len(out)) / float64(len(in))
	}
	bytesPerPoint := 0.0
	if stats.Points > 0 {
		bytesPerPoint = float64(stats.CoordinateBytes) / float64(stats.Points)
	}
	fmt.Printf("divisions=%d rings=%d points=%d input_bytes=%d output_bytes=%d coord_bytes=%d bytes_per_point=%.2f compression_ratio=%.3f\n",
		stats.Divisions, stats.Rings, stats.Points, len(in), len(out), stats.CoordinateBytes, bytesPerPoint, ratio)
	return nil
}
