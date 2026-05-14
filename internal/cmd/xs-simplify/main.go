package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/duckdb/duckdb-go/v2"
	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
	xssimplify "github.com/ringsaturn/xiangshan/internal/simplify"
	"github.com/ringsaturn/xiangshan/internal/topology"
)

type Config struct {
	Input      string
	Output     string
	Tolerances map[string]float64
	MinAreas   map[string]float64
}

var defaultMinAreas = map[string]float64{
	"country":     1.0,
	"dependency":  0.5,
	"macroregion": 0.25,
	"region":      0.25,
	"macrocounty": 0.25,
	"county":      1.0,
	"localadmin":  1.0,
	"locality":    0.01,
}

func main() {
	var cfg Config
	var toleranceJSON string
	var minAreaJSON string
	flag.StringVar(&cfg.Input, "input", "build/extracted.parquet", "input parquet path")
	flag.StringVar(&cfg.Output, "output", "build/simplified.parquet", "output parquet path")
	flag.StringVar(&toleranceJSON, "tolerances", "", "JSON object mapping subtype to tolerance")
	flag.StringVar(&minAreaJSON, "min-areas", "", "JSON object mapping subtype to minimum ring bbox area in sq-degrees")
	flag.Parse()

	cfg.Tolerances = cloneFloatMap(xssimplify.DefaultTolerances)
	cfg.MinAreas = cloneFloatMap(defaultMinAreas)
	if toleranceJSON != "" {
		if err := json.Unmarshal([]byte(toleranceJSON), &cfg.Tolerances); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if minAreaJSON != "" {
		if err := json.Unmarshal([]byte(minAreaJSON), &cfg.MinAreas); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if err := Run(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.Input == "" || cfg.Output == "" {
		return fmt.Errorf("input and output are required")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil {
		return err
	}

	rows, err := readAllRows(ctx, cfg.Input)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "read rows=%d\n", len(rows))

	if err := simplifyRows(rows, cfg); err != nil {
		return err
	}

	return writeRows(ctx, cfg.Output, rows)
}

// fullRow holds all columns from extracted.parquet plus decoded topology polygons.
type fullRow struct {
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
	// Polygons is non-nil only for rows that were simplified.
	Polygons []*topology.Polygon
}

func readAllRows(ctx context.Context, input string) ([]fullRow, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	q := `SELECT area_id, division_id, subtype, admin_level, country, region, name, names_common,
		parent_id, class, wikidata, population, driving_side, local_type,
		xmin, xmax, ymin, ymax, geometry_wkb FROM read_parquet(` + sqlString(input) + `)`
	dbRows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	out := make([]fullRow, 0, 70000)
	for dbRows.Next() {
		var r fullRow
		if err := dbRows.Scan(
			&r.AreaID, &r.DivisionID, &r.Subtype, &r.AdminLevel,
			&r.Country, &r.Region, &r.Name, &r.NamesCommon,
			&r.ParentID, &r.Class, &r.Wikidata, &r.Population,
			&r.DrivingSide, &r.LocalType,
			&r.Xmin, &r.Xmax, &r.Ymin, &r.Ymax, &r.GeometryWKB,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, dbRows.Err()
}

func simplifyRows(rows []fullRow, cfg Config) error {
	statsBySubtype := make(map[string]topology.Stats)
	beforePoints := 0
	afterPoints := 0
	processedRows := 0

	for subtype, indices := range groupBySubtype(rows) {
		tol := cfg.Tolerances[subtype]
		if tol <= 0 || len(indices) == 0 {
			continue
		}
		minArea := cfg.MinAreas[subtype]

		candidateIndices, err := decodeSimplifiable(rows, indices, minArea)
		if err != nil {
			return err
		}
		for _, idx := range candidateIndices {
			beforePoints += countPoints(rows[idx].Polygons)
		}
		fmt.Fprintf(os.Stderr, "subtype=%s rows=%d candidates=%d min_area=%g tolerance=%g\n",
			subtype, len(indices), len(candidateIndices), minArea, tol)
		if len(candidateIndices) == 0 {
			continue
		}
		processedRows += len(candidateIndices)

		ds := &topology.Dataset{Version: subtype}
		for _, idx := range candidateIndices {
			ds.Divisions = append(ds.Divisions, &topology.Division{
				Name:     rows[idx].AreaID,
				Polygons: clonePolygons(rows[idx].Polygons),
			})
		}
		out, stats := topology.DoWithOptions(ds, topology.Options{
			Epsilon:     tol,
			MinRingArea: minArea,
		})
		statsBySubtype[subtype] = stats
		for pos, idx := range candidateIndices {
			rows[idx].Polygons = out.Divisions[pos].Polygons
			afterPoints += countPoints(rows[idx].Polygons)
		}
	}

	// Re-encode simplified geometries to WKB.
	for i := range rows {
		if rows[i].Polygons == nil {
			continue
		}
		geom := topologyToGeometry(rows[i].Polygons)
		newWKB, err := wkb.Marshal(geom)
		if err != nil {
			return fmt.Errorf("encode WKB for %s: %w", rows[i].AreaID, err)
		}
		rows[i].GeometryWKB = newWKB
	}

	fmt.Printf("processed_rows=%d points_before=%d points_after=%d\n", processedRows, beforePoints, afterPoints)
	for _, st := range sortedKeys(statsBySubtype) {
		fmt.Printf("subtype=%s\n%s\n", st, statsBySubtype[st].String())
	}
	return nil
}

func writeRows(ctx context.Context, output string, rows []fullRow) error {
	dbPath := output + ".duckdb"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)

	connector, err := duckdb.NewConnector(dbPath, nil)
	if err != nil {
		return err
	}

	db := sql.OpenDB(connector)
	defer db.Close()

	createSQL := `CREATE TABLE simplified (
		area_id VARCHAR, division_id VARCHAR, subtype VARCHAR,
		admin_level INTEGER, country VARCHAR, region VARCHAR,
		name VARCHAR, names_common VARCHAR, parent_id VARCHAR,
		class VARCHAR, wikidata VARCHAR, population INTEGER,
		driving_side VARCHAR, local_type VARCHAR,
		xmin DOUBLE, xmax DOUBLE, ymin DOUBLE, ymax DOUBLE,
		geometry_wkb BLOB
	)`
	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		return err
	}

	rawConn, err := connector.Connect(ctx)
	if err != nil {
		return err
	}
	defer rawConn.Close()

	appender, err := duckdb.NewAppenderFromConn(rawConn, "", "simplified")
	if err != nil {
		return err
	}

	for i := range rows {
		r := &rows[i]
		if err := appender.AppendRow(
			r.AreaID,
			nullStrVal(r.DivisionID),
			r.Subtype,
			r.AdminLevel,
			nullStrVal(r.Country),
			nullStrVal(r.Region),
			r.Name,
			nullStrVal(r.NamesCommon),
			nullStrVal(r.ParentID),
			nullStrVal(r.Class),
			nullStrVal(r.Wikidata),
			nullInt32Val(r.Population),
			nullStrVal(r.DrivingSide),
			nullStrVal(r.LocalType),
			r.Xmin,
			r.Xmax,
			r.Ymin,
			r.Ymax,
			r.GeometryWKB,
		); err != nil {
			_ = appender.Close()
			return fmt.Errorf("append row %s: %w", r.AreaID, err)
		}
	}
	if err := appender.Close(); err != nil {
		return err
	}

	copySQL := `COPY simplified TO ` + sqlString(output) + ` (FORMAT parquet, COMPRESSION zstd)`
	_, err = db.ExecContext(ctx, copySQL)
	return err
}

func decodeSimplifiable(rows []fullRow, indices []int, minArea float64) ([]int, error) {
	out := make([]int, 0, len(indices))
	for _, idx := range indices {
		r := &rows[idx]
		// Cheap bbox area pre-filter before decoding WKB.
		if minArea > 0 && (r.Xmax-r.Xmin)*(r.Ymax-r.Ymin) < minArea {
			continue
		}
		polygons, err := decodePolygons(r.GeometryWKB, r.AreaID)
		if err != nil {
			return nil, err
		}
		if !hasLargeRing(polygons, minArea) {
			continue
		}
		rows[idx].Polygons = polygons
		out = append(out, idx)
	}
	return out, nil
}

func decodePolygons(wkbBytes []byte, id string) ([]*topology.Polygon, error) {
	geom, err := wkb.Unmarshal(wkbBytes)
	if err != nil {
		return nil, fmt.Errorf("decode WKB for %s: %w", id, err)
	}
	return geometryToTopology(geom, id)
}

func geometryToTopology(g orb.Geometry, id string) ([]*topology.Polygon, error) {
	switch geom := g.(type) {
	case orb.Polygon:
		return []*topology.Polygon{orbPolygonToTopology(geom)}, nil
	case orb.MultiPolygon:
		out := make([]*topology.Polygon, 0, len(geom))
		for _, p := range geom {
			out = append(out, orbPolygonToTopology(p))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported geometry %T for %s", g, id)
	}
}

func orbPolygonToTopology(p orb.Polygon) *topology.Polygon {
	out := &topology.Polygon{}
	if len(p) == 0 {
		return out
	}
	out.Points = orbRingToTopology(p[0])
	for _, h := range p[1:] {
		out.Holes = append(out.Holes, &topology.Polygon{Points: orbRingToTopology(h)})
	}
	return out
}

func orbRingToTopology(r orb.Ring) []*topology.Point {
	pts := make([]*topology.Point, 0, len(r))
	for _, p := range r {
		pts = append(pts, &topology.Point{Lng: float32(p.Lon()), Lat: float32(p.Lat())})
	}
	return pts
}

func topologyToGeometry(polygons []*topology.Polygon) orb.Geometry {
	if len(polygons) == 1 {
		return topologyToOrbPolygon(polygons[0])
	}
	out := make(orb.MultiPolygon, 0, len(polygons))
	for _, p := range polygons {
		out = append(out, topologyToOrbPolygon(p))
	}
	return out
}

func topologyToOrbPolygon(p *topology.Polygon) orb.Polygon {
	out := make(orb.Polygon, 0, 1+len(p.Holes))
	out = append(out, topologyToOrbRing(p.Points))
	for _, h := range p.Holes {
		out = append(out, topologyToOrbRing(h.Points))
	}
	return out
}

func topologyToOrbRing(points []*topology.Point) orb.Ring {
	out := make(orb.Ring, 0, len(points))
	for _, p := range points {
		out = append(out, orb.Point{float64(p.Lng), float64(p.Lat)})
	}
	return out
}

func hasLargeRing(polygons []*topology.Polygon, minArea float64) bool {
	if minArea <= 0 {
		return true
	}
	for _, p := range polygons {
		if ringArea(p.Points) >= minArea {
			return true
		}
		for _, h := range p.Holes {
			if ringArea(h.Points) >= minArea {
				return true
			}
		}
	}
	return false
}

func ringArea(points []*topology.Point) float64 {
	if len(points) < 3 {
		return 0
	}
	area := 0.0
	for i := range points {
		next := points[(i+1)%len(points)]
		area += float64(points[i].Lng)*float64(next.Lat) - float64(next.Lng)*float64(points[i].Lat)
	}
	return math.Abs(area / 2)
}

func clonePolygons(in []*topology.Polygon) []*topology.Polygon {
	out := make([]*topology.Polygon, 0, len(in))
	for _, p := range in {
		cp := &topology.Polygon{Points: clonePoints(p.Points)}
		for _, h := range p.Holes {
			cp.Holes = append(cp.Holes, &topology.Polygon{Points: clonePoints(h.Points)})
		}
		out = append(out, cp)
	}
	return out
}

func clonePoints(in []*topology.Point) []*topology.Point {
	out := make([]*topology.Point, 0, len(in))
	for _, p := range in {
		out = append(out, &topology.Point{Lng: p.Lng, Lat: p.Lat})
	}
	return out
}

func countPoints(polygons []*topology.Polygon) int {
	n := 0
	for _, p := range polygons {
		n += len(p.Points)
		for _, h := range p.Holes {
			n += len(h.Points)
		}
	}
	return n
}

func groupBySubtype(rows []fullRow) map[string][]int {
	out := make(map[string][]int)
	for i := range rows {
		out[rows[i].Subtype] = append(out[rows[i].Subtype], i)
	}
	return out
}

func sortedKeys(m map[string]topology.Stats) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func nullStrVal(s sql.NullString) driver.Value {
	if !s.Valid {
		return nil
	}
	return s.String
}

func nullInt32Val(v sql.NullInt32) driver.Value {
	if !v.Valid {
		return nil
	}
	return v.Int32
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
