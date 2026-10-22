package logengine

import (
	"bytes"
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
	mu                 sync.RWMutex
	dir                string
	partsDir           string
	dict               *LabelDictionary
	cachedBlockCount   int
	blockCountCachedAt time.Time
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

	var bloomTokens [][]byte
	if params.ExactMatch && strings.TrimSpace(params.Query) != "" {
		Tokenize([]byte(params.Query), func(tok []byte) {
			if bytes.IndexByte(tok, '_') != -1 {
				start := 0
				for i := 0; i <= len(tok); i++ {
					if i == len(tok) || tok[i] == '_' {
						if i-start >= 2 {
							bloomTokens = append(bloomTokens, tok[start:i])
						}
						start = i + 1
					}
				}
			} else {
				bloomTokens = append(bloomTokens, tok)
			}
		})
	} else if !params.ExactMatch && params.Query != "" {
		q := []byte(params.Query)
		n := len(q)
		i := 0
		for i < n {
			for i < n && !isAlphaNum(q[i]) {
				i++
			}
			if i >= n {
				break
			}
			start := i
			for i < n && isAlphaNum(q[i]) {
				i++
			}
			end := i
			if start > 0 && end < n && !isAlphaNum(q[start-1]) && !isAlphaNum(q[end]) {
				tok := q[start:end]
				if len(tok) >= 2 {
					bloomTokens = append(bloomTokens, tok)
				}
			}
		}
	}
	queryLower := strings.ToLower(strings.TrimSpace(params.Query))

	partEntries, err := os.ReadDir(r.partsDir)
	if err != nil {
		if os.IsNotExist(err) {
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
		return nil, nil
	}

	view := NewBlockView()
	var results []Entry

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
		results = r.scanPartBlocks(parts[i].dir, candidates, bloomTokens, view, results, limit, sinceUnix, untilUnix, queryLower, params, true)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}

func (r *Reader) scanPartBlocks(pDir string, candidateBlocks []int, bloomTokens [][]byte, view *BlockView, results []Entry, limit int, sinceUnix, untilUnix int64, queryLower string, params QueryParams, reverse bool) []Entry {
	marksData, err := os.ReadFile(filepath.Join(pDir, "marks.bin"))
	if err != nil {
		return results
	}

	var bloomFile *os.File
	var bloomOpened bool
	defer func() {
		if bloomFile != nil {
			_ = bloomFile.Close()
		}
	}()

	var dataFile *os.File
	var dataOpened bool
	defer func() {
		if dataFile != nil {
			_ = dataFile.Close()
		}
	}()

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

		// 1. Bitmask Pruning (Stage 1 in RAM): skip block if level bitmask or service bloom doesn't match
		if targetLevelMask != 0xFFFF && mark.LevelMask != 0 && (mark.LevelMask&targetLevelMask) == 0 {
			continue // 0 extra syscalls, 0 disk reads
		}
		if targetServiceBit != 0 && mark.ServiceBloom != 0 && (mark.ServiceBloom&targetServiceBit) != targetServiceBit {
			continue // 0 extra syscalls, 0 disk reads
		}

		// 2. Bloom Filter Pruning (Stage 2: 1KB stack ReadAt)
		if len(bloomTokens) > 0 {
			if !bloomOpened {
				bloomOpened = true
				bloomFile, _ = os.Open(filepath.Join(pDir, "bloom.bin"))
			}
			if bloomFile != nil {
				var bf BlockBloomFilter
				if _, err := bloomFile.ReadAt(bf[:], int64(bIdx)*BloomFilterSize); err == nil {
					hasAll := true
					for _, tok := range bloomTokens {
						if !bf.Contains(tok) {
							hasAll = false
							break
						}
					}
					if !hasAll {
						continue // missing token in block bloom filter; skip block!
					}
				}
			}
		}

		// 3. Late Materialization & Decompression (Stage 3: lazy data.bin open)
		if !dataOpened {
			dataOpened = true
			dataFile, err = os.Open(filepath.Join(pDir, "data.bin"))
			if err != nil || dataFile == nil {
				return results
			}
		}

		readLen := int(mark.CompressedLength)
		if readLen > cap(compBuf) {
			compBuf = make([]byte, readLen)
		}
		compressedSlice := compBuf[:readLen]
		if _, err := dataFile.ReadAt(compressedSlice, int64(mark.CompressedOffset)); err != nil {
			continue
		}

		if err := view.UnpackMeta(compressedSlice, scratch); err != nil {
			continue
		}

		start, end, step := 0, view.RowCount, 1
		if reverse {
			start, end, step = view.RowCount-1, -1, -1
		}

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
			continue // zero message decompression!
		}

		if err := view.EnsureMessagesDecompressed(scratch); err != nil {
			continue
		}

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

