package logengine

// SnapshotUncommitted returns all uncommitted log entries currently held in the
// Writer's in-memory staging builder and ingest ring buffer, in chronological order.
// Thread-safe and non-destructive with 0 data races.
func (w *Writer) SnapshotUncommitted() []Entry {
	w.builderMu.Lock()
	defer w.builderMu.Unlock()

	// Drain all queued entries from ring buffer into builder under builderMu lock
	batch := make([]Entry, 1024)
	for {
		n := w.drainToBuilderLocked(batch)
		if n == 0 {
			break
		}
	}

	return w.builder.SnapshotEntries(w.dict, w.metaDict)
}

// MetaDictionary returns the Writer's metadata dictionary.
func (w *Writer) MetaDictionary() *MetaDictionary {
	return w.metaDict
}

// ActivePartDir returns the path to the currently active writing part, if any.
func (w *Writer) ActivePartDir() string {
	w.closedMu.RLock()
	defer w.closedMu.RUnlock()
	return w.activePartDir
}

// LastError returns the last asynchronous error encountered during flush.
func (w *Writer) LastError() error {
	w.closedMu.RLock()
	defer w.closedMu.RUnlock()
	return w.lastErr
}

// DroppedEntries returns total entries dropped by ingest ring buffer.
func (w *Writer) DroppedEntries() uint64 {
	if w.ringBuf != nil {
		return w.ringBuf.Dropped()
	}
	return 0
}

// SealActivePart closes the current active multi-block part directory so the next flush starts a new part.
func (w *Writer) SealActivePart() {
	w.closedMu.Lock()
	defer w.closedMu.Unlock()
	w.sealActivePartLocked()
}

func (w *Writer) closeActiveFiles() {
	if w.activeDataFile != nil { _ = w.activeDataFile.Close(); w.activeDataFile = nil }
	if w.activeMarksFile != nil { _ = w.activeMarksFile.Close(); w.activeMarksFile = nil }
	if w.activeBloomFile != nil { _ = w.activeBloomFile.Close(); w.activeBloomFile = nil }
	if w.activePidxFile != nil { _ = w.activePidxFile.Close(); w.activePidxFile = nil }
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

func (w *Writer) setLastError(err error) {
	if err == nil { return }
	w.closedMu.Lock()
	w.lastErr = err
	w.closedMu.Unlock()
}



