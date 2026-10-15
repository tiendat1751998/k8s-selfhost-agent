package clickhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	usecaseIngestGuard "github.com/datdt/k8sselfhost/internal/usecase/ingestguard"
)

// Ensure adapter structs satisfy usecase IngestGuard interfaces.
var (
	_ usecaseIngestGuard.PartsInspector   = (*PartsInspectorAdapter)(nil)
	_ usecaseIngestGuard.BulkWriter       = (*BulkWriterAdapter)(nil)
	_ usecaseIngestGuard.MergeAccelerator = (*MergeAcceleratorAdapter)(nil)
	_ usecaseIngestGuard.SpillJournal     = (*DiskSpillJournal)(nil)
)

// PartsInspectorAdapter queries system.parts to evaluate active parts and merge health.
type PartsInspectorAdapter struct {
	client *Client
}

// NewPartsInspector creates a new PartsInspectorAdapter.
func NewPartsInspector(client *Client) *PartsInspectorAdapter {
	return &PartsInspectorAdapter{client: client}
}

// InspectTableParts inspects active parts on ClickHouse MergeTree tables with a 2s timeout.
func (p *PartsInspectorAdapter) InspectTableParts(ctx context.Context, database, table string) (int64, bool, error) {
	if p.client == nil {
		return 0, false, nil
	}
	conn, err := p.client.Conn(ctx)
	if err != nil {
		return 0, false, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	const query = "SELECT count(), max(modification_time) FROM system.parts WHERE database = currentDatabase() AND table = $1 AND active = 1"
	var (
		count  uint64
		maxMod *time.Time
	)
	if err := conn.QueryRow(queryCtx, query, table).Scan(&count, &maxMod); err != nil {
		return 0, false, nil
	}

	mergeInProgress := maxMod != nil && time.Since(*maxMod) < 5*time.Second
	return int64(count), mergeInProgress, nil
}

// BulkWriterAdapter forwards batches to BatchWriter, LogRepository, or SpillJournal.
type BulkWriterAdapter struct {
	client       *Client
	batchWriter  *BatchWriter
	repo         *LogRepository
	spillJournal usecaseIngestGuard.SpillJournal
}

// NewBulkWriter creates a new BulkWriterAdapter.
func NewBulkWriter(client *Client, bw *BatchWriter, repo *LogRepository, journal usecaseIngestGuard.SpillJournal) *BulkWriterAdapter {
	return &BulkWriterAdapter{
		client:       client,
		batchWriter:  bw,
		repo:         repo,
		spillJournal: journal,
	}
}

// WriteBulk converts and persists bulk records into storage.
func (b *BulkWriterAdapter) WriteBulk(ctx context.Context, database, table string, records []any) error {
	if len(records) == 0 {
		return nil
	}

	entries := make([]logging.LogEntry, 0, len(records))
	for _, rec := range records {
		entry, err := convertToLogEntry(rec)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	if b.batchWriter != nil {
		return b.batchWriter.WriteBatch(entries)
	}
	if b.repo != nil {
		return b.repo.IngestBatch(ctx, entries)
	}

	if b.spillJournal != nil {
		return b.spillJournal.Spill(ctx, database, table, records)
	}
	return nil
}

func convertToLogEntry(rec any) (logging.LogEntry, error) {
	switch v := rec.(type) {
	case logging.LogEntry:
		return v, nil
	case *logging.LogEntry:
		if v != nil {
			return *v, nil
		}
		return logging.LogEntry{}, errors.New("nil LogEntry pointer")
	case []byte:
		var entry logging.LogEntry
		if err := json.Unmarshal(v, &entry); err == nil {
			return entry, nil
		}
		return logging.LogEntry{Timestamp: time.Now().UTC(), Message: string(v)}, nil
	case string:
		var entry logging.LogEntry
		if err := json.Unmarshal([]byte(v), &entry); err == nil {
			return entry, nil
		}
		return logging.LogEntry{Timestamp: time.Now().UTC(), Message: v}, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return logging.LogEntry{}, err
		}
		var entry logging.LogEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return logging.LogEntry{Timestamp: time.Now().UTC(), Message: string(data)}, nil
		}
		return entry, nil
	}
}

// MergeAcceleratorAdapter executes OPTIMIZE TABLE background merges.
type MergeAcceleratorAdapter struct {
	client *Client
}

// NewMergeAccelerator creates a new MergeAcceleratorAdapter.
func NewMergeAccelerator(client *Client) *MergeAcceleratorAdapter {
	return &MergeAcceleratorAdapter{client: client}
}

// ScheduleMerge executes OPTIMIZE TABLE on the target table.
func (m *MergeAcceleratorAdapter) ScheduleMerge(ctx context.Context, database, table string) error {
	if m.client == nil {
		return nil
	}
	conn, err := m.client.Conn(ctx)
	if err != nil {
		return nil
	}

	sanitizedTable := sanitizeIdentifier(table)
	if sanitizedTable == "" {
		sanitizedTable = "cluster_logs"
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return conn.Exec(queryCtx, fmt.Sprintf("OPTIMIZE TABLE %s", sanitizedTable))
}

func sanitizeIdentifier(name string) string {
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

const defaultMaxSpillFileSize = 50 * 1024 * 1024

// DiskSpillJournal writes spilled records in compact JSONL format to local disk.
type DiskSpillJournal struct {
	mu          sync.Mutex
	dir         string
	maxFileSize int64
}

// NewDiskSpillJournal constructs a new DiskSpillJournal.
func NewDiskSpillJournal(spillDir string) *DiskSpillJournal {
	if spillDir == "" {
		spillDir = os.TempDir()
	}
	return &DiskSpillJournal{
		dir:         spillDir,
		maxFileSize: defaultMaxSpillFileSize,
	}
}

// Spill appends records in compact JSONL format to a local spill file with bounded size guard.
func (s *DiskSpillJournal) Spill(ctx context.Context, database, table string, records []any) error {
	if len(records) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0750); err != nil {
		return fmt.Errorf("creating spill directory: %w", err)
	}

	sanitized := sanitizeIdentifier(table)
	if sanitized == "" {
		sanitized = "cluster_logs"
	}
	path := filepath.Join(s.dir, fmt.Sprintf("ingest_spill_%s.jsonl", sanitized))

	info, err := os.Stat(path)
	if err == nil && info.Size() >= s.maxFileSize {
		backupPath := fmt.Sprintf("%s.%d.bak", path, time.Now().UnixNano())
		_ = os.Rename(path, backupPath)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("opening spill journal file %s: %w", path, err)
	}
	defer f.Close()

	for _, rec := range records {
		line, err := json.Marshal(rec)
		if err != nil {
			continue
		}
		line = append(line, '\n')
		if _, err := f.Write(line); err != nil {
			return fmt.Errorf("writing to spill journal file: %w", err)
		}
	}

	return nil
}
