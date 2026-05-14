package topology

type Dataset struct {
	Version   string
	Divisions []*Division
}

type Division struct {
	Name     string
	Polygons []*Polygon
}

type Polygon struct {
	Points []*Point
	Holes  []*Polygon
}

type Point struct {
	Lng float32
	Lat float32
}
