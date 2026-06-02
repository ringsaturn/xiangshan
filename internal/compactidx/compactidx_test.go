package compactidx_test

import (
	"bytes"
	"testing"

	"github.com/ringsaturn/xiangshan/internal/compactidx"
)

func TestWriteRead(t *testing.T) {
	divs := []compactidx.DivRecord{
		{Subtype: 0, BBox: [4]float32{-5, 10, 41, 52}, PolyOffset: 0, PolyLength: 1234},
		{Subtype: 7, BBox: [4]float32{2.2, 2.5, 48.8, 48.95}, PolyOffset: 1234, PolyLength: 5678},
	}
	coarse := map[[2]int16][]uint32{
		{0, 48}: {0, 1},
		{2, 48}: {0, 1},
	}
	fine := map[[2]int16][]uint32{
		{9, 195}: {1},
	}
	preindex := map[[2]int16]uint32{
		{0, 48}: 0,
	}

	var buf bytes.Buffer
	if err := compactidx.Write(&buf, divs, coarse, fine, preindex); err != nil {
		t.Fatalf("Write: %v", err)
	}

	idx, err := compactidx.Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(idx.Records) != 2 {
		t.Fatalf("got %d records, want 2", len(idx.Records))
	}
	if idx.Records[0].Subtype != 0 {
		t.Errorf("Records[0].Subtype = %d, want 0", idx.Records[0].Subtype)
	}
	if idx.Records[1].BBox != [4]float32{2.2, 2.5, 48.8, 48.95} {
		t.Errorf("Records[1].BBox = %v", idx.Records[1].BBox)
	}
	if idx.Records[0].PolyLength != 1234 {
		t.Errorf("Records[0].PolyLength = %d, want 1234", idx.Records[0].PolyLength)
	}
	if len(idx.GridCoarse) != 2 {
		t.Errorf("GridCoarse len = %d, want 2", len(idx.GridCoarse))
	}
	if idx.CountryPreindex[[2]int16{0, 48}] != 0 {
		t.Error("preindex mismatch")
	}
}
