package logengine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CompactorOptions configures part merging thresholds.
type CompactorOptions struct {
	MinPartsToMerge int   // Minimum sealed parts required to trigger merge (default 2)
	MaxPartsToMerge int   // Maximum parts consolidated in a single pass (default 8)
	MaxMergedBytes  int64 // Target maximum compressed bytes per merged part (default 32MB)
}

// DefaultCompactorOptions returns standard MergeTree compactor settings.
func DefaultCompactorOptions() CompactorOptions {
	return CompactorOptions{
		MinPartsToMerge: 2,
		MaxPartsToMerge: 8,
		MaxMergedBytes:  32 * 1024 * 1024,
	}
}

// Compactor consolidates small sealed columnar parts into larger parts without recompressing ZSTD streams.
type Compactor struct {
	mu       sync.Mutex
	partsDir string
	writer   *Writer
	reader   *Reader
	opts     CompactorOptions
	stopCh   chan struct{}
	wg       sync.WaitGroup
	closed   bool
}

// NewCompactor initializes a MergeTree part compactor for partsDir.
func NewCompactor(partsDir string, writer *Writer, reader *Reader, opts ...CompactorOptions) *Compactor {
	opt := DefaultCompactorOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.MinPartsToMerge < 2 {
		opt.MinPartsToMerge = 2
	}
	if opt.MaxPartsToMerge < opt.MinPartsToMerge {
		opt.MaxPartsToMerge = opt.MinPartsToMerge * 4
	}

	return &Compactor{
		partsDir: partsDir,
		writer:   writer,
		reader:   reader,
		opts:     opt,
		stopCh:   make(chan struct{}),
	}
}

type sealedPartInfo struct {
	id   uint64
	name string
	dir  string
	size int64
}

