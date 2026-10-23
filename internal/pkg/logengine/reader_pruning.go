package logengine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExtractBloomTokens extracts search tokens for Bloom filter pruning.
// If exactMatch is true, extracts full tokens (tokenbf_v1).
// If exactMatch is false and len(query) >= 4, extracts 4-grams (ngrambf_v1).
func ExtractBloomTokens(query string, exactMatch bool) [][]byte {
	var bloomTokens [][]byte
	qTrim := strings.TrimSpace(query)
	if qTrim == "" {
		return nil
	}

	if exactMatch {
		Tokenize([]byte(qTrim), func(tok []byte) {
			if bytes.IndexByte(tok, '_') != -1 {
				start := 0
				for i := 0; i <= len(tok); i++ {
					if i == len(tok) || tok[i] == '_' {
						if i-start >= 2 {
							tokCopy := make([]byte, i-start)
							copy(tokCopy, tok[start:i])
							bloomTokens = append(bloomTokens, tokCopy)
						}
						start = i + 1
					}
				}
			} else {
				tokCopy := make([]byte, len(tok))
				copy(tokCopy, tok)
				bloomTokens = append(bloomTokens, tokCopy)
			}
		})
		return bloomTokens
	}

	// Substring query (GET /logs?q=...) with len >= 4: extract 4-grams per token (ngrambf_v1)
	Tokenize([]byte(qTrim), func(tok []byte) {
		if len(tok) >= 4 {
			grams := ExtractNGrams4(string(tok))
			bloomTokens = append(bloomTokens, grams...)
		}
	})

	return bloomTokens
}

// PruneBlockByMarks evaluates Stage 1 RAM bitmasks (level, service bloom, stream, K8s namespace).
// Returns true if the block can be safely skipped without any disk I/O.
func PruneBlockByMarks(mark *BlockMark, params QueryParams, targetLevelMask uint16, targetServiceBit uint64) bool {
	// 1. Level bitmask
	if targetLevelMask != 0xFFFF && mark.LevelMask != 0 && (mark.LevelMask&targetLevelMask) == 0 {
		return true
	}

	// 2. Service bloom bitmask
	if targetServiceBit != 0 && mark.ServiceBloom != 0 && (mark.ServiceBloom&targetServiceBit) != targetServiceBit {
		return true
	}

	// 3. Stream bitmask (stored in Reserved)
	if params.Stream != "" && mark.Reserved != 0 {
		sBit := StreamToBit(params.Stream)
		if sBit != 0 && (mark.Reserved&sBit) == 0 {
			return true
		}
	}

	// 4. Namespace bloom bit (stored in Reserved bits 3..15)
	if params.Namespace != "" && mark.Reserved != 0 {
		nsBit := MetaBloomBit(params.Namespace)
		if (mark.Reserved & nsBit) == 0 {
			return true
		}
	}

	return false
}

// PruneBlockByBloom evaluates Stage 2 Bloom filter (tokenbf_v1 / ngrambf_v1).
// Returns true if the block definitely does NOT contain the search tokens.
func PruneBlockByBloom(bloomFile *os.File, bIdx int, bloomTokens [][]byte) bool {
	if len(bloomTokens) == 0 || bloomFile == nil {
		return false
	}

	var bf BlockBloomFilter
	if _, err := bloomFile.ReadAt(bf[:], int64(bIdx)*BloomFilterSize); err != nil {
		return false
	}

	for _, tok := range bloomTokens {
		if !bf.Contains(tok) {
			return true // missing token/ngram -> prune block!
		}
	}
	return false
}

// MatchEntry filters an in-memory Entry against QueryParams.
func MatchEntry(e Entry, params QueryParams, queryLower string, sinceUnix, untilUnix int64) bool {
	ts := e.Timestamp.UnixNano()
	if (sinceUnix > 0 && ts < sinceUnix) || (untilUnix > 0 && ts > untilUnix) {
		return false
	}
	if params.Service != "" && !matchFilter(e.Service, params.Service) {
		return false
	}
	if params.Level != "" && !matchLevel(e.Level, params.Level) {
		return false
	}
	if params.Namespace != "" && !matchFilter(e.Namespace, params.Namespace) {
		return false
	}
	if params.PodName != "" && !matchFilter(e.PodName, params.PodName) {
		return false
	}
	if params.ContainerName != "" && !matchFilter(e.ContainerName, params.ContainerName) {
		return false
	}
	if params.Stream != "" && !strings.EqualFold(e.Stream, params.Stream) {
		return false
	}
	if queryLower != "" && !bytesContainsFold([]byte(e.Message), queryLower) {
		return false
	}
	return true
}

