package logengine

import (
	"bytes"
	"context"
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
	Service    string
	Level      string
	Query      string
	Since      time.Time
	Until      time.Time
	Limit      int
	ExactMatch bool // If true, treats query as exact tokens allowing Bloom filter block pruning.
}

// Reader executes search queries across columnar parts with 3-stage pruning.
type Reader struct {
	mu       sync.RWMutex
	dir      string
	partsDir string
	dict     *LabelDictionary
}

// NewReader initializes a Reader for the given engine directory.
func NewReader(dir string, dict *LabelDictionary) (*Reader, error) {
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
	return &Reader{dir: dir, partsDir: filepath.Join(dir, "parts"), dict: dict}, nil
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

	var queryTokens [][]byte
	if strings.TrimSpace(params.Query) != "" {
		Tokenize([]byte(params.Query), func(tok []byte) { queryTokens = append(queryTokens, tok) })
	}
	queryLower := strings.ToLower(strings.TrimSpace(params.Query))

	partEntries, err := os.ReadDir(r.partsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type partInfo struct {
		id      uint64
		name    string
		dir     string
		minTime int64
		maxTime int64
		pidx    *PrimaryIndex
	}

	var parts []partInfo
	for _, e := range partEntries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			var id uint64
			_, _ = fmt.Sscanf(e.Name(), "part_%d", &id)
			pDir := filepath.Join(r.partsDir, e.Name())
			pidx, err := loadPrimaryIndex(filepath.Join(pDir, "primary.idx"))
			if err != nil || pidx.Len() == 0 {
				continue
			}
			parts = append(parts, partInfo{
				id:      id,
				name:    e.Name(),
				dir:     pDir,
				minTime: pidx.entries[0].MinTime,
				maxTime: pidx.entries[pidx.Len()-1].MaxTime,
				pidx:    pidx,
			})
		}
	}

	// Sort parts chronologically by numeric part ID and min timestamp
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].id != parts[j].id {
			return parts[i].id < parts[j].id
		}
		return parts[i].minTime < parts[j].minTime
	})

	if len(parts) == 0 {
		return nil, nil
	}

	view := NewBlockView()
	var results []Entry

	filterBlocksByBloom := func(pDir string, candidateBlocks []int) []int {
		if params.ExactMatch && len(queryTokens) > 0 {
			if bloomData, err := os.ReadFile(filepath.Join(pDir, "bloom.bin")); err == nil {
				var surviving []int
				var bf BlockBloomFilter
				for _, bIdx := range candidateBlocks {
					start := bIdx * BloomFilterSize
					if start+BloomFilterSize <= len(bloomData) {
						bf.CopyFrom(bloomData[start : start+BloomFilterSize])
						match := true
						for _, tok := range queryTokens {
							if !bf.Contains(tok) {
								match = false
								break
							}
						}
						if match {
							surviving = append(surviving, bIdx)
						}
					}
				}
				return surviving
			}
		}
		return candidateBlocks
	}

	if sinceUnix > 0 {
		// Forward search starting at targeted chunk index matching the requested time window
		startIdx := sort.Search(len(parts), func(i int) bool {
			return parts[i].maxTime >= sinceUnix
		})
		for i := startIdx; i < len(parts); i++ {
			if ctx.Err() != nil || len(results) >= limit {
				break
			}
			if untilUnix > 0 && parts[i].minTime > untilUnix {
				break
			}
			candidates := parts[i].pidx.Search(sinceUnix, untilUnix)
			candidates = filterBlocksByBloom(parts[i].dir, candidates)
			if len(candidates) == 0 {
				continue
			}
			results = r.scanPartBlocks(parts[i].dir, candidates, view, results, limit, sinceUnix, untilUnix, queryLower, params, false)
		}
	} else {
		// No since filter: retrieve the latest entries up to limit starting from newest chunk parts
		for i := len(parts) - 1; i >= 0; i-- {
			if ctx.Err() != nil || len(results) >= limit {
				break
			}
			if untilUnix > 0 && parts[i].minTime > untilUnix {
				continue
			}
			candidates := parts[i].pidx.Search(sinceUnix, untilUnix)
			candidates = filterBlocksByBloom(parts[i].dir, candidates)
			if len(candidates) == 0 {
				continue
			}
			results = r.scanPartBlocks(parts[i].dir, candidates, view, results, limit, sinceUnix, untilUnix, queryLower, params, true)
		}
		// Sort the latest extracted entries chronologically ascending
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].Timestamp.Before(results[j].Timestamp)
		})
	}

	return results, nil
}

