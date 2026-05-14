package flatbuf

import (
	"encoding/binary"
	"fmt"
	"math"

	flatbuffers "github.com/google/flatbuffers/go"
	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/grid"
	"github.com/ringsaturn/xiangshan/internal/preindex"
)

const CoordScale = 100000.0

type CompressionStats struct {
	Divisions       int
	Rings           int
	Points          int
	CoordinateBytes int
}

func EncodeCompressedDivisionsFromXSFB(root *xs.Divisions) ([]byte, CompressionStats, error) {
	if root == nil {
		return nil, CompressionStats{}, fmt.Errorf("nil divisions root")
	}

	coarse := grid.DecodeFromFlatbuf(root.GridCoarse(nil))
	fine := grid.DecodeFromFlatbuf(root.GridFine(nil))
	if coarse == nil || fine == nil {
		return nil, CompressionStats{}, fmt.Errorf("grid index missing")
	}

	b := flatbuffers.NewBuilder(80 * 1024 * 1024)
	version := string(root.Version())
	source := string(root.Source())
	versionOff := b.CreateString(version)
	sourceOff := b.CreateString(source)
	coarseOff := encodeGridIndex(b, coarse, version)
	fineOff := encodeGridIndex(b, fine, version)
	preindexOff := encodePreindex(b, preindex.Decode(root.CountryPreindex(nil)), version)

	stats := CompressionStats{Divisions: root.ItemsLength()}
	divOffsets := make([]flatbuffers.UOffsetT, root.ItemsLength())
	var div xs.Division
	for i := 0; i < root.ItemsLength(); i++ {
		if !root.Items(&div, i) {
			return nil, CompressionStats{}, fmt.Errorf("read division %d", i)
		}
		off, err := encodeCompressedDivision(b, &div, &stats)
		if err != nil {
			return nil, CompressionStats{}, fmt.Errorf("encode compressed division %d: %w", i, err)
		}
		divOffsets[i] = off
	}

	xs.CompressedDivisionsStartItemsVector(b, len(divOffsets))
	for i := len(divOffsets) - 1; i >= 0; i-- {
		b.PrependUOffsetT(divOffsets[i])
	}
	itemsOff := b.EndVector(len(divOffsets))

	xs.CompressedDivisionsStart(b)
	xs.CompressedDivisionsAddItems(b, itemsOff)
	xs.CompressedDivisionsAddGridCoarse(b, coarseOff)
	xs.CompressedDivisionsAddGridFine(b, fineOff)
	if preindexOff != 0 {
		xs.CompressedDivisionsAddCountryPreindex(b, preindexOff)
	}
	xs.CompressedDivisionsAddVersion(b, versionOff)
	xs.CompressedDivisionsAddSource(b, sourceOff)
	out := xs.CompressedDivisionsEnd(b)
	xs.FinishSizePrefixedCompressedDivisionsBuffer(b, out)
	return b.FinishedBytes(), stats, nil
}

