package xiangshan

import (
	"encoding/json"
	"errors"
	"os"
	"syscall"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/geom"
	"github.com/ringsaturn/xiangshan/internal/grid"
	"github.com/ringsaturn/xiangshan/internal/preindex"
)

type Result struct {
	Country      string
	CountryID    string
	Region       string
	RegionID     string
	County       string
	CountyID     string
	LocalAdmin   string
	LocalAdminID string
	Locality     string
	LocalityID   string
}

type Finder struct {
	f               *os.File
	buf             []byte
	root            *xs.Divisions
	compressedRoot  *xs.CompressedDivisions
	divIndexes      []geom.DivisionIndex // pre-built YStripes indexes; coords stay in mmap
	names           []string
	ids             []string // area IDs (div.Id()), parallel to names
	gridCoarse      map[[2]int16][]uint32
	gridFine        map[[2]int16][]uint32
	countryPreindex map[[2]int16]uint32
}

func NewFinder(path string) (*Finder, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if fi.Size() < 8 {
		_ = f.Close()
		return nil, errors.New("xiangshan: divisions file is too small")
	}
	buf, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !xs.SizePrefixedDivisionsBufferHasIdentifier(buf) {
		if xs.SizePrefixedCompressedDivisionsBufferHasIdentifier(buf) {
			_ = syscall.Munmap(buf)
			_ = f.Close()
			return NewCompressedFinder(path)
		}
		_ = syscall.Munmap(buf)
		_ = f.Close()
		return nil, errors.New("xiangshan: not a valid divisions file")
	}
	root := xs.GetSizePrefixedRootAsDivisions(buf, 0)
	gridCoarse := grid.DecodeFromFlatbuf(root.GridCoarse(nil))
	gridFine := grid.DecodeFromFlatbuf(root.GridFine(nil))
	countryPreindex := preindex.Decode(root.CountryPreindex(nil))
	if gridCoarse == nil || gridFine == nil {
		_ = syscall.Munmap(buf)
		_ = f.Close()
		return nil, errors.New("xiangshan: grid index missing")
	}

	// Pre-build YStripes ring indexes for all divisions.
	// Only Y-range metadata is allocated; X/Y coords stay in the mmap region.
	n := root.ItemsLength()
	geomIndexes := make([]geom.DivisionIndex, n)
	names := make([]string, n)
	ids := make([]string, n)
	var div xs.Division
	for i := range n {
		root.Items(&div, i)
		geomIndexes[i] = geom.NewDivisionIndex(&div)
		names[i] = string(div.Name())
		ids[i] = string(div.Id())
	}

	return &Finder{
		f:               f,
		buf:             buf,
		root:            root,
		divIndexes:      geomIndexes,
		names:           names,
		ids:             ids,
		gridCoarse:      gridCoarse,
		gridFine:        gridFine,
		countryPreindex: countryPreindex,
	}, nil
}

func NewCompressedFinder(path string) (*Finder, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if fi.Size() < 8 {
		_ = f.Close()
		return nil, errors.New("xiangshan: compressed divisions file is too small")
	}
	buf, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !xs.SizePrefixedCompressedDivisionsBufferHasIdentifier(buf) {
		_ = syscall.Munmap(buf)
		_ = f.Close()
		return nil, errors.New("xiangshan: not a valid compressed divisions file")
	}
	root := xs.GetSizePrefixedRootAsCompressedDivisions(buf, 0)
	gridCoarse := grid.DecodeFromFlatbuf(root.GridCoarse(nil))
	gridFine := grid.DecodeFromFlatbuf(root.GridFine(nil))
	countryPreindex := preindex.Decode(root.CountryPreindex(nil))
	if gridCoarse == nil || gridFine == nil {
		_ = syscall.Munmap(buf)
		_ = f.Close()
		return nil, errors.New("xiangshan: grid index missing")
	}

	n := root.ItemsLength()
	names := make([]string, n)
	ids := make([]string, n)
	var div xs.CompressedDivision
	for i := range n {
		root.Items(&div, i)
		names[i] = string(div.Name())
		ids[i] = string(div.Id())
	}

	return &Finder{
		f:               f,
		buf:             buf,
		compressedRoot:  root,
		names:           names,
		ids:             ids,
		gridCoarse:      gridCoarse,
		gridFine:        gridFine,
		countryPreindex: countryPreindex,
	}, nil
}

