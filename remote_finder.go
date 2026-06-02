package xiangshan

import (
	"bytes"
	"compress/gzip"
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	xs "github.com/ringsaturn/xiangshan/generated/xiangshan/v1"
	"github.com/ringsaturn/xiangshan/internal/compactidx"
	"github.com/ringsaturn/xiangshan/internal/geom"
	"github.com/ringsaturn/xiangshan/internal/grid"
)

// SlabFetcher fetches a byte range from the polygon slab.
type SlabFetcher interface {
	FetchRange(ctx context.Context, offset uint64, length uint32) ([]byte, error)
}

// HTTPRangeFetcher implements SlabFetcher against any HTTP server that supports
// Range requests (S3, R2, GCS signed URLs, plain HTTP file servers).
type HTTPRangeFetcher struct {
	URL    string
	Client *http.Client // nil uses http.DefaultClient
}

func (f *HTTPRangeFetcher) FetchRange(ctx context.Context, offset uint64, length uint32) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+uint64(length)-1))

	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xiangshan: slab fetch returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// RemoteFinder queries geographic divisions using a remote polygon slab.
//
// The compact index (~26 MB raw, ~16 MB gzipped) holds only subtype, bbox,
// and slab offsets per division — no names or IDs. All string metadata is
// read from the slab chunk after polygon containment is confirmed, so name
// and ID resolution always comes from the same fetch that checks the polygon.
//
// This design minimises cold-start index size and in-memory footprint (~43 MB
// for the index structures) at the cost of at most one extra Range request per
// query on the short-circuit path (single unambiguous candidate, interior
// point) and for the country preindex path.
type RemoteFinder struct {
	// per-division parallel arrays
	subtypes    []uint8
	bboxes      [][4]float32 // xmin, xmax, ymin, ymax
	polyOffsets []uint64
	polyLengths []uint32

	gridCoarse      map[[2]int16][]uint32
	gridFine        map[[2]int16][]uint32
	countryPreindex map[[2]int16]uint32

	fetcher SlabFetcher
	cache   *slabLRU
}

// NewRemoteFinder constructs a RemoteFinder from a raw (uncompressed) compact
// index (XSCI format) and a SlabFetcher for the .xs-poly slab.
// cacheSize is the number of slab chunks kept in an LRU cache (0 = disabled).
func NewRemoteFinder(indexBytes []byte, fetcher SlabFetcher, cacheSize int) (*RemoteFinder, error) {
	idx, err := compactidx.Read(bytes.NewReader(indexBytes))
	if err != nil {
		return nil, fmt.Errorf("xiangshan: parse compact index: %w", err)
	}
	return newRemoteFinderFromIndex(idx, fetcher, cacheSize)
}

// NewRemoteFinderFromGzip accepts a gzip-compressed index produced by
// xs-remote-split --compress.
func NewRemoteFinderFromGzip(gzipBytes []byte, fetcher SlabFetcher, cacheSize int) (*RemoteFinder, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gzipBytes))
	if err != nil {
		return nil, fmt.Errorf("xiangshan: decompress index: %w", err)
	}
	raw, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("xiangshan: decompress index: %w", err)
	}
	return NewRemoteFinder(raw, fetcher, cacheSize)
}

func newRemoteFinderFromIndex(idx *compactidx.Index, fetcher SlabFetcher, cacheSize int) (*RemoteFinder, error) {
	if idx.GridCoarse == nil || idx.GridFine == nil {
		return nil, errors.New("xiangshan: grid index missing")
	}

	n := len(idx.Records)
	subtypes := make([]uint8, n)
	bboxes := make([][4]float32, n)
	polyOffsets := make([]uint64, n)
	polyLengths := make([]uint32, n)

	for i, r := range idx.Records {
		subtypes[i] = r.Subtype
		bboxes[i] = r.BBox
		polyOffsets[i] = r.PolyOffset
		polyLengths[i] = r.PolyLength
	}

	var cache *slabLRU
	if cacheSize > 0 {
		cache = newSlabLRU(cacheSize)
	}

	return &RemoteFinder{
		subtypes:        subtypes,
		bboxes:          bboxes,
		polyOffsets:     polyOffsets,
		polyLengths:     polyLengths,
		gridCoarse:      idx.GridCoarse,
		gridFine:        idx.GridFine,
		countryPreindex: idx.CountryPreindex,
		fetcher:         fetcher,
		cache:           cache,
	}, nil
}