// EntryDedupeKey generates a unique string key for deduplicating entries across live memory and disk.
func EntryDedupeKey(ts time.Time, svc, msg, stream string) string {
	return strings.Join([]string{ts.Format(time.RFC3339Nano), svc, stream, msg}, "\x00")
}

func (r *Reader) scanPartBlocks(pDir string, candidateBlocks []int, bloomTokens [][]byte, view *BlockView, results []Entry, seenKeys map[string]struct{}, limit int, sinceUnix, untilUnix int64, queryLower string, params QueryParams, reverse bool) []Entry {
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

		// 1. Bitmask Pruning (Stage 1 in RAM): skip block if level, service, stream, or ns bitmask doesn't match
		if PruneBlockByMarks(&mark, params, targetLevelMask, targetServiceBit) {
			continue
		}

		// 2. Bloom Filter Pruning (Stage 2: 1KB stack ReadAt)
		if len(bloomTokens) > 0 {
			if !bloomOpened {
				bloomOpened = true
				bloomFile, _ = os.Open(filepath.Join(pDir, "bloom.bin"))
			}
			if PruneBlockByBloom(bloomFile, bIdx, bloomTokens) {
				continue
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
			if params.Namespace != "" || params.PodName != "" || params.ContainerName != "" {
				var ns, pod, cont string
				if r.metaDict != nil {
					if i < len(view.NamespaceIDs) && view.NamespaceIDs[i] > 0 {
						ns, _ = r.metaDict.LookupNamespace(view.NamespaceIDs[i])
					}
					if i < len(view.PodIDs) && view.PodIDs[i] > 0 {
						pod, _ = r.metaDict.LookupPod(view.PodIDs[i])
					}
					if i < len(view.ContainerIDs) && view.ContainerIDs[i] > 0 {
						cont, _ = r.metaDict.LookupContainer(view.ContainerIDs[i])
					}
				}
				if params.Namespace != "" && !matchFilter(ns, params.Namespace) {
					continue
				}
				if params.PodName != "" && !matchFilter(pod, params.PodName) {
					continue
				}
				if params.ContainerName != "" && !matchFilter(cont, params.ContainerName) {
					continue
				}
			}
			if params.Stream != "" {
				var stream string
				if i < len(view.StreamIDs) {
					stream = StreamIDToString(view.StreamIDs[i])
				}
				if !strings.EqualFold(stream, params.Stream) {
					continue
				}
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
			var ns, pod, cont, stream string
			if r.metaDict != nil {
				if i < len(view.NamespaceIDs) && view.NamespaceIDs[i] > 0 {
					ns, _ = r.metaDict.LookupNamespace(view.NamespaceIDs[i])
				}
				if i < len(view.PodIDs) && view.PodIDs[i] > 0 {
					pod, _ = r.metaDict.LookupPod(view.PodIDs[i])
				}
				if i < len(view.ContainerIDs) && view.ContainerIDs[i] > 0 {
					cont, _ = r.metaDict.LookupContainer(view.ContainerIDs[i])
				}
			}
			if i < len(view.StreamIDs) {
				stream = StreamIDToString(view.StreamIDs[i])
			}
			if params.Namespace != "" && !matchFilter(ns, params.Namespace) {
				continue
			}
			if params.PodName != "" && !matchFilter(pod, params.PodName) {
				continue
			}
			if params.ContainerName != "" && !matchFilter(cont, params.ContainerName) {
				continue
			}
			if params.Stream != "" && !strings.EqualFold(stream, params.Stream) {
				continue
			}
			msg := view.MessageAt(i)
			if queryLower != "" && !bytesContainsFold(msg, queryLower) {
				continue
			}
			entryTs := time.Unix(0, ts).UTC()
			key := EntryDedupeKey(entryTs, svc, string(msg), stream)
			if _, seen := seenKeys[key]; seen {
				continue
			}
			seenKeys[key] = struct{}{}
			results = append(results, Entry{
				Timestamp:     entryTs,
				Service:       svc,
				Level:         lvl,
				Message:       string(msg),
				Namespace:     ns,
				PodName:       pod,
				ContainerName: cont,
				Stream:        stream,
			})
		}
	}
	return results
}

