package types

type BBox struct {
	Xmin, Xmax float32
	Ymin, Ymax float32
}

func (b BBox) Contains(lng, lat float64) bool {
	return float64(b.Xmin) <= lng && lng <= float64(b.Xmax) &&
		float64(b.Ymin) <= lat && lat <= float64(b.Ymax)
}

type Ring struct {
	Coords []float32
}

type Polygon struct {
	Exterior Ring
	Holes    []Ring
}

type Division struct {
	ID          string
	DivisionID  string
	Name        string
	NamesCommon string // JSON-encoded map of lang code → localized name
	Subtype     Subtype
	AdminLevel  int8
	Country     string
	Region      string
	ParentID    string
	Class       string // "land" or "maritime"
	Wikidata    string // Wikidata QID, e.g. "Q148"
	Population  int32  // 0 means unknown
	DrivingSide string // "left", "right", or ""
	LocalType   string // English local admin type, e.g. "province", "district"
	BBox        BBox
	Polygons    []Polygon
}
