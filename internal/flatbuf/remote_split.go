package flatbuf

import (
	"bytes"
	"fmt"

	flatbuffers "github.com/google/flatbuffers/go"
	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/compactidx"
	"github.com/ringsaturn/xiangshan/internal/grid"
	"github.com/ringsaturn/xiangshan/internal/preindex"
)

// SplitResult holds the two output files produced by SplitCompressedDivisions.
type SplitResult struct {
	// Index is the compact binary index (XSCI format, load with compactidx.Read).
	Index []byte
	// Slab is the polygon slab: concatenated size-prefixed CompressedDivision
	// flatbufs, one per division in index order, accessed via HTTP Range requests.
	Slab []byte
}

// SplitCompressedDivisions reads a CompressedDivisions flatbuffer and splits it
// into a compact index (XSCI) and a polygon slab.
func SplitCompressedDivisions(root *xs.CompressedDivisions) (*SplitResult, error) {
	n := root.ItemsLength()

	var slab bytes.Buffer
	slab.Grow(400 * 1024 * 1024)

	divRecs := make([]compactidx.DivRecord, n)
	var div xs.CompressedDivision
	for i := range n {
		if !root.Items(&div, i) {
			return nil, fmt.Errorf("read division %d", i)
		}
		chunk, err := encodeSlabDivision(&div)
		if err != nil {
			return nil, fmt.Errorf("encode slab division %d: %w", i, err)
		}

		var fbBBox xs.BBox
		bbox := div.Bbox(&fbBBox)
		if bbox == nil {
			return nil, fmt.Errorf("division %d has no bbox", i)
		}

		divRecs[i] = compactidx.DivRecord{
			Subtype:    uint8(div.Subtype()),
			BBox:       [4]float32{bbox.Xmin(), bbox.Xmax(), bbox.Ymin(), bbox.Ymax()},
			PolyOffset: uint64(slab.Len()),
			PolyLength: uint32(len(chunk)),
		}
		slab.Write(chunk)
	}

	coarse := grid.DecodeFromFlatbuf(root.GridCoarse(nil))
	fine := grid.DecodeFromFlatbuf(root.GridFine(nil))
	cpre := preindex.Decode(root.CountryPreindex(nil))

	var indexBuf bytes.Buffer
	indexBuf.Grow(80 * 1024 * 1024)
	if err := compactidx.Write(&indexBuf, divRecs, coarse, fine, cpre); err != nil {
		return nil, fmt.Errorf("write compact index: %w", err)
	}

	return &SplitResult{Index: indexBuf.Bytes(), Slab: slab.Bytes()}, nil
}

// encodeSlabDivision re-encodes a single CompressedDivision as a standalone
// size-prefixed flatbuffer containing id, subtype, bbox, names_common, and
// polygons. names_common is included here so QueryI18n can resolve it after a
// polygon hit without needing it in the index.
func encodeSlabDivision(div *xs.CompressedDivision) ([]byte, error) {
	b := flatbuffers.NewBuilder(8192)

	// name and id are the only metadata needed at query time; everything else
	// stays in the index or is not needed for point-in-polygon.
	nameOff := b.CreateString(string(div.Name()))
	idOff := b.CreateString(string(div.Id()))
	namesCommonOff := optionalString(b, string(div.NamesCommon()))

	nPoly := div.PolygonsLength()
	polyOffs := make([]flatbuffers.UOffsetT, nPoly)
	var poly xs.CompressedPolygon
	for i := range nPoly {
		if !div.Polygons(&poly, i) {
			return nil, fmt.Errorf("read polygon %d", i)
		}
		polyOffs[i] = copyCompressedPolygon(b, &poly)
	}
	xs.CompressedDivisionStartPolygonsVector(b, nPoly)
	for i := nPoly - 1; i >= 0; i-- {
		b.PrependUOffsetT(polyOffs[i])
	}
	polygonsOff := b.EndVector(nPoly)

	var fbBBox xs.BBox
	bbox := div.Bbox(&fbBBox)
	if bbox == nil {
		return nil, fmt.Errorf("missing bbox")
	}

	xs.CompressedDivisionStart(b)
	xs.CompressedDivisionAddId(b, idOff)
	xs.CompressedDivisionAddName(b, nameOff)
	xs.CompressedDivisionAddSubtype(b, div.Subtype())
	if namesCommonOff != 0 {
		xs.CompressedDivisionAddNamesCommon(b, namesCommonOff)
	}
	xs.CompressedDivisionAddPolygons(b, polygonsOff)
	bboxOff := xs.CreateBBox(b, bbox.Xmin(), bbox.Xmax(), bbox.Ymin(), bbox.Ymax())
	xs.CompressedDivisionAddBbox(b, bboxOff)
	root := xs.CompressedDivisionEnd(b)
	xs.FinishSizePrefixedCompressedDivisionBuffer(b, root)
	return b.FinishedBytes(), nil
}

func copyCompressedPolygon(b *flatbuffers.Builder, poly *xs.CompressedPolygon) flatbuffers.UOffsetT {
	var ext xs.CompressedRing
	extOff := copyCompressedRing(b, poly.Exterior(&ext))

	nHoles := poly.HolesLength()
	var holesOff flatbuffers.UOffsetT
	if nHoles > 0 {
		holeOffs := make([]flatbuffers.UOffsetT, nHoles)
		var hole xs.CompressedRing
		for i := range nHoles {
			if poly.Holes(&hole, i) {
				holeOffs[i] = copyCompressedRing(b, &hole)
			}
		}
		xs.CompressedPolygonStartHolesVector(b, nHoles)
		for i := nHoles - 1; i >= 0; i-- {
			b.PrependUOffsetT(holeOffs[i])
		}
		holesOff = b.EndVector(nHoles)
	}

	xs.CompressedPolygonStart(b)
	xs.CompressedPolygonAddExterior(b, extOff)
	if holesOff != 0 {
		xs.CompressedPolygonAddHoles(b, holesOff)
	}
	return xs.CompressedPolygonEnd(b)
}

func copyCompressedRing(b *flatbuffers.Builder, r *xs.CompressedRing) flatbuffers.UOffsetT {
	if r == nil {
		return 0
	}
	data := r.DataBytes()
	pointCount := r.PointCount()

	xs.CompressedRingStartDataVector(b, len(data))
	for i := len(data) - 1; i >= 0; i-- {
		b.PrependByte(data[i])
	}
	dataOff := b.EndVector(len(data))

	xs.CompressedRingStart(b)
	xs.CompressedRingAddData(b, dataOff)
	xs.CompressedRingAddPointCount(b, pointCount)
	return xs.CompressedRingEnd(b)
}
