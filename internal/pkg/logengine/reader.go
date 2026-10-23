package logengine

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// QueryParams specifies search filters for the columnar log engine.
type QueryParams struct {
	Service       string
	Level         string
	Query         string
	Since         time.Time
	Until         time.Time
	Limit         int
	ExactMatch    bool // If true, treats query as exact tokens allowing Bloom filter block pruning.
	Namespace     string
	PodName       string
	ContainerName string
	Stream        string
}

// ReaderOption configures the Reader.
type ReaderOption func(*Reader)

// WithLiveWriter configures Reader with a live Writer for real-time 0ms visibility.
func WithLiveWriter(w *Writer) ReaderOption {
	return func(r *Reader) {
		r.liveWriter = w
		if w != nil && w.MetaDictionary() != nil {
			r.metaDict = w.MetaDictionary()
		}
	}
}

// Reader executes search queries across columnar parts with 3-stage pruning.
type Reader struct {
	mu                 sync.RWMutex
	dir                string
	partsDir           string
	dict               *LabelDictionary
	metaDict           *MetaDictionary
	liveWriter         *Writer
	cachedBlockCount   int
	blockCountCachedAt time.Time
}

// SetLiveWriter updates the live Writer source for real-time snapshot querying.
func (r *Reader) SetLiveWriter(w *Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.liveWriter = w
	if w != nil && w.MetaDictionary() != nil {
		r.metaDict = w.MetaDictionary()
	}
}

// SetLiveSource is an alias for SetLiveWriter.
func (r *Reader) SetLiveSource(w *Writer) {
	r.SetLiveWriter(w)
}

