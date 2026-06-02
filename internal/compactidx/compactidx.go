// Package compactidx provides a compact binary format for the remote finder index.
//
// The format stores per-division metadata and grid lookup tables without the
// per-table overhead of FlatBuffers, resulting in a ~3x smaller file and lower
// in-memory footprint compared to the FlatBuffers RemoteIndex.
//
// File layout:
//
//	Header          24 bytes
//	Division section  variable (one record per division, sequential)
//	Coarse grid       variable
//	Fine grid         variable
//	Preindex          N × 8 bytes
//
// All multibyte integers are little-endian.
package compactidx

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
)

// Magic is the 4-byte file identifier.
const Magic = "XSCI"

const currentVersion = byte(1)

// DivRecord is the minimal per-division data written to the compact index.
// Only subtype, bbox, and slab location are stored. name, id, and
// names_common live exclusively in the slab chunk and are read after a
// polygon hit, keeping the index as small as possible.
type DivRecord struct {
	Subtype    uint8
	BBox       [4]float32 // xmin, xmax, ymin, ymax
	PolyOffset uint64
	PolyLength uint32
}

// Index is the fully-loaded compact index, ready for query use.
type Index struct {
	Records         []DivRecord
	GridCoarse      map[[2]int16][]uint32
	GridFine        map[[2]int16][]uint32
	CountryPreindex map[[2]int16]uint32
}

// Write serializes divs, grids, and preindex to w in compact binary format.
// The caller is responsible for any outer compression (e.g. gzip).
func Write(
	w io.Writer,
	divs []DivRecord,
	coarse, fine map[[2]int16][]uint32,
	preindex map[[2]int16]uint32,
) error {
	bw := bufio.NewWriterSize(w, 1<<20)

	// Header: [4]magic [1]version [3]reserved [4]div_count [4]coarse_count [4]fine_count [4]preindex_count
	var hdr [24]byte
	copy(hdr[0:4], Magic)
	hdr[4] = currentVersion
	putu32(hdr[8:12], uint32(len(divs)))
	putu32(hdr[12:16], uint32(len(coarse)))
	putu32(hdr[16:20], uint32(len(fine)))
	putu32(hdr[20:24], uint32(len(preindex)))
	if _, err := bw.Write(hdr[:]); err != nil {
		return err
	}

	// Division records: subtype + bbox + poly_offset + poly_length only.
	// name, id, and names_common live in the slab chunk.
	var scratch [8]byte
	for i := range divs {
		r := &divs[i]

		scratch[0] = r.Subtype
		bw.Write(scratch[:1])

		putf32(scratch[0:4], r.BBox[0])
		putf32(scratch[4:8], r.BBox[1])
		bw.Write(scratch[:8])
		putf32(scratch[0:4], r.BBox[2])
		putf32(scratch[4:8], r.BBox[3])
		bw.Write(scratch[:8])

		putu64(scratch[:8], r.PolyOffset)
		bw.Write(scratch[:8])
		putu32(scratch[:4], r.PolyLength)
		bw.Write(scratch[:4])
	}

	// Grid sections (coarse then fine)
	for _, cells := range []map[[2]int16][]uint32{coarse, fine} {
		if err := writeGrid(bw, cells, scratch); err != nil {
			return err
		}
	}

	// Preindex
	if err := writePreindex(bw, preindex, scratch); err != nil {
		return err
	}

	return bw.Flush()
}

func writeGrid(bw *bufio.Writer, cells map[[2]int16][]uint32, scratch [8]byte) error {
	keys := sortedKeys(cells)
	for _, k := range keys {
		indices := cells[k]
		if len(indices) > 0xFFFF {
			return fmt.Errorf("compactidx: cell (%d,%d) has %d indices (max 65535)", k[0], k[1], len(indices))
		}
		putu16(scratch[0:2], uint16(k[0]))
		putu16(scratch[2:4], uint16(k[1]))
		putu16(scratch[4:6], uint16(len(indices)))
		bw.Write(scratch[:6])
		for _, idx := range indices {
			putu32(scratch[:4], idx)
			bw.Write(scratch[:4])
		}
	}
	return nil
}

func writePreindex(bw *bufio.Writer, preindex map[[2]int16]uint32, scratch [8]byte) error {
	ks := make([][2]int16, 0, len(preindex))
	for k := range preindex {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		if ks[i][0] != ks[j][0] {
			return ks[i][0] < ks[j][0]
		}
		return ks[i][1] < ks[j][1]
	})
	for _, k := range ks {
		putu16(scratch[0:2], uint16(k[0]))
		putu16(scratch[2:4], uint16(k[1]))
		putu32(scratch[4:8], preindex[k])
		bw.Write(scratch[:8])
	}
	return nil
}

