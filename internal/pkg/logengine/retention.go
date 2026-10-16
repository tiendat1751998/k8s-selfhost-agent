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
	if maxRetention <= 0 && maxDiskBytes <= 0 {
		return nil
	}

	entries, err := os.ReadDir(partsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var parts []partDiskInfo
	var totalBytes int64

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

		info, err := e.Info()
		if err != nil {
			continue
		}

		var pSize int64
		_ = filepath.Walk(pDir, func(_ string, fi os.FileInfo, err error) error {
			if err == nil && fi != nil && !fi.IsDir() {
				pSize += fi.Size()
			}
			return nil
		})

		totalBytes += pSize
		parts = append(parts, partDiskInfo{
			name:    e.Name(),
			dir:     pDir,
			modTime: info.ModTime(),
			size:    pSize,
			id:      id,
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
