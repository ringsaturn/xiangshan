package flatbuf

import (
	"fmt"
	"sort"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/paulmach/orb"
	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/preindex"
	"github.com/ringsaturn/xiangshan/internal/types"
)

func EncodeDivisions(
	divs []types.Division,
	coarse map[[2]int16][]uint32,
	fine map[[2]int16][]uint32,
	version string,
	source string,
) ([]byte, error) {
	return EncodeDivisionsWithCountryPreindex(
		divs,
		coarse,
		fine,
		preindex.BuildCountry(coarse, divs),
		version,
		source,
	)
}

func EncodeDivisionsWithCountryPreindex(
	divs []types.Division,
	coarse map[[2]int16][]uint32,
	fine map[[2]int16][]uint32,
	countryPreindex map[[2]int16]uint32,
	version string,
	source string,
) ([]byte, error) {
	b := flatbuffers.NewBuilder(160 * 1024 * 1024)

	versionOff := b.CreateString(version)
	sourceOff := b.CreateString(source)
	coarseOff := encodeGridIndex(b, coarse, version)
	fineOff := encodeGridIndex(b, fine, version)
	preindexOff := encodePreindex(b, countryPreindex, version)

	divOffsets := make([]flatbuffers.UOffsetT, len(divs))
	for i := range divs {
		off, err := encodeDivision(b, divs[i])
		if err != nil {
			return nil, fmt.Errorf("encode division %d: %w", i, err)
		}
		divOffsets[i] = off
	}
	xs.DivisionsStartItemsVector(b, len(divOffsets))
	for i := len(divOffsets) - 1; i >= 0; i-- {
		b.PrependUOffsetT(divOffsets[i])
	}
	itemsOff := b.EndVector(len(divOffsets))

	xs.DivisionsStart(b)
	xs.DivisionsAddItems(b, itemsOff)
	xs.DivisionsAddGridCoarse(b, coarseOff)
	xs.DivisionsAddGridFine(b, fineOff)
	xs.DivisionsAddVersion(b, versionOff)
	xs.DivisionsAddSource(b, sourceOff)
	if preindexOff != 0 {
		xs.DivisionsAddCountryPreindex(b, preindexOff)
	}
	root := xs.DivisionsEnd(b)
	xs.FinishSizePrefixedDivisionsBuffer(b, root)
	return b.FinishedBytes(), nil
}

func EncodeRing(r orb.Ring) []float32 {
	if len(r) == 0 {
		return nil
	}
	n := len(r)
	if n > 1 && r[0] == r[n-1] {
		n--
	}
	coords := make([]float32, 0, n*2)
	for i := 0; i < n; i++ {
		coords = append(coords, float32(r[i].Lon()), float32(r[i].Lat()))
	}
	return coords
}

func EncodePolygon(b *flatbuffers.Builder, p orb.Polygon) flatbuffers.UOffsetT {
	poly := types.Polygon{}
	if len(p) > 0 {
		poly.Exterior = types.Ring{Coords: EncodeRing(p[0])}
		for _, h := range p[1:] {
			poly.Holes = append(poly.Holes, types.Ring{Coords: EncodeRing(h)})
		}
	}
	return encodePolygon(b, poly)
}

func encodeDivision(b *flatbuffers.Builder, d types.Division) (flatbuffers.UOffsetT, error) {
	if d.ID == "" {
		return 0, fmt.Errorf("missing id")
	}
	if d.Name == "" {
		return 0, fmt.Errorf("missing name")
	}
	if len(d.Polygons) == 0 {
		return 0, fmt.Errorf("missing polygons")
	}
	idOff := b.CreateString(d.ID)
	divisionIDOff := optionalString(b, d.DivisionID)
	nameOff := b.CreateString(d.Name)
	namesCommonOff := optionalString(b, d.NamesCommon)
	countryOff := optionalString(b, d.Country)
	regionOff := optionalString(b, d.Region)
	parentIDOff := optionalString(b, d.ParentID)
	classOff := optionalString(b, d.Class)
	wikidataOff := optionalString(b, d.Wikidata)
	drivingSideOff := optionalString(b, d.DrivingSide)
	localTypeOff := optionalString(b, d.LocalType)

	polyOffsets := make([]flatbuffers.UOffsetT, len(d.Polygons))
	for i := range d.Polygons {
		polyOffsets[i] = encodePolygon(b, d.Polygons[i])
	}
	xs.DivisionStartPolygonsVector(b, len(polyOffsets))
	for i := len(polyOffsets) - 1; i >= 0; i-- {
		b.PrependUOffsetT(polyOffsets[i])
	}
	polygonsOff := b.EndVector(len(polyOffsets))

	xs.DivisionStart(b)
	xs.DivisionAddId(b, idOff)
	if divisionIDOff != 0 {
		xs.DivisionAddDivisionId(b, divisionIDOff)
	}
	xs.DivisionAddName(b, nameOff)
	if namesCommonOff != 0 {
		xs.DivisionAddNamesCommon(b, namesCommonOff)
	}
	xs.DivisionAddSubtype(b, xs.Subtype(d.Subtype))
	xs.DivisionAddAdminLevel(b, d.AdminLevel)
	if countryOff != 0 {
		xs.DivisionAddCountry(b, countryOff)
	}
	if regionOff != 0 {
		xs.DivisionAddRegion(b, regionOff)
	}
	if parentIDOff != 0 {
		xs.DivisionAddParentId(b, parentIDOff)
	}
	if classOff != 0 {
		xs.DivisionAddClass(b, classOff)
	}
	if wikidataOff != 0 {
		xs.DivisionAddWikidata(b, wikidataOff)
	}
	xs.DivisionAddPopulation(b, d.Population)
	if drivingSideOff != 0 {
		xs.DivisionAddDrivingSide(b, drivingSideOff)
	}
	if localTypeOff != 0 {
		xs.DivisionAddLocalType(b, localTypeOff)
	}
	bboxOff := xs.CreateBBox(b, d.BBox.Xmin, d.BBox.Xmax, d.BBox.Ymin, d.BBox.Ymax)
	xs.DivisionAddBbox(b, bboxOff)
	xs.DivisionAddPolygons(b, polygonsOff)
	return xs.DivisionEnd(b), nil
}