func encodeCompressedDivision(b *flatbuffers.Builder, d *xs.Division, stats *CompressionStats) (flatbuffers.UOffsetT, error) {
	if len(d.Id()) == 0 {
		return 0, fmt.Errorf("missing id")
	}
	if len(d.Name()) == 0 {
		return 0, fmt.Errorf("missing name")
	}
	if d.PolygonsLength() == 0 {
		return 0, fmt.Errorf("missing polygons")
	}

	idOff := b.CreateString(string(d.Id()))
	divisionIDOff := optionalString(b, string(d.DivisionId()))
	nameOff := b.CreateString(string(d.Name()))
	namesCommonOff := optionalString(b, string(d.NamesCommon()))
	countryOff := optionalString(b, string(d.Country()))
	regionOff := optionalString(b, string(d.Region()))
	parentIDOff := optionalString(b, string(d.ParentId()))
	classOff := optionalString(b, string(d.Class()))
	wikidataOff := optionalString(b, string(d.Wikidata()))
	drivingSideOff := optionalString(b, string(d.DrivingSide()))
	localTypeOff := optionalString(b, string(d.LocalType()))

	polyOffsets := make([]flatbuffers.UOffsetT, d.PolygonsLength())
	var poly xs.Polygon
	for i := 0; i < d.PolygonsLength(); i++ {
		if !d.Polygons(&poly, i) {
			return 0, fmt.Errorf("read polygon %d", i)
		}
		polyOffsets[i] = encodeCompressedPolygon(b, &poly, stats)
	}
	xs.CompressedDivisionStartPolygonsVector(b, len(polyOffsets))
	for i := len(polyOffsets) - 1; i >= 0; i-- {
		b.PrependUOffsetT(polyOffsets[i])
	}
	polygonsOff := b.EndVector(len(polyOffsets))

	var fbBBox xs.BBox
	bbox := d.Bbox(&fbBBox)
	if bbox == nil {
		return 0, fmt.Errorf("missing bbox")
	}

	xs.CompressedDivisionStart(b)
	xs.CompressedDivisionAddId(b, idOff)
	if divisionIDOff != 0 {
		xs.CompressedDivisionAddDivisionId(b, divisionIDOff)
	}
	xs.CompressedDivisionAddName(b, nameOff)
	if namesCommonOff != 0 {
		xs.CompressedDivisionAddNamesCommon(b, namesCommonOff)
	}
	xs.CompressedDivisionAddSubtype(b, d.Subtype())
	xs.CompressedDivisionAddAdminLevel(b, d.AdminLevel())
	if countryOff != 0 {
		xs.CompressedDivisionAddCountry(b, countryOff)
	}
	if regionOff != 0 {
		xs.CompressedDivisionAddRegion(b, regionOff)
	}
	if parentIDOff != 0 {
		xs.CompressedDivisionAddParentId(b, parentIDOff)
	}
	if classOff != 0 {
		xs.CompressedDivisionAddClass(b, classOff)
	}
	if wikidataOff != 0 {
		xs.CompressedDivisionAddWikidata(b, wikidataOff)
	}
	xs.CompressedDivisionAddPopulation(b, d.Population())
	if drivingSideOff != 0 {
		xs.CompressedDivisionAddDrivingSide(b, drivingSideOff)
	}
	if localTypeOff != 0 {
		xs.CompressedDivisionAddLocalType(b, localTypeOff)
	}
	bboxOff := xs.CreateBBox(b, bbox.Xmin(), bbox.Xmax(), bbox.Ymin(), bbox.Ymax())
	xs.CompressedDivisionAddBbox(b, bboxOff)
	xs.CompressedDivisionAddPolygons(b, polygonsOff)
	return xs.CompressedDivisionEnd(b), nil
}

func encodeCompressedPolygon(b *flatbuffers.Builder, p *xs.Polygon, stats *CompressionStats) flatbuffers.UOffsetT {
	var ext xs.Ring
	extOff := encodeCompressedRing(b, p.Exterior(&ext), stats)
	holeOffsets := make([]flatbuffers.UOffsetT, p.HolesLength())
	var hole xs.Ring
	for i := 0; i < p.HolesLength(); i++ {
		if p.Holes(&hole, i) {
			holeOffsets[i] = encodeCompressedRing(b, &hole, stats)
		}
	}
	var holesOff flatbuffers.UOffsetT
	if len(holeOffsets) > 0 {
		xs.CompressedPolygonStartHolesVector(b, len(holeOffsets))
		for i := len(holeOffsets) - 1; i >= 0; i-- {
			b.PrependUOffsetT(holeOffsets[i])
		}
		holesOff = b.EndVector(len(holeOffsets))
	}
	xs.CompressedPolygonStart(b)
	xs.CompressedPolygonAddExterior(b, extOff)
	if holesOff != 0 {
		xs.CompressedPolygonAddHoles(b, holesOff)
	}
	return xs.CompressedPolygonEnd(b)
}

func encodeCompressedRing(b *flatbuffers.Builder, r *xs.Ring, stats *CompressionStats) flatbuffers.UOffsetT {
	data := EncodeCompressedRing(r)
	pointCount := uint32(0)
	if r != nil {
		pointCount = uint32(r.CoordsLength() / 2)
	}
	stats.Rings++
	stats.Points += int(pointCount)
	stats.CoordinateBytes += len(data)

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

func EncodeCompressedRing(r *xs.Ring) []byte {
	if r == nil {
		return nil
	}
	n := r.CoordsLength() / 2
	out := make([]byte, 0, n*4)
	var scratch [binary.MaxVarintLen32]byte
	var prevLng, prevLat int32
	for i := 0; i < n; i++ {
		lng := scaleCoord(r.Coords(i * 2))
		lat := scaleCoord(r.Coords(i*2 + 1))
		out = appendVarint32(out, &scratch, lng-prevLng)
		out = appendVarint32(out, &scratch, lat-prevLat)
		prevLng = lng
		prevLat = lat
	}
	return out
}

func appendVarint32(out []byte, scratch *[binary.MaxVarintLen32]byte, v int32) []byte {
	u := uint32(v<<1) ^ uint32(v>>31)
	n := binary.PutUvarint(scratch[:], uint64(u))
	return append(out, scratch[:n]...)
}

func scaleCoord(v float32) int32 {
	return int32(math.Round(float64(v) * CoordScale))
}
