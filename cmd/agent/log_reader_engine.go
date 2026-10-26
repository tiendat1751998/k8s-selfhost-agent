package main

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// EngineLogSource wraps the embedded columnar log engine as a LogSource.
type EngineLogSource struct {
	writer    *logengine.Writer
	reader    *logengine.Reader
	compactor *logengine.Compactor
	dict      *logengine.LabelDictionary
}

// NewEngineLogSource creates a new EngineLogSource backed by logengine.
func NewEngineLogSource(engineDir string) (*EngineLogSource, error) {
	dict := logengine.NewLabelDictionary()
	maxDiskBytes := int64(512 * 1024 * 1024)
	if envVal := os.Getenv("AGENT_LOG_MAX_DISK_MB"); envVal != "" {
		if mb, err := strconv.Atoi(envVal); err == nil && mb > 0 {
			maxDiskBytes = int64(mb) * 1024 * 1024
		}
	}
	w, err := logengine.NewWriter(engineDir, dict,
		logengine.WithMaxRetention(7*24*time.Hour),
		logengine.WithMaxDiskBytes(maxDiskBytes),
	)
	if err != nil {
		return nil, err
	}
	r, err := logengine.NewReader(engineDir, dict, logengine.WithLiveWriter(w))
	if err != nil {
		_ = w.Close()
		return nil, err
	}

	compactInterval := 30 * time.Second
	if envVal := os.Getenv("AGENT_LOG_COMPACT_INTERVAL_SEC"); envVal != "" {
		if sec, err := strconv.Atoi(envVal); err == nil && sec > 0 {
			compactInterval = time.Duration(sec) * time.Second
		}
	}
	partsDir := filepath.Join(engineDir, "parts")
	compactor := logengine.NewCompactor(partsDir, w, r)
	compactor.Start(compactInterval)

	return &EngineLogSource{writer: w, reader: r, compactor: compactor, dict: dict}, nil
}

func (e *EngineLogSource) Writer() *logengine.Writer       { return e.writer }
func (e *EngineLogSource) Reader() *logengine.Reader       { return e.reader }
func (e *EngineLogSource) Compactor() *logengine.Compactor { return e.compactor }

func (e *EngineLogSource) Close() error {
	var compactorErr error
	if e.compactor != nil {
		compactorErr = e.compactor.Close()
	}
	var writerErr error
	if e.writer != nil {
		writerErr = e.writer.Close()
	}
	if compactorErr != nil {
		return compactorErr
	}
	return writerErr
}


// BlockCount returns total indexed blocks in the embedded columnar log engine.
func (e *EngineLogSource) BlockCount() int {
	if e == nil || e.reader == nil {
		return 0
	}
	return e.reader.BlockCount()
}

// HasBlocks returns true if the embedded columnar log engine has any blocks.
func (e *EngineLogSource) HasBlocks() bool {
	if e == nil || e.reader == nil {
		return false
	}
	return e.reader.HasBlocks()
}

func (e *EngineLogSource) ListServices(_ context.Context) ([]string, error) {
	if e.dict == nil {
		return nil, nil
	}
	return e.dict.Export(), nil
}

func (e *EngineLogSource) GetLogs(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string) ([]LogEntry, error) {
	return e.GetLogsWithK8s(ctx, app, tail, sinceTime, untilTime, query, level, "", "", "", "")
}

// GetLogsWithK8s executes search with full K8s metadata pruning before tail truncation.
func (e *EngineLogSource) GetLogsWithK8s(ctx context.Context, app string, tail int, sinceTime, untilTime *time.Time, query string, level string, ns, pod, cont, stream string) ([]LogEntry, error) {
	if e.reader == nil {
		return nil, nil
	}
	var since, until time.Time
	if sinceTime != nil {
		since = *sinceTime
	}
	if untilTime != nil {
		until = *untilTime
	}

	params := logengine.QueryParams{
		Service:       app,
		Level:         level,
		Query:         query,
		Since:         since,
		Until:         until,
		Limit:         tail,
		Namespace:     ns,
		PodName:       pod,
		ContainerName: cont,
		Stream:        stream,
	}

	trimmedQuery := strings.TrimSpace(query)
	if len(trimmedQuery) >= 2 && trimmedQuery[0] == '"' && trimmedQuery[len(trimmedQuery)-1] == '"' {
		params.ExactMatch = true
		params.Query = trimmedQuery[1 : len(trimmedQuery)-1]
	}

	entries, err := e.reader.Search(ctx, params)
	if err != nil {
		return nil, err
	}
	results := make([]LogEntry, len(entries))
	for i, en := range entries {
		le := LogEntry{
			Timestamp:     en.Timestamp,
			Service:       en.Service,
			Level:         en.Level,
			Message:       en.Message,
			Namespace:     en.Namespace,
			PodName:       en.PodName,
			ContainerName: en.ContainerName,
			Stream:        en.Stream,
		}
		le.Raw = formatLogEntry(le)
		results[i] = le
	}
	return results, nil
}
