package main

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// EngineLogSource wraps the embedded columnar log engine as a LogSource.
type EngineLogSource struct {
	writer *logengine.Writer
	reader *logengine.Reader
	dict   *logengine.LabelDictionary
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
	r, err := logengine.NewReader(engineDir, dict)
	if err != nil {
		_ = w.Close()
		return nil, err
	}
	return &EngineLogSource{writer: w, reader: r, dict: dict}, nil
}

func (e *EngineLogSource) Writer() *logengine.Writer { return e.writer }
func (e *EngineLogSource) Reader() *logengine.Reader { return e.reader }

func (e *EngineLogSource) Close() error {
	if e.writer != nil {
		return e.writer.Close()
	}
	return nil
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
		Service: app, Level: level, Query: query, Since: since, Until: until, Limit: tail,
	}

	var entries []logengine.Entry
	var err error

	trimmedQuery := strings.TrimSpace(query)
	hasAlphaNum := false
	if trimmedQuery != "" {
		for i := 0; i < len(trimmedQuery); i++ {
			c := trimmedQuery[i]
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
				hasAlphaNum = true
				break
			}
		}
	}

	if hasAlphaNum {
		params.ExactMatch = true
		entries, err = e.reader.Search(ctx, params)
		if err == nil && len(entries) == 0 {
			params.ExactMatch = false
			entries, err = e.reader.Search(ctx, params)
		}
	} else {
		entries, err = e.reader.Search(ctx, params)
	}

	if err != nil {
		return nil, err
	}
	results := make([]LogEntry, len(entries))
	for i, en := range entries {
		le := LogEntry{Timestamp: en.Timestamp, Service: en.Service, Level: en.Level, Message: en.Message}
		le.Raw = formatLogEntry(le)
		results[i] = le
	}
	return results, nil
}