// CompactOnce inspects sealed parts, merges a batch of small parts into a single part, and returns the merged count.
func (c *Compactor) CompactOnce() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.partsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	activeDir := ""
	if c.writer != nil {
		activeDir = c.writer.ActivePartDir()
	}

	var candidates []sealedPartInfo
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "part_") {
			continue
		}
		pDir := filepath.Join(c.partsDir, e.Name())
		if activeDir != "" && (pDir == activeDir || filepath.Clean(pDir) == filepath.Clean(activeDir)) {
			continue // MUST NOT touch active writing part!
		}
		var id uint64
		if _, err := fmt.Sscanf(e.Name(), "part_%d", &id); err != nil {
			continue
		}
		dataFi, err := os.Stat(filepath.Join(pDir, "data.bin"))
		if err != nil {
			continue
		}
		candidates = append(candidates, sealedPartInfo{
			id:   id,
			name: e.Name(),
			dir:  pDir,
			size: dataFi.Size(),
		})
	}

	if len(candidates) < c.opts.MinPartsToMerge {
		return 0, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].id < candidates[j].id
	})

	// Select contiguous batch of parts to merge
	var batch []sealedPartInfo
	var batchBytes int64
	for _, p := range candidates {
		batch = append(batch, p)
		batchBytes += p.size
		if len(batch) >= c.opts.MaxPartsToMerge || (c.opts.MaxMergedBytes > 0 && batchBytes >= c.opts.MaxMergedBytes) {
			break
		}
	}

	if len(batch) < c.opts.MinPartsToMerge {
		return 0, nil
	}

	minID := batch[0].id
	tmpDir := filepath.Join(c.partsDir, fmt.Sprintf("tmp_merge_%016d", minID))
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return 0, fmt.Errorf("create tmp merge dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	tmpData, err := os.OpenFile(filepath.Join(tmpDir, "data.bin"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("open tmp data.bin: %w", err)
	}
	defer tmpData.Close()

	tmpMarks, err := os.OpenFile(filepath.Join(tmpDir, "marks.bin"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("open tmp marks.bin: %w", err)
	}
	defer tmpMarks.Close()

	tmpBloom, err := os.OpenFile(filepath.Join(tmpDir, "bloom.bin"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("open tmp bloom.bin: %w", err)
	}
	defer tmpBloom.Close()

	tmpPidx, err := os.OpenFile(filepath.Join(tmpDir, "primary.idx"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return 0, fmt.Errorf("open tmp primary.idx: %w", err)
	}
	defer tmpPidx.Close()

	var currentMergedOffset uint64
	for _, p := range batch {
		// 1. Concatenate data.bin without ZSTD recompression
		srcData, err := os.Open(filepath.Join(p.dir, "data.bin"))
		if err != nil {
			return 0, fmt.Errorf("open src data.bin in %s: %w", p.name, err)
		}
		nCopied, err := io.Copy(tmpData, srcData)
		_ = srcData.Close()
		if err != nil {
			return 0, fmt.Errorf("copy data.bin: %w", err)
		}

		// 2. Re-base marks.bin compressed offsets
		marksBytes, err := os.ReadFile(filepath.Join(p.dir, "marks.bin"))
		if err != nil {
			return 0, fmt.Errorf("read src marks.bin in %s: %w", p.name, err)
		}
		numMarks := len(marksBytes) / BlockMarkSize
		for mIdx := 0; mIdx < numMarks; mIdx++ {
			offset := mIdx * BlockMarkSize
			var mark BlockMark
			mark.Decode(marksBytes[offset : offset+BlockMarkSize])
			mark.CompressedOffset += currentMergedOffset // Re-base offset!
			var encoded [BlockMarkSize]byte
			mark.Encode(encoded[:])
			if _, err := tmpMarks.Write(encoded[:]); err != nil {
				return 0, fmt.Errorf("write merged marks.bin: %w", err)
			}
		}

		// 3. Concatenate bloom.bin
		bloomBytes, err := os.ReadFile(filepath.Join(p.dir, "bloom.bin"))
		if err != nil && !os.IsNotExist(err) {
			return 0, fmt.Errorf("read src bloom.bin in %s: %w", p.name, err)
		}
		if len(bloomBytes) > 0 {
			if _, err := tmpBloom.Write(bloomBytes); err != nil {
				return 0, fmt.Errorf("write merged bloom.bin: %w", err)
			}
		}

		// 4. Concatenate primary.idx
		pidxBytes, err := os.ReadFile(filepath.Join(p.dir, "primary.idx"))
		if err != nil {
			return 0, fmt.Errorf("read src primary.idx in %s: %w", p.name, err)
		}
		if _, err := tmpPidx.Write(pidxBytes); err != nil {
			return 0, fmt.Errorf("write merged primary.idx: %w", err)
		}

		currentMergedOffset += uint64(nCopied)
	}

	_ = tmpData.Sync(); _ = tmpData.Close()
	_ = tmpMarks.Sync(); _ = tmpMarks.Close()
	_ = tmpBloom.Sync(); _ = tmpBloom.Close()
	_ = tmpPidx.Sync(); _ = tmpPidx.Close()

	// Atomic directory swap under coordination locks
	if c.reader != nil {
		c.reader.mu.Lock()
		defer c.reader.mu.Unlock()
	}
	if c.writer != nil {
		c.writer.closedMu.Lock()
		defer c.writer.closedMu.Unlock()
	}

	finalPartName := fmt.Sprintf("part_%016d", minID)
	finalPartDir := filepath.Join(c.partsDir, finalPartName)

	bakDir := finalPartDir + ".bak"
	bakMoved := false
	if _, err := os.Stat(finalPartDir); err == nil {
		if err := os.Rename(finalPartDir, bakDir); err != nil {
			_ = os.RemoveAll(tmpDir)
			return 0, fmt.Errorf("stage aside existing part %s: %w", finalPartName, err)
		}
		bakMoved = true
	}

	if err := os.Rename(tmpDir, finalPartDir); err != nil {
		if bakMoved {
			_ = os.Rename(bakDir, finalPartDir)
		}
		_ = os.RemoveAll(tmpDir)
		return 0, fmt.Errorf("rename merged part to %s: %w", finalPartName, err)
	}

	// Delete old source parts ONLY after finalPartDir has been safely placed
	for _, p := range batch {
		if p.dir != finalPartDir {
			_ = os.RemoveAll(p.dir)
		}
	}
	if bakMoved {
		_ = os.RemoveAll(bakDir)
	}

	if c.reader != nil {
		c.reader.cachedBlockCount = 0
	}

	return len(batch), nil
}

// StartBackground runs periodic compaction in the background until Stop is called or ctx is canceled.
func (c *Compactor) StartBackground(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				_, _ = c.CompactOnce()
			}
		}
	}()
}

// Stop terminates background compaction.
func (c *Compactor) Stop() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.stopCh)
	c.mu.Unlock()
	c.wg.Wait()
}
