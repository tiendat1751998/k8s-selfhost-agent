package logengine

import (
	"context"
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

// WithRedactor configures the secret redactor on the Writer (pass nil to disable).
func WithRedactor(r *InStreamRedactor) WriterOption {
	return func(w *Writer) {
		w.redactor = r
	}
}

// WithRingBufferCapacity configures the ingest ring buffer capacity.
func WithRingBufferCapacity(cap int) WriterOption {
	return func(w *Writer) {
		if cap > 0 {
			w.ringCap = cap
		}
	}
}

// Writer writes log entries to columnar parts on disk with sparse indexing.
// Ingestion is completely decoupled from disk I/O via a lock-free MPSC ring buffer.
type Writer struct {
	dir           string
	partsDir      string
	dict          *LabelDictionary
	builder       *BlockBuilder
	redactor      *InStreamRedactor
	ringBuf       *RingBuffer
	ringCap       int
	maxRows       int
	flushInterval time.Duration
	nextPartID    uint64

	flushReqCh chan chan error
	closeCh    chan struct{}
	wg         sync.WaitGroup

	closedMu sync.RWMutex
	closed   bool
	lastErr  error
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
		redactor:      NewInStreamRedactor(),
		ringCap:       DefaultRingBufferCapacity,
		maxRows:       2048,
		flushInterval: 5 * time.Second,
		flushReqCh:    make(chan chan error),
		closeCh:       make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	w.ringBuf = NewRingBuffer(w.ringCap)

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
	go w.flusherLoop()

	return w, nil
}

// flusherLoop is the single background flusher thread that drains batches
// from the lock-free ring buffer, packs columnar blocks, compresses via zstd,
// and writes parts to disk.
func (w *Writer) flusherLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]Entry, w.maxRows)

	for {
		drained := w.drainToBuilder(batch)

		if w.builder.RowCount() >= w.maxRows || w.builder.EstimatedSize() >= BlockBufferSize-4096 {
			if err := w.flushCurrentBlock(); err != nil {
				w.setLastError(err)
			}
		}

		select {
		case <-w.closeCh:
			// Drain all remaining items in the ring buffer
			for {
				n := w.drainToBuilder(batch)
				if n == 0 {
					break
				}
				if w.builder.RowCount() >= w.maxRows || w.builder.EstimatedSize() >= BlockBufferSize-4096 {
					if err := w.flushCurrentBlock(); err != nil {
						w.setLastError(err)
					}
				}
			}
			// Flush final block
			if err := w.flushCurrentBlock(); err != nil {
				w.setLastError(err)
			}
			return

		case req := <-w.flushReqCh:
			// Explicit Flush() request: drain all items currently in ring buffer
			for {
				n := w.drainToBuilder(batch)
				if n == 0 {
					break
				}
				if w.builder.RowCount() >= w.maxRows || w.builder.EstimatedSize() >= BlockBufferSize-4096 {
					if err := w.flushCurrentBlock(); err != nil {
						w.setLastError(err)
					}
				}
			}
			err := w.flushCurrentBlock()
			if err != nil {
				w.setLastError(err)
			}
			req <- err

		case <-ticker.C:
			if w.builder.RowCount() > 0 {
				if err := w.flushCurrentBlock(); err != nil {
					w.setLastError(err)
				}
			}

		default:
			if drained > 0 {
				continue
			}
			// Ring buffer is empty, wait briefly or for notifications
			select {
			case <-w.closeCh:
			case req := <-w.flushReqCh:
				for {
					n := w.drainToBuilder(batch)
					if n == 0 {
						break
					}
					if w.builder.RowCount() >= w.maxRows || w.builder.EstimatedSize() >= BlockBufferSize-4096 {
						if err := w.flushCurrentBlock(); err != nil {
							w.setLastError(err)
						}
					}
				}
				err := w.flushCurrentBlock()
				if err != nil {
					w.setLastError(err)
				}
				req <- err
			case <-ticker.C:
				if w.builder.RowCount() > 0 {
					if err := w.flushCurrentBlock(); err != nil {
						w.setLastError(err)
					}
				}
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
}

func (w *Writer) drainToBuilder(batch []Entry) int {
	n := w.ringBuf.DrainBatch(batch)
	for i := 0; i < n; i++ {
		e := &batch[i]
		svcID := w.dict.GetOrInsert(e.Service)
		lvlID := w.dict.GetOrInsert(e.Level)
		ts := e.Timestamp.UnixNano()
		w.builder.Append(ts, svcID, lvlID, []byte(e.Message))
	}
	return n
}

// LastError returns the last asynchronous error encountered during flush.
func (w *Writer) LastError() error {
	w.closedMu.RLock()
	defer w.closedMu.RUnlock()
	return w.lastErr
}

func (w *Writer) setLastError(err error) {
	if err == nil {
		return
	}
	w.closedMu.Lock()
	w.lastErr = err
	w.closedMu.Unlock()
}

// DroppedEntries returns total entries dropped by ingest ring buffer.
func (w *Writer) DroppedEntries() uint64 {
	if w.ringBuf != nil {
		return w.ringBuf.Dropped()
	}
	return 0
}

// Write pushes an entry into the lock-free ring buffer without acquiring a disk mutex.
func (w *Writer) Write(entry Entry) error {
	w.closedMu.RLock()
	if w.closed {
		w.closedMu.RUnlock()
		return fmt.Errorf("writer is closed")
	}
	w.closedMu.RUnlock()

	if w.redactor != nil {
		entry.Message = w.redactor.MaskString(entry.Message)
	}

	if w.ringBuf.Push(entry) {
		return nil
	}

	// Backpressure: buffer is temporarily full, retry with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := w.ringBuf.PushWait(ctx, entry); err != nil {
		return fmt.Errorf("ingest ring buffer full: %w", err)
	}
	return nil
}

// WriteBatch pushes a slice of entries into the lock-free ring buffer.
func (w *Writer) WriteBatch(entries []Entry) error {
	w.closedMu.RLock()
	if w.closed {
		w.closedMu.RUnlock()
		return fmt.Errorf("writer is closed")
	}
	w.closedMu.RUnlock()

	var writeErr error
	for i := range entries {
		entry := entries[i]
		if w.redactor != nil {
			entry.Message = w.redactor.MaskString(entry.Message)
		}
		if !w.ringBuf.Push(entry) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := w.ringBuf.PushWait(ctx, entry); err != nil {
				cancel()
				if writeErr == nil {
					writeErr = fmt.Errorf("ingest ring buffer full: %w", err)
				}
				continue
			}
			cancel()
		}
	}
	return writeErr
}

// Flush forces a synchronous write of all buffered items to disk.
func (w *Writer) Flush() error {
	w.closedMu.RLock()
	if w.closed {
		w.closedMu.RUnlock()
		return fmt.Errorf("writer is closed")
	}
	w.closedMu.RUnlock()

	resp := make(chan error, 1)
	select {
	case w.flushReqCh <- resp:
		return <-resp
	case <-w.closeCh:
		return fmt.Errorf("writer is closed")
	}
}

func (w *Writer) flushCurrentBlock() error {
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

	// 4. Write primary.idx
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

// Close flushes all remaining items in the ring buffer, writes the final block, and stops flusher.
func (w *Writer) Close() error {
	w.closedMu.Lock()
	if w.closed {
		w.closedMu.Unlock()
		return nil
	}
	w.closed = true
	close(w.closeCh)
	w.closedMu.Unlock()

	w.wg.Wait()
	return w.LastError()
}