// NewReader initializes a Reader for the given engine directory.
func NewReader(dir string, dict *LabelDictionary, opts ...ReaderOption) (*Reader, error) {
	if dict == nil {
		dict = NewLabelDictionary()
		dictPath := filepath.Join(dir, "dict.json")
		if data, err := os.ReadFile(dictPath); err == nil {
			var entries []string
			if err := json.Unmarshal(data, &entries); err == nil {
				dict.Import(entries)
			}
		}
	}

	metaDict := NewMetaDictionary()
	_ = metaDict.Load(filepath.Join(dir, "meta_dict.json"))

	r := &Reader{
		dir:      dir,
		partsDir: filepath.Join(dir, "parts"),
		dict:     dict,
		metaDict: metaDict,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r, nil
}

func readPartTimeBounds(pDir string) (minTime, maxTime int64, ok bool) {
	f, err := os.Open(filepath.Join(pDir, "primary.idx"))
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.Size() < 16 {
		return 0, 0, false
	}
	sz := fi.Size()

	var buf [16]byte
	if _, err := f.ReadAt(buf[0:8], 0); err != nil {
		return 0, 0, false
	}
	minTime = int64(binary.LittleEndian.Uint64(buf[0:8]))

	if _, err := f.ReadAt(buf[8:16], sz-8); err != nil {
		return 0, 0, false
	}
	maxTime = int64(binary.LittleEndian.Uint64(buf[8:16]))

	return minTime, maxTime, true
}

// Search executes query with 3-stage pruning (time index, bloom filter if exact match, decompression).
func (r *Reader) Search(ctx context.Context, params QueryParams) ([]Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	var sinceUnix, untilUnix int64
	if !params.Since.IsZero() {
		sinceUnix = params.Since.UnixNano()
	}
	if !params.Until.IsZero() {
		untilUnix = params.Until.UnixNano()
	}

	bloomTokens := ExtractBloomTokens(params.Query, params.ExactMatch)
	queryLower := strings.ToLower(strings.TrimSpace(params.Query))

	if r.liveWriter != nil && r.liveWriter.MetaDictionary() != nil {
		r.metaDict = r.liveWriter.MetaDictionary()
	} else if r.metaDict == nil || r.metaDict.Size() <= 1 {
		if r.metaDict == nil {
			r.metaDict = NewMetaDictionary()
		}
		_ = r.metaDict.Load(filepath.Join(r.dir, "meta_dict.json"))
	}

	var results []Entry
	seenKeys := make(map[string]struct{})

	// 1. Hot In-Memory Buffer Snapshot (MemTable / Real-Time 0ms Visibility)
	if r.liveWriter != nil {
		liveEntries := r.liveWriter.SnapshotUncommitted()
		for i := len(liveEntries) - 1; i >= 0; i-- {
			e := liveEntries[i]
			if MatchEntry(e, params, queryLower, sinceUnix, untilUnix) {
				key := EntryDedupeKey(e.Timestamp, e.Service, e.Message, e.Stream)
				if _, seen := seenKeys[key]; !seen {
					seenKeys[key] = struct{}{}
					results = append(results, e)
					if len(results) >= limit {
						break
					}
				}
			}
		}
	}

	if len(results) >= limit {
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Timestamp.Before(results[j].Timestamp)
		})
		return results, nil
	}

	partEntries, err := os.ReadDir(r.partsDir)
	if err != nil {
		if os.IsNotExist(err) {
			if len(results) > 0 {
				sort.SliceStable(results, func(i, j int) bool {
					return results[i].Timestamp.Before(results[j].Timestamp)
				})
				return results, nil
			}
			return nil, nil
		}
		return nil, err
	}

	type partMeta struct {
		id   uint64
		name string
		dir  string
	}

	var parts []partMeta
	for _, e := range partEntries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			var id uint64
			_, _ = fmt.Sscanf(e.Name(), "part_%d", &id)
			parts = append(parts, partMeta{
				id:   id,
				name: e.Name(),
				dir:  filepath.Join(r.partsDir, e.Name()),
			})
		}
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].id < parts[j].id
	})

	if len(parts) == 0 {
		if len(results) > 0 {
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].Timestamp.Before(results[j].Timestamp)
			})
			return results, nil
		}
		return nil, nil
	}

	view := NewBlockView()

	startIdx := 0
	if sinceUnix > 0 && len(parts) > 8 {
		startIdx = sort.Search(len(parts), func(i int) bool {
			_, maxT, ok := readPartTimeBounds(parts[i].dir)
			return ok && maxT >= sinceUnix
		})
		if startIdx > 0 {
			startIdx--
		}
	}

	endIdx := len(parts) - 1
	if untilUnix > 0 && len(parts) > 8 {
		idx := sort.Search(len(parts), func(i int) bool {
			minT, _, ok := readPartTimeBounds(parts[i].dir)
			return ok && minT > untilUnix
		})
		if idx < len(parts) {
			endIdx = idx
		}
	}
	if endIdx >= len(parts) {
		endIdx = len(parts) - 1
	}

	for i := endIdx; i >= startIdx; i-- {
		if ctx.Err() != nil || len(results) >= limit {
			break
		}
		pidx, err := loadPrimaryIndex(filepath.Join(parts[i].dir, "primary.idx"))
		if err != nil || pidx.Len() == 0 {
			continue
		}
		if sinceUnix > 0 && pidx.entries[pidx.Len()-1].MaxTime < sinceUnix {
			if i < endIdx {
				break
			}
			continue
		}
		if untilUnix > 0 && pidx.entries[0].MinTime > untilUnix {
			continue
		}
		candidates := pidx.Search(sinceUnix, untilUnix)
		if len(candidates) == 0 {
			continue
		}
		results = r.scanPartBlocks(parts[i].dir, candidates, bloomTokens, view, results, seenKeys, limit, sinceUnix, untilUnix, queryLower, params, true)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}



func loadPrimaryIndex(path string) (*PrimaryIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	pidx := NewPrimaryIndex()
	if err := pidx.ReadPrimaryIndex(f); err != nil {
		return nil, err
	}
	return pidx, nil
}

// BlockCount returns total indexed blocks across all parts (cached with 5s TTL).
func (r *Reader) BlockCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.blockCountCachedAt.IsZero() && time.Since(r.blockCountCachedAt) < 5*time.Second {
		return r.cachedBlockCount
	}

	partEntries, err := os.ReadDir(r.partsDir)
	if err != nil {
		r.cachedBlockCount = 0
		r.blockCountCachedAt = time.Now()
		return 0
	}

	total := 0
	for _, e := range partEntries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			fi, err := os.Stat(filepath.Join(r.partsDir, e.Name(), "primary.idx"))
			if err == nil {
				total += int(fi.Size() / 16)
			}
		}
	}

	r.cachedBlockCount = total
	r.blockCountCachedAt = time.Now()
	return total
}

// HasBlocks returns true quickly if there are any indexed blocks.
func (r *Reader) HasBlocks() bool {
	r.mu.RLock()
	if r.cachedBlockCount > 0 && time.Since(r.blockCountCachedAt) < 5*time.Second {
		r.mu.RUnlock()
		return true
	}
	r.mu.RUnlock()

	return r.BlockCount() > 0
}

func (r *Reader) Close() error { return nil }