func encodePolygon(b *flatbuffers.Builder, p types.Polygon) flatbuffers.UOffsetT {
	extOff := encodeCoordsRing(b, p.Exterior)
	holeOffsets := make([]flatbuffers.UOffsetT, len(p.Holes))
	for i := range p.Holes {
		holeOffsets[i] = encodeCoordsRing(b, p.Holes[i])
	}
	var holesOff flatbuffers.UOffsetT
	if len(holeOffsets) > 0 {
		xs.PolygonStartHolesVector(b, len(holeOffsets))
		for i := len(holeOffsets) - 1; i >= 0; i-- {
			b.PrependUOffsetT(holeOffsets[i])
		}
		holesOff = b.EndVector(len(holeOffsets))
	}
	xs.PolygonStart(b)
	xs.PolygonAddExterior(b, extOff)
	if holesOff != 0 {
		xs.PolygonAddHoles(b, holesOff)
	}
	return xs.PolygonEnd(b)
}

func encodeCoordsRing(b *flatbuffers.Builder, r types.Ring) flatbuffers.UOffsetT {
	xs.RingStartCoordsVector(b, len(r.Coords))
	for i := len(r.Coords) - 1; i >= 0; i-- {
		b.PrependFloat32(r.Coords[i])
	}
	coordsOff := b.EndVector(len(r.Coords))
	xs.RingStart(b)
	xs.RingAddCoords(b, coordsOff)
	return xs.RingEnd(b)
}

func encodeGridIndex(b *flatbuffers.Builder, cells map[[2]int16][]uint32, version string) flatbuffers.UOffsetT {
	keys := make([][2]int16, 0, len(cells))
	for k := range cells {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] == keys[j][0] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})

	cellOffsets := make([]flatbuffers.UOffsetT, len(keys))
	for i, k := range keys {
		indices := cells[k]
		xs.GridCellStartIndicesVector(b, len(indices))
		for j := len(indices) - 1; j >= 0; j-- {
			b.PrependUint32(indices[j])
		}
		indicesOff := b.EndVector(len(indices))
		xs.GridCellStart(b)
		xs.GridCellAddLng(b, k[0])
		xs.GridCellAddLat(b, k[1])
		xs.GridCellAddIndices(b, indicesOff)
		cellOffsets[i] = xs.GridCellEnd(b)
	}

	xs.GridIndexStartCellsVector(b, len(cellOffsets))
	for i := len(cellOffsets) - 1; i >= 0; i-- {
		b.PrependUOffsetT(cellOffsets[i])
	}
	cellsOff := b.EndVector(len(cellOffsets))
	versionOff := b.CreateString(version)
	xs.GridIndexStart(b)
	xs.GridIndexAddCells(b, cellsOff)
	xs.GridIndexAddVersion(b, versionOff)
	return xs.GridIndexEnd(b)
}

func encodePreindex(b *flatbuffers.Builder, cells map[[2]int16]uint32, version string) flatbuffers.UOffsetT {
	if len(cells) == 0 {
		return 0
	}
	keys := make([][2]int16, 0, len(cells))
	for k := range cells {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] == keys[j][0] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})

	cellOffsets := make([]flatbuffers.UOffsetT, len(keys))
	for i, k := range keys {
		xs.PreindexCellStart(b)
		xs.PreindexCellAddLng(b, k[0])
		xs.PreindexCellAddLat(b, k[1])
		xs.PreindexCellAddIndex(b, cells[k])
		cellOffsets[i] = xs.PreindexCellEnd(b)
	}

	xs.PreindexStartCellsVector(b, len(cellOffsets))
	for i := len(cellOffsets) - 1; i >= 0; i-- {
		b.PrependUOffsetT(cellOffsets[i])
	}
	cellsOff := b.EndVector(len(cellOffsets))
	versionOff := b.CreateString(version)
	xs.PreindexStart(b)
	xs.PreindexAddCells(b, cellsOff)
	xs.PreindexAddVersion(b, versionOff)
	return xs.PreindexEnd(b)
}

func optionalString(b *flatbuffers.Builder, s string) flatbuffers.UOffsetT {
	if s == "" {
		return 0
	}
	return b.CreateString(s)
}