// nameFunc resolves a display name for a division by its cache index.
type nameFunc func(idx uint32) string

func (f *Finder) Query(lng, lat float64) Result {
	return f.queryWith(lng, lat, func(idx uint32) string { return f.names[idx] })
}

// QueryI18n returns a Result with names resolved in lang (e.g. "zh", "en", "ja").
// Falls back to the primary name when a translation is unavailable.
func (f *Finder) QueryI18n(lng, lat float64, lang string) Result {
	return f.queryWith(lng, lat, f.langResolver(lang))
}

func (f *Finder) langResolver(lang string) nameFunc {
	if f.root != nil {
		return func(idx uint32) string {
			var div xs.Division
			if !f.root.Items(&div, int(idx)) {
				return f.names[idx]
			}
			nc := div.NamesCommon()
			if len(nc) == 0 {
				return f.names[idx]
			}
			var m map[string]string
			if json.Unmarshal(nc, &m) != nil {
				return f.names[idx]
			}
			if v := m[lang]; v != "" {
				return v
			}
			return f.names[idx]
		}
	}
	return func(idx uint32) string {
		var div xs.CompressedDivision
		if !f.compressedRoot.Items(&div, int(idx)) {
			return f.names[idx]
		}
		nc := div.NamesCommon()
		if len(nc) == 0 {
			return f.names[idx]
		}
		var m map[string]string
		if json.Unmarshal(nc, &m) != nil {
			return f.names[idx]
		}
		if v := m[lang]; v != "" {
			return v
		}
		return f.names[idx]
	}
}

func (f *Finder) queryWith(lng, lat float64, name nameFunc) Result {
	if f.compressedRoot != nil {
		return f.queryCompressedWith(lng, lat, name)
	}

	var r Result
	var div xs.Division

	coarseKey := grid.CoarseKey(lng, lat)
	coarseCandidates := f.gridCoarse[coarseKey]
	if idx, ok := f.countryPreindex[coarseKey]; ok && f.root.Items(&div, int(idx)) {
		r.Country = name(idx)
		r.CountryID = f.ids[idx]
	}
	if len(coarseCandidates) == 1 && canShortCircuit(lng, lat) {
		if f.root.Items(&div, int(coarseCandidates[0])) {
			applyCoarseCandidate(&r, div.Subtype(), name(coarseCandidates[0]), f.ids[coarseCandidates[0]])
		}
	} else {
		for _, idx := range coarseCandidates {
			if r.Country != "" && r.Region != "" && r.County != "" {
				break
			}
			if !f.root.Items(&div, int(idx)) {
				continue
			}
			subtype := div.Subtype()
			if r.Country != "" && (subtype == xs.SubtypeCountry || subtype == xs.SubtypeDependency) {
				continue
			}
			if !f.divIndexes[idx].ContainsPoint(&div, lng, lat) {
				continue
			}
			applyCoarseCandidate(&r, subtype, name(idx), f.ids[idx])
		}
	}

	fineCandidates := f.gridFine[grid.FineKey(lng, lat)]
	if len(fineCandidates) == 1 && canShortCircuit(lng, lat) {
		if f.root.Items(&div, int(fineCandidates[0])) {
			applyFineCandidate(&r, div.Subtype(), name(fineCandidates[0]), f.ids[fineCandidates[0]])
		}
	} else {
		for _, idx := range fineCandidates {
			if r.County != "" && r.LocalAdmin != "" && r.Locality != "" {
				break
			}
			if !f.root.Items(&div, int(idx)) || !f.divIndexes[idx].ContainsPoint(&div, lng, lat) {
				continue
			}
			applyFineCandidate(&r, div.Subtype(), name(idx), f.ids[idx])
		}
	}

	return r
}

