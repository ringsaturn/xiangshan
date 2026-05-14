package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
	xssimplify "github.com/ringsaturn/xiangshan/internal/simplify"
	"github.com/ringsaturn/xiangshan/internal/topology"
)

type Config struct {
	Input      string
	Base       string
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
}

func main() {
	var cfg Config
	var toleranceJSON string
	var minAreaJSON string
	flag.StringVar(&cfg.Input, "input", "build/extracted.parquet", "input parquet path")
	flag.StringVar(&cfg.Base, "base", "", "base parquet copied to output before topology updates")
	flag.StringVar(&cfg.Output, "output", "build/topo-simplified.parquet", "output parquet path")
	flag.StringVar(&toleranceJSON, "tolerances", "", "JSON object mapping subtype to tolerance")
	flag.StringVar(&minAreaJSON, "min-areas", "", "JSON object mapping subtype to minimum ring area in square degrees")
	flag.Parse()

	cfg.Tolerances = cloneTolerances(xssimplify.DefaultTolerances)
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
	if cfg.Base == "" {
		cfg.Base = cfg.Input
	}
	if cfg.Tolerances == nil {
		cfg.Tolerances = xssimplify.DefaultTolerances
	}
	if cfg.MinAreas == nil {
		cfg.MinAreas = defaultMinAreas
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Output), 0o755); err != nil {
		return err
	}

	rows, err := readCandidateRows(ctx, cfg)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "read rough_candidate_rows=%d\n", len(rows))

	processedRows := 0
	beforePoints := 0
	afterPoints := 0
	statsBySubtype := make(map[string]topology.Stats)
	for subtype, indices := range groupBySubtype(rows) {
		tol := cfg.Tolerances[subtype]
		if tol <= 0 || len(indices) == 0 {
			continue
		}
		minArea := cfg.MinAreas[subtype]
		roughCandidateIndices := indices
		candidateIndices, err := decodeSimplifiableRows(rows, roughCandidateIndices, minArea)
		if err != nil {
			return err
		}
		for _, idx := range candidateIndices {
			beforePoints += countTopologyPoints(rows[idx].Polygons)
		}
		fmt.Fprintf(os.Stderr, "subtype=%s rows=%d rough_candidates=%d candidates=%d min_area=%g tolerance=%g\n",
			subtype, len(indices), len(roughCandidateIndices), len(candidateIndices), minArea, tol)
		if len(candidateIndices) == 0 {
			continue
		}
		processedRows += len(candidateIndices)
		ds := &topology.Dataset{Version: subtype}
		for _, idx := range candidateIndices {
			ds.Divisions = append(ds.Divisions, &topology.Division{
				Name:     rows[idx].AreaID,
				Polygons: cloneTopologyPolygons(rows[idx].Polygons),
			})
		}
		out, stats := topology.DoWithOptions(ds, topology.Options{
			Epsilon:     tol,
			MinRingArea: minArea,
		})
		if err := topology.ValidateWithOptions(out, topology.ReductionValidateOptions()); err != nil {
			return fmt.Errorf("validate topology output for subtype %s: %w", subtype, err)
		}
		statsBySubtype[subtype] = stats
		for pos, idx := range candidateIndices {
			rows[idx].Polygons = out.Divisions[pos].Polygons
			afterPoints += countTopologyPoints(rows[idx].Polygons)
		}
		fmt.Fprintf(os.Stderr, "subtype=%s done\n", subtype)
	}

	for i := range rows {
		if rows[i].Polygons == nil {
			continue
		}
		geom := topologyToGeometry(rows[i].Polygons)
		outWKB, err := wkb.Marshal(geom)
		if err != nil {
			return fmt.Errorf("encode WKB for %s: %w", rows[i].AreaID, err)
		}
		rows[i].GeometryWKB = outWKB
	}

	if err := writeRows(ctx, cfg.Base, cfg.Output, rows); err != nil {
		return err
	}

	fmt.Printf("rows=%d processed_rows=%d points_before=%d points_after=%d\n", len(rows), processedRows, beforePoints, afterPoints)
	for _, subtype := range sortedSubtypeKeys(statsBySubtype) {
		fmt.Printf("subtype=%s\n%s\n", subtype, statsBySubtype[subtype].String())
	}
	return nil
}

