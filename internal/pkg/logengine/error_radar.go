package logengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrorGroup aggregates similar error messages under a normalized signature.
type ErrorGroup struct {
	Service       string    `json:"service"`
	Level         string    `json:"level"`
	Fingerprint   string    `json:"fingerprint"`
	SampleMessage string    `json:"sample_message"`
	Count         int       `json:"count"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}

// FingerprintError normalizes variable tokens (numbers, IPs, hex, IDs) to '*' to group recurring errors.
func FingerprintError(msg []byte) string {
	var buf strings.Builder
	buf.Grow(len(msg))
	n := len(msg)
	for i := 0; i < n; {
		c := msg[i]
		if isAlphaNum(c) {
			start, hasDigit := i, false
			for i < n && isAlphaNum(msg[i]) {
				if msg[i] >= '0' && msg[i] <= '9' { hasDigit = true }
				i++
			}
			if hasDigit { buf.WriteByte('*') } else { buf.Write(msg[start:i]) }
		} else {
			buf.WriteByte(c)
			i++
		}
	}
	return buf.String()
}

// IsNoiseService returns true if the service name represents a systemd slice, scope, user session, or empty marker.
func IsNoiseService(svc string) bool {
	lower := strings.ToLower(strings.TrimSpace(svc))
	if lower == "" || lower == "-- no entries --" || lower == "init.scope" {
		return true
	}
	if strings.HasSuffix(lower, ".scope") || strings.HasSuffix(lower, ".slice") {
		return true
	}
	if strings.HasPrefix(lower, "user@") || strings.HasPrefix(lower, "user-runtime-dir@") {
		return true
	}
	return false
}

// SummarizeErrors discovers all unique error groups across the parts in the specified time window without keyword search.
func (r *Reader) SummarizeErrors(ctx context.Context, params QueryParams) ([]ErrorGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sinceUnix, untilUnix int64
	if !params.Since.IsZero() { sinceUnix = params.Since.UnixNano() }
	if !params.Until.IsZero() { untilUnix = params.Until.UnixNano() }

	targetLevelMask := LevelBitError | LevelBitFatal
	if params.Level != "" { targetLevelMask = LevelFilterMask(params.Level) }

	var targetServiceBit uint64
	var targetServiceID uint16
	var hasTargetService bool
	if params.Service != "" {
		if id, ok := r.dict.LookupID(params.Service); ok {
			targetServiceID, hasTargetService, targetServiceBit = id, true, ServiceBloomBit(id)
		}
	}

	partEntries, err := os.ReadDir(r.partsDir)
	if err != nil {
		if os.IsNotExist(err) { return nil, nil }
		return nil, err
	}

	type partMeta struct { id uint64; dir string }
	var parts []partMeta
	for _, e := range partEntries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "part_") {
			var id uint64
			_, _ = fmt.Sscanf(e.Name(), "part_%d", &id)
			parts = append(parts, partMeta{id: id, dir: filepath.Join(r.partsDir, e.Name())})
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].id < parts[j].id })
	if len(parts) == 0 { return nil, nil }

	startIdx, endIdx := 0, len(parts)-1
	if sinceUnix > 0 && len(parts) > 8 {
		startIdx = sort.Search(len(parts), func(i int) bool {
			_, maxT, ok := readPartTimeBounds(parts[i].dir)
			return ok && maxT >= sinceUnix
		})
		if startIdx > 0 { startIdx-- }
	}
	if untilUnix > 0 && len(parts) > 8 {
		if idx := sort.Search(len(parts), func(i int) bool {
			minT, _, ok := readPartTimeBounds(parts[i].dir)
			return ok && minT > untilUnix
		}); idx < len(parts) { endIdx = idx }
	}
	if endIdx >= len(parts) { endIdx = len(parts) - 1 }

	view := NewBlockView()
	compBuf := GetBlockBuffer()
	defer PutBlockBuffer(compBuf)
	scratch := GetBlockBuffer()
	defer PutBlockBuffer(scratch)

	groups := make(map[string]*ErrorGroup)

	for i := endIdx; i >= startIdx; i-- {
		if ctx.Err() != nil { return nil, ctx.Err() }
		pDir := parts[i].dir
		marksData, err := os.ReadFile(filepath.Join(pDir, "marks.bin"))
		if err != nil { continue }

		numBlocks := len(marksData) / BlockMarkSize
		if numBlocks == 0 { continue }

		var dataFile *os.File
		for bIdx := numBlocks - 1; bIdx >= 0; bIdx-- {
			if ctx.Err() != nil {
				if dataFile != nil { _ = dataFile.Close() }
				return nil, ctx.Err()
			}
			markOffset := bIdx * BlockMarkSize
			var mark BlockMark
			mark.Decode(marksData[markOffset : markOffset+BlockMarkSize])

			if (mark.LevelMask & targetLevelMask) == 0 { continue }
			if hasTargetService && (mark.ServiceBloom&targetServiceBit) != targetServiceBit { continue }

			if dataFile == nil {
				if dataFile, err = os.Open(filepath.Join(pDir, "data.bin")); err != nil { break }
			}

			if int(mark.CompressedLength) > cap(compBuf) {
				compBuf = make([]byte, mark.CompressedLength)
			} else {
				compBuf = compBuf[:mark.CompressedLength]
			}
			if _, err := dataFile.ReadAt(compBuf, int64(mark.CompressedOffset)); err != nil { continue }
			if err := view.UnpackMeta(compBuf, scratch); err != nil { continue }

			hasMatchingRow := false
			for rIdx := 0; rIdx < view.RowCount; rIdx++ {
				ts := view.Timestamps[rIdx]
				if (sinceUnix > 0 && ts < sinceUnix) || (untilUnix > 0 && ts > untilUnix) || (hasTargetService && view.ServiceIDs[rIdx] != targetServiceID) { continue }
				lvlStr := LevelIDToString(view.LevelIDs[rIdx])
				if (LevelToBit(lvlStr) & targetLevelMask) != 0 {
					hasMatchingRow = true
					break
				}
			}
			if !hasMatchingRow { continue }

			if err := view.EnsureMessagesDecompressed(scratch); err != nil { continue }

			for rIdx := 0; rIdx < view.RowCount; rIdx++ {
				ts := view.Timestamps[rIdx]
				if (sinceUnix > 0 && ts < sinceUnix) || (untilUnix > 0 && ts > untilUnix) || (hasTargetService && view.ServiceIDs[rIdx] != targetServiceID) { continue }
				lvlStr := LevelIDToString(view.LevelIDs[rIdx])
				if (LevelToBit(lvlStr) & targetLevelMask) == 0 { continue }

				msgBytes := view.MessageAt(rIdx)
				fp := FingerprintError(msgBytes)
				svcName, _ := r.dict.Lookup(view.ServiceIDs[rIdx])
				if IsNoiseService(svcName) { continue }
				key := svcName + "|" + lvlStr + "|" + fp
				tVal := time.Unix(0, ts).UTC()

				if eg, exists := groups[key]; exists {
					eg.Count++
					if tVal.Before(eg.FirstSeen) { eg.FirstSeen = tVal }
					if tVal.After(eg.LastSeen) { eg.LastSeen = tVal }
				} else {
					groups[key] = &ErrorGroup{
						Service: svcName, Level: lvlStr, Fingerprint: fp,
						SampleMessage: string(msgBytes), Count: 1, FirstSeen: tVal, LastSeen: tVal,
					}
				}
			}
		}
		if dataFile != nil { _ = dataFile.Close() }
	}

	result := make([]ErrorGroup, 0, len(groups))
	for _, g := range groups { result = append(result, *g) }
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count { return result[i].Count > result[j].Count }
		return result[i].LastSeen.After(result[j].LastSeen)
	})
	if params.Limit > 0 && len(result) > params.Limit { result = result[:params.Limit] }
	return result, nil
}
