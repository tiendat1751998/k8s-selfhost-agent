package logengine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single log entry passed into the log engine.
type Entry struct {
	Timestamp time.Time
	Service   string
	Level     string
	Message   string
	Raw       string
}

// WriterOption configures the Writer.
type WriterOption func(*Writer)

// WithMaxRowsPerBlock configures the threshold rows before flushing a block.
func WithMaxRowsPerBlock(n int) WriterOption {
	return func(w *Writer) {
		if n > 0 {
			w.maxRows = n
		}
	}
}

// WithFlushInterval configures the periodic flush duration.
func WithFlushInterval(d time.Duration) WriterOption {
	return func(w *Writer) {
		if d > 0 {
			w.flushInterval = d
		}
	}
}

// Writer writes log entries to columnar parts on disk with sparse indexing.
type Writer struct {
	mu            sync.Mutex
	dir           string
	partsDir      string
	dict          *LabelDictionary
	builder       *BlockBuilder
	maxRows       int
	flushInterval time.Duration
	nextPartID    uint64
	closeCh       chan struct{}
	wg            sync.WaitGroup
	closed        bool
	lastErr       error
}

// NewWriter initializes a Writer on the specified directory.
func NewWriter(dir string, dict *LabelDictionary, opts ...WriterOption) (*Writer, error) {
	partsDir := filepath.Join(dir, "parts")
	if err := os.MkdirAll(partsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parts dir: %w", err)
	}

	w := &Writer{
		dir:           dir,
		partsDir:      partsDir,
		dict:          dict,
		builder:       NewBlockBuilder(),
		maxRows:       2048,
		flushInterval: 5 * time.Second,
		closeCh:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	entries, _ := os.ReadDir(partsDir)
	for _, e := range entries {
		if e.IsDir() {
			var id uint64
			if _, err := fmt.Sscanf(e.Name(), "part_%d", &id); err == nil {
				if id >= w.nextPartID {
					w.nextPartID = id + 1
				}
			}
		}
	}
	if w.nextPartID == 0 {
		w.nextPartID = 1
	}

	w.wg.Add(1)
	go w.flushLoop()

	return w, nil
}

func (w *Writer) flushLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.closeCh:
			return
		case <-ticker.C:
			w.mu.Lock()
			if err := w.flushLocked(); err != nil {
				w.lastErr = err
			}
			w.mu.Unlock()
		}
	}
}

// LastError returns the last asynchronous error encountered during periodic flush.
func (w *Writer) LastError() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastErr
}

// Write appends an entry to the current staging block and flushes if threshold is reached.
func (w *Writer) Write(entry Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	svcID := w.dict.GetOrInsert(entry.Service)
	lvlID := w.dict.GetOrInsert(entry.Level)
	ts := entry.Timestamp.UnixNano()

	w.builder.Append(ts, svcID, lvlID, []byte(entry.Message))

	if w.builder.RowCount() >= w.maxRows || w.builder.EstimatedSize() >= BlockBufferSize-4096 {
		return w.flushLocked()
	}
	return nil
}

// Flush forces a write of buffered rows to a new part directory.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

func (w *Writer) flushLocked() error {
	if w.builder.RowCount() == 0 {
		return nil
	}

	partName := fmt.Sprintf("part_%016d", w.nextPartID)
	w.nextPartID++
	partDir := filepath.Join(w.partsDir, partName)
	tmpDir := filepath.Join(w.partsDir, fmt.Sprintf("%s.tmp_%d", partName, time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp part dir %s: %w", tmpDir, err)
	}

	scratchBuf := GetBlockBuffer()
	defer PutBlockBuffer(scratchBuf)

	compBuf := GetBlockBuffer()
	defer PutBlockBuffer(compBuf)

	uncompressed, minTime, maxTime, bloom := w.builder.Pack(scratchBuf)
	compressed := CompressBlock(uncompressed, compBuf)

	// 1. Write data.bin
	if err := os.WriteFile(filepath.Join(tmpDir, "data.bin"), compressed, 0644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to write data.bin: %w", err)
	}

	// 2. Write marks.bin
	mark := BlockMark{
		CompressedOffset: 0,
		CompressedLength: uint32(len(compressed)),
		UncompressedSize: uint32(len(uncompressed)),
		RowCount:         uint32(w.builder.RowCount()),
	}
	markBytes := make([]byte, BlockMarkSize)
	mark.Encode(markBytes)
	if err := os.WriteFile(filepath.Join(tmpDir, "marks.bin"), markBytes, 0644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to write marks.bin: %w", err)
	}

	// 3. Write bloom.bin
	if err := os.WriteFile(filepath.Join(tmpDir, "bloom.bin"), bloom.Bytes(), 0644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to write bloom.bin: %w", err)
	}

	// 4. Write primary.idx with deferred close helper
	pidx := NewPrimaryIndex()
	pidx.Add(minTime, maxTime)
	if err := writePrimaryIndexAtomic(filepath.Join(tmpDir, "primary.idx"), pidx); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to write primary.idx: %w", err)
	}

	// 5. Atomic Rename to final immutable part directory
	if err := os.Rename(tmpDir, partDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("failed to commit atomic part directory %s: %w", partName, err)
	}

	// 6. Persist dictionary snapshot
	dictData, err := json.Marshal(w.dict.Export())
	if err != nil {
		return fmt.Errorf("failed to marshal dict: %w", err)
	}
	if err := os.WriteFile(filepath.Join(w.dir, "dict.json"), dictData, 0644); err != nil {
		return fmt.Errorf("failed to write dict.json: %w", err)
	}

	w.builder.Reset()
	return nil
}

func writePrimaryIndexAtomic(path string, pidx *PrimaryIndex) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return pidx.WritePrimaryIndex(f)
}

// Close flushes buffered rows, stops the flush loop, and releases resources.
func (w *Writer) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	close(w.closeCh)
	w.mu.Unlock()

	w.wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}
