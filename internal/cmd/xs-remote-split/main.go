// xs-remote-split splits a .cf.bin compressed divisions file into two files:
//   - <out>.xs-index  lightweight RemoteIndex (grid, metadata, polygon offsets)
//   - <out>.xs-poly   polygon slab (raw CompressedDivision chunks, Range-accessible)
//
// With --compress the index is written as gzip (.xs-index.gz), which is
// typically 2-3x smaller. Load with NewRemoteFinderFromGzip.
package main

import (
	"compress/gzip"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/flatbuf"
)

func main() {
	input := flag.String("input", "", "path to divisions.cf.bin")
	outIndex := flag.String("index", "", "output path for index (default: <input>.xs-index[.gz])")
	outSlab := flag.String("slab", "", "output path for polygon slab (default: <input>.xs-poly)")
	compress := flag.Bool("compress", false, "gzip-compress the index (recommended; use NewRemoteFinderFromGzip to load)")
	flag.Parse()

	if *input == "" {
		flag.Usage()
		os.Exit(1)
	}

	suffix := ".xs-index"
	if *compress {
		suffix = ".xs-index.gz"
	}
	if *outIndex == "" {
		*outIndex = *input + suffix
	}
	if *outSlab == "" {
		*outSlab = *input + ".xs-poly"
	}

	f, err := os.Open(*input)
	if err != nil {
		log.Fatalf("open %s: %v", *input, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Fatalf("stat: %v", err)
	}

	buf, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("mmap: %v", err)
	}
	defer syscall.Munmap(buf)

	if !xs.SizePrefixedCompressedDivisionsBufferHasIdentifier(buf) {
		log.Fatalf("%s is not a valid CompressedDivisions file (expected XSCF)", *input)
	}

	root := xs.GetSizePrefixedRootAsCompressedDivisions(buf, 0)
	log.Printf("divisions: %d, version: %s", root.ItemsLength(), root.Version())
	log.Println("splitting...")

	result, err := flatbuf.SplitCompressedDivisions(root)
	if err != nil {
		log.Fatalf("split: %v", err)
	}

	indexData := result.Index
	if *compress {
		var gz bytes.Buffer
		w := gzip.NewWriter(&gz)
		if _, err := w.Write(result.Index); err != nil {
			log.Fatalf("gzip index: %v", err)
		}
		if err := w.Close(); err != nil {
			log.Fatalf("gzip close: %v", err)
		}
		log.Printf("index compressed: %.1f MB → %.1f MB (%.0f%%)",
			float64(len(result.Index))/(1<<20),
			float64(gz.Len())/(1<<20),
			float64(gz.Len())*100/float64(len(result.Index)),
		)
		indexData = gz.Bytes()
	}

	if err := os.WriteFile(*outIndex, indexData, 0o644); err != nil {
		log.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(*outSlab, result.Slab, 0o644); err != nil {
		log.Fatalf("write slab: %v", err)
	}

	fmt.Printf("index: %s (%.1f MB)\n", *outIndex, float64(len(indexData))/(1<<20))
	fmt.Printf("slab:  %s (%.1f MB)\n", *outSlab, float64(len(result.Slab))/(1<<20))
}
