package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
	orbsimplify "github.com/paulmach/orb/simplify"
	"github.com/ringsaturn/xiangshan/internal/flatbuf"
	"github.com/ringsaturn/xiangshan/internal/grid"
	"github.com/ringsaturn/xiangshan/internal/preindex"
	"github.com/ringsaturn/xiangshan/internal/types"
)

type Config struct {
	Input   string
	Output  string
	Version string
	Source  string
}

func main() {
	var cfg Config
	flag.StringVar(&cfg.Input, "input", "build/simplified.parquet", "input parquet path")
	flag.StringVar(&cfg.Output, "output", "build/divisions.bin", "output FlatBuffers path")
	flag.StringVar(&cfg.Version, "version", "", "dataset version")
	flag.StringVar(&cfg.Source, "source", "", "dataset source")
	flag.Parse()
	if err := Run(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// maxLocalityRingPoints caps locality ring point count before FlatBuffers
// encoding. 567K locality entries at full resolution exceed the 2GB builder limit;
// 50 points per ring keeps accuracy adequate for point-in-polygon queries.
const maxLocalityRingPoints = 50

func Run(ctx context.Context, cfg Config) error {
	if cfg.Input == "" || cfg.Output == "" {
		return fmt.Errorf("input and output are required")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil {
		return err
	}
	divs, err := readDivisions(ctx, cfg.Input)
	if err != nil {
		return err
	}
	capLocalityRings(divs, maxLocalityRingPoints)
	coarse := grid.Build(divs, grid.CoarseTierSubtypes, 1.0)
	fine := grid.Build(divs, grid.FineTierSubtypes, 4.0)
	countryPreindex := preindex.BuildCountry(coarse, divs)
	buf, err := flatbuf.EncodeDivisionsWithCountryPreindex(divs, coarse, fine, countryPreindex, cfg.Version, cfg.Source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfg.Output, buf, 0o644); err != nil {
		return err
	}
	fmt.Printf("divisions=%d coarse_cells=%d fine_cells=%d country_preindex_cells=%d bytes=%d\n",
		len(divs), len(coarse), len(fine), len(countryPreindex), len(buf))
	return nil
}

func readDivisions(ctx context.Context, input string) ([]types.Division, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `SELECT area_id, division_id, subtype, admin_level, country, region, name, names_common,
		parent_id, class, wikidata, population, driving_side, local_type,
		xmin, xmax, ymin, ymax, geometry_wkb FROM read_parquet(` + sqlString(input) + `)`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	divs := make([]types.Division, 0, 70000)
	for rows.Next() {
		var row divisionRow
		if err := scanDivisionRow(rows, &row); err != nil {
			return nil, err
		}
		subtype, err := types.SubtypeFromString(row.Subtype)
		if err != nil {
			return nil, err
		}
		geom, err := wkb.Unmarshal(row.GeometryWKB)
		if err != nil {
			return nil, fmt.Errorf("decode WKB for %s: %w", row.AreaID, err)
		}
		polys, err := geometryToPolygons(geom)
		if err != nil {
			return nil, fmt.Errorf("convert geometry for %s: %w", row.AreaID, err)
		}
		pop := int32(0)
		if row.Population.Valid {
			pop = row.Population.Int32
		}
		divs = append(divs, types.Division{
			ID:          row.AreaID,
			DivisionID:  nullString(row.DivisionID),
			Name:        row.Name,
			NamesCommon: nullString(row.NamesCommon),
			Subtype:     subtype,
			AdminLevel:  int8(row.AdminLevel),
			Country:     nullString(row.Country),
			Region:      nullString(row.Region),
			ParentID:    nullString(row.ParentID),
			Class:       nullString(row.Class),
			Wikidata:    nullString(row.Wikidata),
			Population:  pop,
			DrivingSide: nullString(row.DrivingSide),
			LocalType:   nullString(row.LocalType),
			BBox: types.BBox{
				Xmin: float32(row.Xmin),
				Xmax: float32(row.Xmax),
				Ymin: float32(row.Ymin),
				Ymax: float32(row.Ymax),
			},
			Polygons: polys,
		})
	}
	return divs, rows.Err()
}

type divisionRow struct {
	AreaID      string
	DivisionID  sql.NullString
	Subtype     string
	AdminLevel  int32
	Country     sql.NullString
	Region      sql.NullString
	Name        string
	NamesCommon sql.NullString
	ParentID    sql.NullString
	Class       sql.NullString
	Wikidata    sql.NullString
	Population  sql.NullInt32
	DrivingSide sql.NullString
	LocalType   sql.NullString
	Xmin, Xmax  float64
	Ymin, Ymax  float64
	GeometryWKB []byte
}

func scanDivisionRow(rows *sql.Rows, r *divisionRow) error {
	return rows.Scan(
		&r.AreaID,
		&r.DivisionID,
		&r.Subtype,
		&r.AdminLevel,
		&r.Country,
		&r.Region,
		&r.Name,
		&r.NamesCommon,
		&r.ParentID,
		&r.Class,
		&r.Wikidata,
		&r.Population,
		&r.DrivingSide,
		&r.LocalType,
		&r.Xmin,
		&r.Xmax,
		&r.Ymin,
		&r.Ymax,
		&r.GeometryWKB,
	)
}

func geometryToPolygons(g orb.Geometry) ([]types.Polygon, error) {
	switch geom := g.(type) {
	case orb.Polygon:
		return []types.Polygon{polygonToTypes(geom)}, nil
	case orb.MultiPolygon:
		out := make([]types.Polygon, 0, len(geom))
		for _, p := range geom {
			out = append(out, polygonToTypes(p))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported geometry %T", g)
	}
}

func polygonToTypes(p orb.Polygon) types.Polygon {
	var out types.Polygon
	if len(p) == 0 {
		return out
	}
	out.Exterior = types.Ring{Coords: ringToCoords(p[0])}
	for _, h := range p[1:] {
		out.Holes = append(out.Holes, types.Ring{Coords: ringToCoords(h)})
	}
	return out
}

func ringToCoords(r orb.Ring) []float32 {
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

func nullString(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// capLocalityRings applies D-P simplification to any locality ring that exceeds
// maxPoints unique vertices, keeping binary output within the FlatBuffers 2GB limit.
func capLocalityRings(divs []types.Division, maxPoints int) {
	for i := range divs {
		if divs[i].Subtype != types.SubtypeLocality {
			continue
		}
		for j := range divs[i].Polygons {
			divs[i].Polygons[j].Exterior.Coords = capRingCoords(divs[i].Polygons[j].Exterior.Coords, maxPoints)
			for k := range divs[i].Polygons[j].Holes {
				divs[i].Polygons[j].Holes[k].Coords = capRingCoords(divs[i].Polygons[j].Holes[k].Coords, maxPoints)
			}
		}
	}
}

func capRingCoords(coords []float32, maxPoints int) []float32 {
	nPts := len(coords) / 2
	if nPts <= maxPoints {
		return coords
	}
	// Build a closed orb.Ring from the flat float32 coord array.
	ring := make(orb.Ring, nPts+1)
	for i := 0; i < nPts; i++ {
		ring[i] = orb.Point{float64(coords[i*2]), float64(coords[i*2+1])}
	}
	ring[nPts] = ring[0]

	// Increase D-P tolerance until ring fits within maxPoints.
	tolerance := 0.001
	var simplified orb.Ring
	for range 20 {
		simplified = orbsimplify.DouglasPeucker(tolerance).Ring(ring.Clone())
		if len(simplified) <= maxPoints+1 {
			break
		}
		tolerance *= 2
	}
	// Fall back to original if simplification produced a degenerate ring.
	if len(simplified) < 4 {
		return coords
	}
	n := len(simplified)
	if n > 1 && simplified[0] == simplified[n-1] {
		n--
	}
	out := make([]float32, 0, n*2)
	for i := 0; i < n; i++ {
		out = append(out, float32(simplified[i][0]), float32(simplified[i][1]))
	}
	return out
}