// Query returns the geographic result for (lng, lat) using primary names.
func (f *RemoteFinder) Query(ctx context.Context, lng, lat float64) (Result, error) {
	return f.queryWith(ctx, lng, lat, "")
}

// QueryI18n returns the geographic result with names resolved in lang (e.g.
// "zh", "en", "ja"). Falls back to the primary name when unavailable.
func (f *RemoteFinder) QueryI18n(ctx context.Context, lng, lat float64, lang string) (Result, error) {
	return f.queryWith(ctx, lng, lat, lang)
}

func (f *RemoteFinder) queryWith(ctx context.Context, lng, lat float64, lang string) (Result, error) {
	var r Result

	// --- country preindex: fast-path country name from a single Range fetch ---
	coarseKey := grid.CoarseKey(lng, lat)
	if countryIdx, ok := f.countryPreindex[coarseKey]; ok {
		chunk, err := f.fetchOne(ctx, countryIdx)
		if err != nil {
			return Result{}, err
		}
		div := xs.GetSizePrefixedRootAsCompressedDivision(chunk, 0)
		r.Country = nameFromChunk(div, lang)
		r.CountryID = string(div.Id())
	}

	// --- coarse tier ---
	coarseCandidates := f.gridCoarse[coarseKey]

	if len(coarseCandidates) == 1 && canShortCircuit(lng, lat) {
		// Single unambiguous candidate: skip polygon check, fetch for name.
		idx := coarseCandidates[0]
		chunk, err := f.fetchOne(ctx, idx)
		if err != nil {
			return Result{}, err
		}
		div := xs.GetSizePrefixedRootAsCompressedDivision(chunk, 0)
		applyCoarseCandidate(&r, xs.Subtype(f.subtypes[idx]), nameFromChunk(div, lang), string(div.Id()))
	} else {
		// Collect candidates passing bbox pre-filter, then fetch in parallel.
		need := f.bboxCandidates(coarseCandidates, lng, lat, func(idx uint32) bool {
			sub := xs.Subtype(f.subtypes[idx])
			return !(r.Country != "" && (sub == xs.SubtypeCountry || sub == xs.SubtypeDependency))
		})
		fetched, err := f.fetchParallel(ctx, need)
		if err != nil {
			return Result{}, err
		}
		for i, idx := range need {
			if r.Country != "" && r.Region != "" && r.County != "" {
				break
			}
			chunk := fetched[i]
			if chunk == nil {
				continue
			}
			div := xs.GetSizePrefixedRootAsCompressedDivision(chunk, 0)
			if !geom.CompressedDivisionContainsPointFB(div, lng, lat) {
				continue
			}
			applyCoarseCandidate(&r, xs.Subtype(f.subtypes[idx]), nameFromChunk(div, lang), string(div.Id()))
		}
	}

	// --- fine tier ---
	fineCandidates := f.gridFine[grid.FineKey(lng, lat)]

	if len(fineCandidates) == 1 && canShortCircuit(lng, lat) {
		idx := fineCandidates[0]
		chunk, err := f.fetchOne(ctx, idx)
		if err != nil {
			return Result{}, err
		}
		div := xs.GetSizePrefixedRootAsCompressedDivision(chunk, 0)
		applyFineCandidate(&r, xs.Subtype(f.subtypes[idx]), nameFromChunk(div, lang), string(div.Id()))
	} else {
		need := f.bboxCandidates(fineCandidates, lng, lat, nil)
		fetched, err := f.fetchParallel(ctx, need)
		if err != nil {
			return Result{}, err
		}
		for i, idx := range need {
			if r.County != "" && r.LocalAdmin != "" && r.Locality != "" {
				break
			}
			chunk := fetched[i]
			if chunk == nil {
				continue
			}
			div := xs.GetSizePrefixedRootAsCompressedDivision(chunk, 0)
			if !geom.CompressedDivisionContainsPointFB(div, lng, lat) {
				continue
			}
			applyFineCandidate(&r, xs.Subtype(f.subtypes[idx]), nameFromChunk(div, lang), string(div.Id()))
		}
	}

	return r, nil
}

