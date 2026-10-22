package logengine

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	maxBlocksPerPart = 256
	maxPartBytes     = 16 * 1024 * 1024 // 16MB compressed
)

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

	activePartDir    string
	activePartBlocks int
	activePartBytes  int64
	activeDataFile   *os.File
	activeMarksFile  *os.File
	activeBloomFile  *os.File
	activePidxFile   *os.File
	maxRetention      time.Duration
	maxDiskBytes      int64
	lastPruneTime     time.Time
	lastSavedDictSize int

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
	go func() {
		_ = pruneOldParts(w.partsDir, "", w.maxRetention, w.maxDiskBytes)
	}()

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()
	idleTimer := time.NewTimer(100 * time.Millisecond)
	defer idleTimer.Stop()

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
			w.closedMu.Lock()
			w.sealActivePartLocked()
			w.closedMu.Unlock()
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
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(100 * time.Millisecond)

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
			case <-idleTimer.C:
			}
		}
	}
}

func (w *Writer) drainToBuilder(batch []Entry) int {
	n := w.ringBuf.DrainBatch(batch)
	for i := 0; i < n; i++ {
		e := &batch[i]
		svcID := w.dict.GetOrInsert(e.Service)
		lvlID := ParseLevelID(e.Level)
		ts := e.Timestamp.UnixNano()
		w.builder.Append(ts, svcID, lvlID, []byte(e.Message))
		batch[i] = Entry{}
	}
	return n
}

// SealActivePart closes the current active multi-block part directory so the next flush starts a new part.
func (w *Writer) SealActivePart() {
	w.closedMu.Lock()
	defer w.closedMu.Unlock()
	w.sealActivePartLocked()
}

func (w *Writer) closeActiveFiles() {
	if w.activeDataFile != nil {
		_ = w.activeDataFile.Close()
		w.activeDataFile = nil
	}
	if w.activeMarksFile != nil {
		_ = w.activeMarksFile.Close()
		w.activeMarksFile = nil
	}
	if w.activeBloomFile != nil {
		_ = w.activeBloomFile.Close()
		w.activeBloomFile = nil
	}
	if w.activePidxFile != nil {
		_ = w.activePidxFile.Close()
		w.activePidxFile = nil
	}
}

func (w *Writer) sealActivePartLocked() {
	w.closeActiveFiles()
	if w.activePartDir != "" {
		w.nextPartID++
		w.activePartDir = ""
		w.activePartBlocks = 0
		w.activePartBytes = 0
	}
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

	w.closedMu.Lock()
	isNewPart := false
	if w.activePartDir == "" || w.activePartBlocks >= maxBlocksPerPart || w.activePartBytes >= maxPartBytes {
		if w.activePartDir != "" {
			w.closeActiveFiles()
			w.nextPartID++
		}
		partName := fmt.Sprintf("part_%016d", w.nextPartID)
		w.activePartDir = filepath.Join(w.partsDir, partName)
		w.activePartBlocks = 0
		w.activePartBytes = 0
		if err := os.MkdirAll(w.activePartDir, 0755); err != nil {
			w.closedMu.Unlock()
			return fmt.Errorf("failed to create part dir %s: %w", w.activePartDir, err)
		}
		isNewPart = true
	}
	activeDir := w.activePartDir

	if w.activeDataFile == nil {
		var err error
		w.activeDataFile, err = os.OpenFile(filepath.Join(activeDir, "data.bin"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			w.closeActiveFiles()
			w.closedMu.Unlock()
			return fmt.Errorf("failed to open data.bin: %w", err)
		}
		w.activeMarksFile, err = os.OpenFile(filepath.Join(activeDir, "marks.bin"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			w.closeActiveFiles()
			w.closedMu.Unlock()
			return fmt.Errorf("failed to open marks.bin: %w", err)
		}
		w.activeBloomFile, err = os.OpenFile(filepath.Join(activeDir, "bloom.bin"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			w.closeActiveFiles()
			w.closedMu.Unlock()
			return fmt.Errorf("failed to open bloom.bin: %w", err)
		}
		w.activePidxFile, err = os.OpenFile(filepath.Join(activeDir, "primary.idx"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			w.closeActiveFiles()
			w.closedMu.Unlock()
			return fmt.Errorf("failed to open primary.idx: %w", err)
		}
	}
	w.closedMu.Unlock()

	scratchBuf := GetBlockBuffer()
	defer PutBlockBuffer(scratchBuf)

	compBuf := GetBlockBuffer()
	defer PutBlockBuffer(compBuf)

	compressed, uncompressedSize, minTime, maxTime, bloom, levelMask, serviceBloom := w.builder.PackAndCompress(scratchBuf, compBuf)

	// 1. Append data.bin
	offset := w.activePartBytes
	if _, err := w.activeDataFile.Write(compressed); err != nil {
		return fmt.Errorf("failed to append data.bin: %w", err)
	}

	// 2. Append marks.bin
	mark := BlockMark{
		CompressedOffset: uint64(offset),
		CompressedLength: uint32(len(compressed)),
		UncompressedSize: uint32(uncompressedSize),
		RowCount:         uint32(w.builder.RowCount()),
		LevelMask:        levelMask,
		ServiceBloom:     serviceBloom,
	}
	var markBytes [BlockMarkSize]byte
	mark.Encode(markBytes[:])
	if _, err := w.activeMarksFile.Write(markBytes[:]); err != nil {
		return fmt.Errorf("failed to append marks.bin: %w", err)
	}

	// 3. Append bloom.bin
	if _, err := w.activeBloomFile.Write(bloom.Bytes()); err != nil {
		return fmt.Errorf("failed to append bloom.bin: %w", err)
	}

	// 4. Append primary.idx
	var idxBytes [16]byte
	binary.LittleEndian.PutUint64(idxBytes[0:8], uint64(minTime))
	binary.LittleEndian.PutUint64(idxBytes[8:16], uint64(maxTime))
	if _, err := w.activePidxFile.Write(idxBytes[:]); err != nil {
		return fmt.Errorf("failed to append primary.idx: %w", err)
	}

	w.closedMu.Lock()
	w.activePartBlocks++
	w.activePartBytes += int64(len(compressed))
	w.closedMu.Unlock()

	// 5. Persist dictionary snapshot (only when dictionary has changed)
	if w.dict != nil && w.dict.Size() != w.lastSavedDictSize {
		dictData, err := json.Marshal(w.dict.Export())
		if err != nil {
			return fmt.Errorf("failed to marshal dict: %w", err)
		}
		if err := os.WriteFile(filepath.Join(w.dir, "dict.json"), dictData, 0644); err != nil {
			return fmt.Errorf("failed to write dict.json: %w", err)
		}
		w.lastSavedDictSize = w.dict.Size()
	}

	w.builder.Reset()

	// 6. Check retention / disk quota pruning (throttled: only on new part or once per 60s)
	now := time.Now()
	if isNewPart || w.lastPruneTime.IsZero() || now.Sub(w.lastPruneTime) >= 60*time.Second {
		w.lastPruneTime = now
		if err := pruneOldParts(w.partsDir, activeDir, w.maxRetention, w.maxDiskBytes); err != nil {
			w.setLastError(err)
		}
	}

	return nil
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

	w.closedMu.Lock()
	w.sealActivePartLocked()
	w.closedMu.Unlock()
	return w.LastError()
}