func (r *Reader) scanPartBlocks(pDir string, candidateBlocks []int, view *BlockView, results []Entry, limit int, sinceUnix, untilUnix int64, queryLower string, params QueryParams, reverse bool) []Entry {
	marksData, err := os.ReadFile(filepath.Join(pDir, "marks.bin"))
	if err != nil {
		return results
	}
	dataFile, err := os.Open(filepath.Join(pDir, "data.bin"))
	if err != nil {
		return results
	}
	defer dataFile.Close()

	blocks := candidateBlocks
	if reverse {
		blocks = make([]int, len(candidateBlocks))
		for i, b := range candidateBlocks {
			blocks[len(candidateBlocks)-1-i] = b
		}
	}

	targetLevelMask := LevelFilterMask(params.Level)
	var targetServiceBit uint64
	if params.Service != "" {
		if svcID, ok := r.dict.LookupID(params.Service); ok {
			targetServiceBit = ServiceBloomBit(svcID)
		}
	}

	compBuf := GetBlockBuffer()
	defer PutBlockBuffer(compBuf)

	scratch := GetBlockBuffer()
	defer PutBlockBuffer(scratch)

	for _, bIdx := range blocks {
		if len(results) >= limit {
			break
		}
		markOffset := bIdx * BlockMarkSize
		if markOffset+BlockMarkSize > len(marksData) {
			continue
		}

		var mark BlockMark
		mark.Decode(marksData[markOffset : markOffset+BlockMarkSize])

		// 1. Bitmask Pruning: skip block if level bitmask does not intersect
		if targetLevelMask != 0xFFFF && mark.LevelMask != 0 && (mark.LevelMask&targetLevelMask) == 0 {
			continue // zero disk read, zero decompression!
		}

		// 2. ServiceBloom Pruning: skip block if service bloom hash mask does not contain target bit
		if targetServiceBit != 0 && mark.ServiceBloom != 0 && (mark.ServiceBloom&targetServiceBit) != targetServiceBit {
			continue // zero disk read, zero decompression!
		}

		readLen := int(mark.CompressedLength)
		if readLen > cap(compBuf) {
			compBuf = make([]byte, readLen)
		}
		compressedSlice := compBuf[:readLen]
		if _, err := dataFile.ReadAt(compressedSlice, int64(mark.CompressedOffset)); err != nil {
			continue
		}

		// 3. Late Materialization: unpack ONLY metadata columns first
		if err := view.UnpackMeta(compressedSlice, scratch); err != nil {
			continue
		}

		start, end, step := 0, view.RowCount, 1
		if reverse {
			start, end, step = view.RowCount-1, -1, -1
		}

		// First pass: check if any row matches timestamp, service, and level filters
		hasMatchingRow := false
		for i := start; i != end; i += step {
			ts := view.Timestamps[i]
			if (sinceUnix > 0 && ts < sinceUnix) || (untilUnix > 0 && ts > untilUnix) {
				continue
			}
			svc, _ := r.dict.Lookup(view.ServiceIDs[i])
			if params.Service != "" && !matchFilter(svc, params.Service) {
				continue
			}
			lvl := LevelIDToString(view.LevelIDs[i])
			if params.Level != "" && !matchLevel(lvl, params.Level) {
				continue
			}
			hasMatchingRow = true
			break
		}

		if !hasMatchingRow {
			continue // LATE MATERIALIZATION: Zero message decompression!
		}

		// Row matched! Decompress messages frame now
		if err := view.EnsureMessagesDecompressed(scratch); err != nil {
			continue
		}

		// Extract matched entries
		for i := start; i != end && len(results) < limit; i += step {
			ts := view.Timestamps[i]
			if (sinceUnix > 0 && ts < sinceUnix) || (untilUnix > 0 && ts > untilUnix) {
				continue
			}
			svc, _ := r.dict.Lookup(view.ServiceIDs[i])
			if params.Service != "" && !matchFilter(svc, params.Service) {
				continue
			}
			lvl := LevelIDToString(view.LevelIDs[i])
			if params.Level != "" && !matchLevel(lvl, params.Level) {
				continue
			}
			msg := view.MessageAt(i)
			if queryLower != "" && !bytesContainsFold(msg, queryLower) {
				continue
			}
			results = append(results, Entry{
				Timestamp: time.Unix(0, ts).UTC(),
				Service:   svc,
				Level:     lvl,
				Message:   string(msg),
			})
		}
	}
	return results
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

// BlockCount returns total indexed blocks across all parts.
func (r *Reader) BlockCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	partEntries, err := os.ReadDir(r.partsDir)
	if err != nil {
		return 0
	}
	total := 0
	for _, e := range partEntries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			pidx, err := loadPrimaryIndex(filepath.Join(r.partsDir, e.Name(), "primary.idx"))
			if err == nil {
				total += pidx.Len()
			}
		}
	}
	return total
}

func (r *Reader) Close() error { return nil }

func matchFilter(actual, pattern string) bool {
	if strings.EqualFold(actual, pattern) {
		return true
	}
	if len(pattern) <= 3 {
		return false
	}
	actLower := strings.ToLower(actual)
	patLower := strings.ToLower(pattern)
	if strings.HasPrefix(actLower, patLower) {
		if len(actLower) > len(patLower) && isBoundary(actLower[len(patLower)]) {
			return true
		}
	}
	if strings.HasSuffix(actLower, patLower) {
		if len(actLower) > len(patLower) && isBoundary(actLower[len(actLower)-len(patLower)-1]) {
			return true
		}
	}
	return false
}

func isBoundary(b byte) bool {
	return b == '-' || b == '_' || b == '.' || b == '/' || b == ':' || (b < 'a' || b > 'z') && (b < '0' || b > '9')
}

func matchLevel(actual, filter string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return true
	}
	actual = strings.TrimSpace(strings.ToLower(actual))
	switch filter {
	case "error", "err", "fatal", "critical":
		return actual == "error" || actual == "fatal" || actual == "critical"
	case "warn", "warning":
		return actual == "error" || actual == "fatal" || actual == "critical" || actual == "warn"
	case "info":
		return actual == "error" || actual == "warn" || actual == "info"
	case "debug", "trace":
		return true
	default:
		return actual == filter || strings.Contains(actual, filter)
	}
}

func bytesContainsFold(b []byte, subLower string) bool {
	if len(subLower) == 0 {
		return true
	}
	if len(b) < len(subLower) {
		return false
	}
	if bytes.Contains(b, []byte(subLower)) {
		return true
	}
	subLen := len(subLower)
	maxI := len(b) - subLen
	for i := 0; i <= maxI; i++ {
		match := true
		for j := 0; j < subLen; j++ {
			c := b[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			if c != subLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
