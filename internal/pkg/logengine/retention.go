package logengine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WithMaxRetention configures automatic age-based part pruning.
func WithMaxRetention(d time.Duration) WriterOption {
	return func(w *Writer) {
		w.maxRetention = d
	}
}

// WithMaxDiskBytes configures automatic disk quota pruning.
func WithMaxDiskBytes(bytes int64) WriterOption {
	return func(w *Writer) {
		w.maxDiskBytes = bytes
	}
}

const maxSealedParts = 128

type partDiskInfo struct {
	name    string
	dir     string
	modTime time.Time
	size    int64
	id      uint64
}

// pruneOldParts removes older parts when retention duration or disk quota is exceeded.
// The currently active part directory is never pruned.
func pruneOldParts(partsDir, activePartDir string, maxRetention time.Duration, maxDiskBytes int64) error {
	entries, err := os.ReadDir(partsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	type partCandidate struct {
		name string
		dir  string
		id   uint64
	}

	var candidates []partCandidate
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "part_") {
			continue
		}
		pDir := filepath.Join(partsDir, e.Name())
		if activePartDir != "" && pDir == activePartDir {
			continue // active part is immune from pruning
		}
		var id uint64
		_, _ = fmt.Sscanf(e.Name(), "part_%d", &id)
		candidates = append(candidates, partCandidate{
			name: e.Name(),
			dir:  pDir,
			id:   id,
		})
	}

	// Sort candidates from oldest to newest by part ID
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].id < candidates[j].id
	})

	// 0. Enforce cap of effectiveMaxParts sealed directories BEFORE walking files
	effectiveMaxParts := maxSealedParts
	if maxDiskBytes > 0 {
		needed := int(maxDiskBytes/(4*1024*1024)) + 16
		if needed > effectiveMaxParts {
			effectiveMaxParts = needed
		}
	}

	if len(candidates) > effectiveMaxParts {
		excess := len(candidates) - effectiveMaxParts
		for i := 0; i < excess; i++ {
			_ = os.RemoveAll(candidates[i].dir)
			if (i+1)%25 == 0 {
				time.Sleep(2 * time.Millisecond)
			}
		}
		candidates = candidates[excess:]
	}

	if maxRetention <= 0 && maxDiskBytes <= 0 {
		return nil
	}

	var parts []partDiskInfo
	var totalBytes int64

	for _, c := range candidates {
		info, err := os.Stat(c.dir)
		if err != nil {
			continue
		}

		var pSize int64
		_ = filepath.Walk(c.dir, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && fi != nil && !fi.IsDir() {
				pSize += fi.Size()
			}
			return nil
		})

		totalBytes += pSize
		parts = append(parts, partDiskInfo{
			name:    c.name,
			dir:     c.dir,
			modTime: info.ModTime(),
			size:    pSize,
			id:      c.id,
		})
	}

	// Sort parts from oldest to newest by numeric part ID and modification time
	sort.Slice(parts, func(i, j int) bool {
		if parts[i].id != parts[j].id {
			return parts[i].id < parts[j].id
		}
		return parts[i].modTime.Before(parts[j].modTime)
	})

	now := time.Now()
	var remainingParts []partDiskInfo

	// 1. Retention-based pruning
	for _, p := range parts {
		if maxRetention > 0 && now.Sub(p.modTime) > maxRetention {
			_ = os.RemoveAll(p.dir)
			totalBytes -= p.size
		} else {
			remainingParts = append(remainingParts, p)
		}
	}

	// 2. Disk quota-based pruning
	if maxDiskBytes > 0 {
		for len(remainingParts) > 0 && totalBytes > maxDiskBytes {
			oldest := remainingParts[0]
			_ = os.RemoveAll(oldest.dir)
			totalBytes -= oldest.size
			remainingParts = remainingParts[1:]
		}
	}

	return nil
}