func (f *Finder) queryCompressedWith(lng, lat float64, name nameFunc) Result {
	var r Result
	var div xs.CompressedDivision

	coarseKey := grid.CoarseKey(lng, lat)
	coarseCandidates := f.gridCoarse[coarseKey]
	if idx, ok := f.countryPreindex[coarseKey]; ok && f.compressedRoot.Items(&div, int(idx)) {
		r.Country = name(idx)
		r.CountryID = f.ids[idx]
	}
	if len(coarseCandidates) == 1 && canShortCircuit(lng, lat) {
		if f.compressedRoot.Items(&div, int(coarseCandidates[0])) {
			applyCoarseCandidate(&r, div.Subtype(), name(coarseCandidates[0]), f.ids[coarseCandidates[0]])
		}
	} else {
		for _, idx := range coarseCandidates {
			if r.Country != "" && r.Region != "" && r.County != "" {
				break
			}
			if !f.compressedRoot.Items(&div, int(idx)) {
				continue
			}
			subtype := div.Subtype()
			if r.Country != "" && (subtype == xs.SubtypeCountry || subtype == xs.SubtypeDependency) {
				continue
			}
			if !geom.CompressedDivisionContainsPointFB(&div, lng, lat) {
				continue
			}
			applyCoarseCandidate(&r, subtype, name(idx), f.ids[idx])
		}
	}

	fineCandidates := f.gridFine[grid.FineKey(lng, lat)]
	if len(fineCandidates) == 1 && canShortCircuit(lng, lat) {
		if f.compressedRoot.Items(&div, int(fineCandidates[0])) {
			applyFineCandidate(&r, div.Subtype(), name(fineCandidates[0]), f.ids[fineCandidates[0]])
		}
	} else {
		for _, idx := range fineCandidates {
			if r.County != "" && r.LocalAdmin != "" && r.Locality != "" {
				break
			}
			if !f.compressedRoot.Items(&div, int(idx)) || !geom.CompressedDivisionContainsPointFB(&div, lng, lat) {
				continue
			}
			applyFineCandidate(&r, div.Subtype(), name(idx), f.ids[idx])
		}
	}

	return r
}

func canShortCircuit(lng, lat float64) bool {
	return lng > -179 && lng < 179 && lat > -89 && lat < 89
}

func applyCoarseCandidate(r *Result, subtype xs.Subtype, name, id string) {
	switch subtype {
	case xs.SubtypeCountry, xs.SubtypeDependency:
		if r.Country == "" {
			r.Country = name
			r.CountryID = id
		}
	case xs.SubtypeMacroRegion, xs.SubtypeRegion:
		if r.Region == "" {
			r.Region = name
			r.RegionID = id
		}
	case xs.SubtypeMacroCounty:
		if r.County == "" {
			r.County = name
			r.CountyID = id
		}
	}
}

func applyFineCandidate(r *Result, subtype xs.Subtype, name, id string) {
	switch subtype {
	case xs.SubtypeCounty:
		if r.County == "" {
			r.County = name
			r.CountyID = id
		}
	case xs.SubtypeLocalAdmin:
		if r.LocalAdmin == "" {
			r.LocalAdmin = name
			r.LocalAdminID = id
		}
	case xs.SubtypeLocality:
		if r.Locality == "" {
			r.Locality = name
			r.LocalityID = id
		}
	}
}

func (f *Finder) Close() error {
	var err error
	if f.buf != nil {
		err = syscall.Munmap(f.buf)
		f.buf = nil
	}
	if f.f != nil {
		if closeErr := f.f.Close(); err == nil {
			err = closeErr
		}
		f.f = nil
	}
	return err
}
