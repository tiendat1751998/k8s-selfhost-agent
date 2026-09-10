package logging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Domain errors for logging domain.
var (
	ErrInvalidLogQuery    = errors.New("invalid log query")
	ErrLogIngestionFailed = errors.New("log ingestion failed")
	ErrLogNotFound        = errors.New("log entry not found")
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	return string(l)
}

// IsValid checks whether the LogLevel is one of the supported levels.
func (l LogLevel) IsValid() bool {
	switch l {
	case LogLevelTrace, LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelFatal:
		return true
	default:
		return false
	}
}

// ParseLogLevel converts a string to a typed LogLevel.
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LogLevelTrace, nil
	case "debug":
		return LogLevelDebug, nil
	case "info":
		return LogLevelInfo, nil
	case "warn", "warning":
		return LogLevelWarn, nil
	case "error":
		return LogLevelError, nil
	case "fatal":
		return LogLevelFatal, nil
	default:
		return "", fmt.Errorf("%w: unrecognized log level '%s'", ErrInvalidLogQuery, s)
	}
}

// LogEntry represents an immutable streaming log event.
type LogEntry struct {
	Timestamp     time.Time         `json:"timestamp"`
	TenantID      string            `json:"tenant_id"`
	ClusterID     string            `json:"cluster_id"`
	Namespace     string            `json:"namespace"`
	PodName       string            `json:"pod_name"`
	ContainerName string            `json:"container_name"`
	Stream        string            `json:"stream"`
	LogLevel      LogLevel          `json:"log_level"`
	Message       string            `json:"message"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// Validate ensures all mandatory fields in LogEntry are populated with sensible defaults.
func (e *LogEntry) Validate() error {
	if strings.TrimSpace(e.TenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidLogQuery)
	}
	if strings.TrimSpace(e.ClusterID) == "" {
		return fmt.Errorf("%w: cluster_id is required", ErrInvalidLogQuery)
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if strings.TrimSpace(e.Stream) == "" {
		e.Stream = "stdout"
	}
	if strings.TrimSpace(string(e.LogLevel)) == "" {
		e.LogLevel = LogLevelInfo
	}
	return nil
}

// LogFilter defines query parameters for log searches with sparse index support.
type LogFilter struct {
	TenantID      string            `json:"tenant_id"`
	ClusterID     string            `json:"cluster_id"`
	Namespace     string            `json:"namespace,omitempty"`
	PodName       string            `json:"pod_name,omitempty"`
	ContainerName string            `json:"container_name,omitempty"`
	Stream        string            `json:"stream,omitempty"`
	LogLevel      LogLevel          `json:"log_level,omitempty"`
	SearchText    string            `json:"search_text,omitempty"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	Limit         int               `json:"limit"`
	Offset        int               `json:"offset"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

// Validate checks filter constraints.
func (f *LogFilter) Validate() error {
	if strings.TrimSpace(f.TenantID) == "" {
		return fmt.Errorf("%w: tenant_id is required", ErrInvalidLogQuery)
	}
	if !f.StartTime.IsZero() && !f.EndTime.IsZero() && f.StartTime.After(f.EndTime) {
		return fmt.Errorf("%w: start_time cannot be after end_time", ErrInvalidLogQuery)
	}
	if f.Limit < 0 {
		return fmt.Errorf("%w: limit cannot be negative", ErrInvalidLogQuery)
	}
	if f.Offset < 0 {
		return fmt.Errorf("%w: offset cannot be negative", ErrInvalidLogQuery)
	}
	return nil
}

// Sanitize applies standard bounds: limit clamped between 1 and 100, default time range.
func (f *LogFilter) Sanitize() {
	if f.Limit <= 0 {
		f.Limit = 50
	} else if f.Limit > 100 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.EndTime.IsZero() {
		f.EndTime = time.Now().UTC()
	}
	if f.StartTime.IsZero() {
		f.StartTime = f.EndTime.Add(-1 * time.Hour)
	}
}

// LogSearchResult holds query results with pagination metadata.
type LogSearchResult struct {
	Entries    []LogEntry `json:"entries"`
	TotalCount int64      `json:"total_count"`
	HasMore    bool       `json:"has_more"`
}

// LogAggregationBucket holds aggregated log counts for time buckets.
type LogAggregationBucket struct {
	TimeBucket time.Time         `json:"time_bucket"`
	TotalCount uint64            `json:"total_count"`
	LevelCount map[string]uint64 `json:"level_count,omitempty"`
}

// LogRepository defines the storage interface for streaming cluster logs.
type LogRepository interface {
	IngestBatch(ctx context.Context, entries []LogEntry) error
	QueryLogs(ctx context.Context, filter LogFilter) (*LogSearchResult, error)
	GetHistogram(ctx context.Context, filter LogFilter, intervalSeconds int) ([]LogAggregationBucket, error)
	TailLogs(ctx context.Context, filter LogFilter) (<-chan LogEntry, error)
}