// bboxCandidates filters indices to those whose bbox contains (lng, lat).
// skip is an optional predicate; returning true skips the candidate.
func (f *RemoteFinder) bboxCandidates(indices []uint32, lng, lat float64, skip func(uint32) bool) []uint32 {
	var need []uint32
	for _, idx := range indices {
		if skip != nil && skip(idx) {
			continue
		}
		bb := f.bboxes[idx]
		if geom.BBoxContains(bb[0], bb[1], bb[2], bb[3], lng, lat) {
			need = append(need, idx)
		}
	}
	return need
}

// nameFromChunk returns the division name from a fetched slab chunk,
// resolving i18n if lang is non-empty.
func nameFromChunk(div *xs.CompressedDivision, lang string) string {
	name := string(div.Name())
	if lang != "" {
		name = resolveI18nName(div.NamesCommon(), lang, name)
	}
	return name
}

func resolveI18nName(namesCommon []byte, lang, fallback string) string {
	if len(namesCommon) == 0 {
		return fallback
	}
	var m map[string]string
	if json.Unmarshal(namesCommon, &m) != nil {
		return fallback
	}
	if v := m[lang]; v != "" {
		return v
	}
	return fallback
}

// fetchParallel fetches slab chunks for indices in parallel, result in order.
func (f *RemoteFinder) fetchParallel(ctx context.Context, indices []uint32) ([][]byte, error) {
	if len(indices) == 0 {
		return nil, nil
	}

	results := make([][]byte, len(indices))
	errs := make([]error, len(indices))
	var wg sync.WaitGroup

	for i, idx := range indices {
		if f.cache != nil {
			if data, ok := f.cache.get(idx); ok {
				results[i] = data
				continue
			}
		}
		wg.Add(1)
		go func(i int, idx uint32) {
			defer wg.Done()
			data, err := f.fetcher.FetchRange(ctx, f.polyOffsets[idx], f.polyLengths[idx])
			if err != nil {
				errs[i] = err
				return
			}
			if f.cache != nil {
				f.cache.put(idx, data)
			}
			results[i] = data
		}(i, idx)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (f *RemoteFinder) fetchOne(ctx context.Context, idx uint32) ([]byte, error) {
	if f.cache != nil {
		if data, ok := f.cache.get(idx); ok {
			return data, nil
		}
	}
	data, err := f.fetcher.FetchRange(ctx, f.polyOffsets[idx], f.polyLengths[idx])
	if err != nil {
		return nil, err
	}
	if f.cache != nil {
		f.cache.put(idx, data)
	}
	return data, nil
}

// --- LRU cache for polygon slab chunks ---

type slabLRU struct {
	mu    sync.Mutex
	cap   int
	items map[uint32]*list.Element
	list  *list.List
}

type slabEntry struct {
	key  uint32
	data []byte
}

func newSlabLRU(capacity int) *slabLRU {
	return &slabLRU{
		cap:   capacity,
		items: make(map[uint32]*list.Element, capacity),
		list:  list.New(),
	}
}

func (c *slabLRU) get(key uint32) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.list.MoveToFront(el)
	return el.Value.(*slabEntry).data, true
}

func (c *slabLRU) put(key uint32, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.list.MoveToFront(el)
		el.Value.(*slabEntry).data = data
		return
	}
	el := c.list.PushFront(&slabEntry{key: key, data: data})
	c.items[key] = el
	if c.list.Len() > c.cap {
		if oldest := c.list.Back(); oldest != nil {
			c.list.Remove(oldest)
			delete(c.items, oldest.Value.(*slabEntry).key)
		}
	}
}