func readCandidateRows(ctx context.Context, cfg Config) ([]divisionRow, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `SELECT area_id, division_id, subtype, admin_level, country, region, name, parent_id,
		xmin, xmax, ymin, ymax, geometry_wkb FROM read_parquet(` + sqlString(cfg.Input) + `)
		WHERE ` + candidateWhere(cfg)
	dbRows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	out := make([]divisionRow, 0, 70000)
	for dbRows.Next() {
		var row divisionRow
		if err := scanDivisionRow(dbRows, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, dbRows.Err()
}

func candidateWhere(cfg Config) string {
	parts := make([]string, 0, len(cfg.MinAreas))
	for subtype, minArea := range cfg.MinAreas {
		if cfg.Tolerances[subtype] <= 0 {
			continue
		}
		part := "subtype = " + sqlString(subtype)
		if minArea > 0 {
			part = "(" + part + " AND (xmax - xmin) * (ymax - ymin) >= " + floatSQL(minArea) + ")"
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return "FALSE"
	}
	return strings.Join(parts, " OR ")
}

func floatSQL(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func writeRows(ctx context.Context, input string, output string, rows []divisionRow) error {
	dbPath := output + ".duckdb"
	_ = os.Remove(dbPath)
	defer os.Remove(dbPath)
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	create := `CREATE TABLE simplified AS SELECT * FROM read_parquet(` + sqlString(input) + `)`
	if _, err := db.ExecContext(ctx, create); err != nil {
		return err
	}
	updateSQL := `UPDATE simplified SET geometry_wkb = ? WHERE area_id = ?`
	stmt, err := db.PrepareContext(ctx, updateSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range rows {
		if rows[i].Polygons == nil {
			continue
		}
		if _, err := stmt.ExecContext(ctx, rows[i].GeometryWKB, rows[i].AreaID); err != nil {
			return err
		}
	}
	copySQL := `COPY simplified TO ` + sqlString(output) + ` (FORMAT parquet, COMPRESSION zstd)`
	_, err = db.ExecContext(ctx, copySQL)
	return err
}

type divisionRow struct {
	AreaID      string
	DivisionID  sql.NullString
	Subtype     string
	AdminLevel  int32
	Country     sql.NullString
	Region      sql.NullString
	Name        string
	ParentID    sql.NullString
	Xmin, Xmax  float64
	Ymin, Ymax  float64
	GeometryWKB []byte
	Polygons    []*topology.Polygon
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
		&r.ParentID,
		&r.Xmin,
		&r.Xmax,
		&r.Ymin,
		&r.Ymax,
		&r.GeometryWKB,
	)
}

func geometryToTopology(g orb.Geometry) ([]*topology.Polygon, error) {
	switch geom := g.(type) {
	case orb.Polygon:
		return []*topology.Polygon{polygonToTopology(geom)}, nil
	case orb.MultiPolygon:
		out := make([]*topology.Polygon, 0, len(geom))
		for _, p := range geom {
			out = append(out, polygonToTopology(p))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported geometry %T", g)
	}
}

func polygonToTopology(p orb.Polygon) *topology.Polygon {
	out := &topology.Polygon{}
	if len(p) == 0 {
		return out
	}
	out.Points = ringToTopology(p[0])
	for _, h := range p[1:] {
		out.Holes = append(out.Holes, &topology.Polygon{Points: ringToTopology(h)})
	}
	return out
}

func ringToTopology(r orb.Ring) []*topology.Point {
	points := make([]*topology.Point, 0, len(r))
	for _, p := range r {
		points = append(points, &topology.Point{Lng: float32(p.Lon()), Lat: float32(p.Lat())})
	}
	return points
}

func topologyToGeometry(polygons []*topology.Polygon) orb.Geometry {
	if len(polygons) == 1 {
		return topologyToPolygon(polygons[0])
	}
	out := make(orb.MultiPolygon, 0, len(polygons))
	for _, p := range polygons {
		out = append(out, topologyToPolygon(p))
	}
	return out
}

func topologyToPolygon(p *topology.Polygon) orb.Polygon {
	out := make(orb.Polygon, 0, 1+len(p.Holes))
	out = append(out, topologyToRing(p.Points))
	for _, h := range p.Holes {
		out = append(out, topologyToRing(h.Points))
	}
	return out
}

func topologyToRing(points []*topology.Point) orb.Ring {
	out := make(orb.Ring, 0, len(points))
	for _, p := range points {
		out = append(out, orb.Point{float64(p.Lng), float64(p.Lat)})
	}
	return out
}

func groupBySubtype(rows []divisionRow) map[string][]int {
	out := make(map[string][]int)
	for i := range rows {
		out[rows[i].Subtype] = append(out[rows[i].Subtype], i)
	}
	return out
}

func decodeSimplifiableRows(rows []divisionRow, indices []int, minArea float64) ([]int, error) {
	out := make([]int, 0, len(indices))
	for _, idx := range indices {
		polygons, err := decodeTopologyPolygons(rows[idx])
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

func decodeTopologyPolygons(row divisionRow) ([]*topology.Polygon, error) {
	geom, err := wkb.Unmarshal(row.GeometryWKB)
	if err != nil {
		return nil, fmt.Errorf("decode WKB for %s: %w", row.AreaID, err)
	}
	polygons, err := geometryToTopology(geom)
	if err != nil {
		return nil, fmt.Errorf("convert geometry for %s: %w", row.AreaID, err)
	}
	return polygons, nil
}

func hasLargeRing(polygons []*topology.Polygon, minArea float64) bool {
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

func sortedSubtypeKeys(m map[string]topology.Stats) []string {
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

func cloneTopologyPolygons(in []*topology.Polygon) []*topology.Polygon {
	out := make([]*topology.Polygon, 0, len(in))
	for _, p := range in {
		cp := &topology.Polygon{Points: cloneTopologyPoints(p.Points)}
		for _, h := range p.Holes {
			cp.Holes = append(cp.Holes, &topology.Polygon{Points: cloneTopologyPoints(h.Points)})
		}
		out = append(out, cp)
	}
	return out
}

func cloneTopologyPoints(in []*topology.Point) []*topology.Point {
	out := make([]*topology.Point, 0, len(in))
	for _, p := range in {
		out = append(out, &topology.Point{Lng: p.Lng, Lat: p.Lat})
	}
	return out
}

func countTopologyPoints(polygons []*topology.Polygon) int {
	n := 0
	for _, p := range polygons {
		n += len(p.Points)
		for _, h := range p.Holes {
			n += len(h.Points)
		}
	}
	return n
}

func cloneTolerances(in map[string]float64) map[string]float64 {
	return cloneFloatMap(in)
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