func sortedKeys(cells map[[2]int16][]uint32) [][2]int16 {
	keys := make([][2]int16, 0, len(cells))
	for k := range cells {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	return keys
}

// Read parses a compact index from r.
// r must be positioned at the start of the file (after any outer decompression).
func Read(r io.Reader) (*Index, error) {
	br := bufio.NewReaderSize(r, 1<<20)

	var hdr [24]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return nil, fmt.Errorf("compactidx: read header: %w", err)
	}
	if string(hdr[0:4]) != Magic {
		return nil, fmt.Errorf("compactidx: invalid magic %q (expected %q)", hdr[0:4], Magic)
	}
	if hdr[4] != currentVersion {
		return nil, fmt.Errorf("compactidx: unsupported version %d", hdr[4])
	}

	divCount := getu32(hdr[8:12])
	coarseCount := getu32(hdr[12:16])
	fineCount := getu32(hdr[16:20])
	preindexCount := getu32(hdr[20:24])

	records := make([]DivRecord, divCount)
	for i := range records {
		r2, err := readDivRecord(br)
		if err != nil {
			return nil, fmt.Errorf("compactidx: division %d: %w", i, err)
		}
		records[i] = r2
	}

	coarse, err := readGrid(br, coarseCount)
	if err != nil {
		return nil, fmt.Errorf("compactidx: coarse grid: %w", err)
	}
	fine, err := readGrid(br, fineCount)
	if err != nil {
		return nil, fmt.Errorf("compactidx: fine grid: %w", err)
	}
	preindex, err := readPreindex(br, preindexCount)
	if err != nil {
		return nil, fmt.Errorf("compactidx: preindex: %w", err)
	}

	return &Index{
		Records:         records,
		GridCoarse:      coarse,
		GridFine:        fine,
		CountryPreindex: preindex,
	}, nil
}

func readDivRecord(br *bufio.Reader) (DivRecord, error) {
	var r DivRecord
	var b8 [8]byte

	sub, err := br.ReadByte()
	if err != nil {
		return r, err
	}
	r.Subtype = sub

	if _, err := io.ReadFull(br, b8[:8]); err != nil {
		return r, err
	}
	r.BBox[0] = getf32(b8[0:4])
	r.BBox[1] = getf32(b8[4:8])

	if _, err := io.ReadFull(br, b8[:8]); err != nil {
		return r, err
	}
	r.BBox[2] = getf32(b8[0:4])
	r.BBox[3] = getf32(b8[4:8])

	if _, err := io.ReadFull(br, b8[:8]); err != nil {
		return r, err
	}
	r.PolyOffset = getu64(b8[:8])

	if _, err := io.ReadFull(br, b8[:4]); err != nil {
		return r, err
	}
	r.PolyLength = getu32(b8[:4])

	return r, nil
}

func readGrid(br *bufio.Reader, count uint32) (map[[2]int16][]uint32, error) {
	cells := make(map[[2]int16][]uint32, count)
	var b8 [8]byte
	for range count {
		if _, err := io.ReadFull(br, b8[:6]); err != nil {
			return nil, err
		}
		lng := int16(getu16(b8[0:2]))
		lat := int16(getu16(b8[2:4]))
		n := getu16(b8[4:6])
		indices := make([]uint32, n)
		for j := range indices {
			if _, err := io.ReadFull(br, b8[:4]); err != nil {
				return nil, err
			}
			indices[j] = getu32(b8[:4])
		}
		cells[[2]int16{lng, lat}] = indices
	}
	return cells, nil
}

func readPreindex(br *bufio.Reader, count uint32) (map[[2]int16]uint32, error) {
	preindex := make(map[[2]int16]uint32, count)
	var b8 [8]byte
	for range count {
		if _, err := io.ReadFull(br, b8[:8]); err != nil {
			return nil, err
		}
		lng := int16(getu16(b8[0:2]))
		lat := int16(getu16(b8[2:4]))
		preindex[[2]int16{lng, lat}] = getu32(b8[4:8])
	}
	return preindex, nil
}

// --- binary helpers (little-endian) ---

func putu16(b []byte, v uint16) { binary.LittleEndian.PutUint16(b, v) }
func putu32(b []byte, v uint32) { binary.LittleEndian.PutUint32(b, v) }
func putu64(b []byte, v uint64) { binary.LittleEndian.PutUint64(b, v) }
func putf32(b []byte, v float32) {
	binary.LittleEndian.PutUint32(b, math.Float32bits(v))
}

func getu16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func getu32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func getu64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }
func getf32(b []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}
