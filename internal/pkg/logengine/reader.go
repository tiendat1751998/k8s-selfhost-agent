package logengine

import (
	"bytes"
	"context"
	"encoding/json"
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

	var partDirs []string
	for _, e := range partEntries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			partDirs = append(partDirs, e.Name())
		}
	}
	sort.Strings(partDirs)

	view := NewBlockView()
	var results []Entry

	for _, pName := range partDirs {
		if ctx.Err() != nil || len(results) >= limit {
			break
		}
		pDir := filepath.Join(r.partsDir, pName)

		// 1. Stage 1: Time-range pruning on primary.idx (with deferred file close)
		pidx, err := loadPrimaryIndex(filepath.Join(pDir, "primary.idx"))
		if err != nil || pidx.Len() == 0 {
			continue
		}

		candidateBlocks := pidx.Search(sinceUnix, untilUnix)
		if len(candidateBlocks) == 0 {
			continue
		}

		// 2. Stage 2: Bloom filter pruning on bloom.bin.
		// Only prune when ExactMatch is enabled to prevent false negatives on substring searches.
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
				candidateBlocks = surviving
			}
		}
		if len(candidateBlocks) == 0 {
			continue
		}

		// 3. Stage 3: Scan candidate blocks with deferred dataFile close
		results = r.scanPartBlocks(pDir, candidateBlocks, view, results, limit, sinceUnix, untilUnix, queryLower, params)
	}
	return results, nil
}

func (r *Reader) scanPartBlocks(pDir string, candidateBlocks []int, view *BlockView, results []Entry, limit int, sinceUnix, untilUnix int64, queryLower string, params QueryParams) []Entry {
	marksData, err := os.ReadFile(filepath.Join(pDir, "marks.bin"))
	if err != nil {
		return results
	}
	dataFile, err := os.Open(filepath.Join(pDir, "data.bin"))
	if err != nil {
		return results
	}
	defer dataFile.Close()

	for _, bIdx := range candidateBlocks {
		if len(results) >= limit {
			break
		}
		markOffset := bIdx * BlockMarkSize
		if markOffset+BlockMarkSize > len(marksData) {
			continue
		}

		var mark BlockMark
		mark.Decode(marksData[markOffset : markOffset+BlockMarkSize])
		compressedBuf := make([]byte, mark.CompressedLength)
		if _, err := dataFile.ReadAt(compressedBuf, int64(mark.CompressedOffset)); err != nil {
			continue
		}

		scratch := GetBlockBuffer()
		uncompressed, err := DecompressBlock(compressedBuf, scratch)
		if err != nil {
			PutBlockBuffer(scratch)
			continue
		}
		if err := view.Unpack(uncompressed); err != nil {
			PutBlockBuffer(scratch)
			continue
		}

		for i := 0; i < view.RowCount && len(results) < limit; i++ {
			ts := view.Timestamps[i]
			if (sinceUnix > 0 && ts < sinceUnix) || (untilUnix > 0 && ts > untilUnix) {
				continue
			}
			svc, _ := r.dict.Lookup(view.ServiceIDs[i])
			if params.Service != "" && !matchFilter(svc, params.Service) {
				continue
			}
			lvl, _ := r.dict.Lookup(view.LevelIDs[i])
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
		PutBlockBuffer(scratch)
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

func (r *Reader) Close() error { return nil }

func matchFilter(actual, pattern string) bool {
	return strings.EqualFold(actual, pattern) || strings.Contains(strings.ToLower(actual), strings.ToLower(pattern))
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
