package database

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Domain error definitions for database SRE operations.
var (
	// ErrIdleTransactionTerminated is returned when an idle-in-transaction connection is terminated.
	ErrIdleTransactionTerminated = errors.New("transaction terminated by SRE Watchdog: Idle in transaction exceeded threshold (10s)")
	// ErrTableBloatCritical is returned when table bloat exceeds acceptable thresholds.
	ErrTableBloatCritical = errors.New("table bloat critical: vacuum or pg_repack required")
	// ErrActiveTransactionProtected is returned when attempting to terminate an active transaction.
	ErrActiveTransactionProtected = errors.New("cannot terminate active transaction: only idle in transaction is permitted")
	// ErrTransactionNotFound is returned when the target transaction PID cannot be found.
	ErrTransactionNotFound = errors.New("transaction not found")
	// ErrDatabaseSREInvalidInput is returned when input parameters are invalid.
	ErrDatabaseSREInvalidInput = errors.New("invalid SRE input parameters")
)

// IdleTransactionRecord represents a database connection lingering in 'idle in transaction' state.
type IdleTransactionRecord struct {
	PID             int           `json:"pid"`
	Database        string        `json:"database"`
	Username        string        `json:"username"`
	ClientIP        string        `json:"client_ip"`
	ApplicationName string        `json:"application_name"`
	Duration        time.Duration `json:"duration"`
	StateChangeAge  time.Duration `json:"state_change_age"`
	CurrentQuery    string        `json:"current_query"`
}

// NewIdleTransactionRecord creates and validates a new IdleTransactionRecord.
func NewIdleTransactionRecord(
	pid int,
	databaseName string,
	username string,
	clientIP string,
	applicationName string,
	duration time.Duration,
	stateChangeAge time.Duration,
	currentQuery string,
) (*IdleTransactionRecord, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("%w: PID must be positive, got %d", ErrDatabaseSREInvalidInput, pid)
	}
	if stateChangeAge < 0 {
		stateChangeAge = 0
	}
	if duration < 0 {
		duration = 0
	}

	return &IdleTransactionRecord{
		PID:             pid,
		Database:        databaseName,
		Username:        username,
		ClientIP:        clientIP,
		ApplicationName: applicationName,
		Duration:        duration,
		StateChangeAge:  stateChangeAge,
		CurrentQuery:    currentQuery,
	}, nil
}

// IsStale returns true if the transaction has been idle longer than the specified threshold.
func (r *IdleTransactionRecord) IsStale(threshold time.Duration) bool {
	return r.StateChangeAge >= threshold
}

// TableBloatReport contains dead tuple and bloat analysis for a PostgreSQL relation.
type TableBloatReport struct {
	TableName       string  `json:"table_name"`
	TotalSizeBytes  int64   `json:"total_size_bytes"`
	BloatSizeBytes  int64   `json:"bloat_size_bytes"`
	BloatPercentage float64 `json:"bloat_percentage"`
	DeadTuples      int64   `json:"dead_tuples"`
	LiveTuples      int64   `json:"live_tuples"`
	RecommendedFix  string  `json:"recommended_fix"` // VACUUM FULL (locking), pg_repack (online)
}

// NewTableBloatReport creates and calculates bloat metrics for a PostgreSQL relation.
func NewTableBloatReport(
	tableName string,
	totalSizeBytes int64,
	deadTuples int64,
	liveTuples int64,
) (*TableBloatReport, error) {
	if tableName == "" {
		return nil, fmt.Errorf("%w: table name cannot be empty", ErrDatabaseSREInvalidInput)
	}
	if deadTuples < 0 {
		deadTuples = 0
	}
	if liveTuples < 0 {
		liveTuples = 0
	}
	if totalSizeBytes < 0 {
		totalSizeBytes = 0
	}

	totalTuples := deadTuples + liveTuples
	var bloatPercentage float64
	if totalTuples > 0 {
		bloatPercentage = (float64(deadTuples) * 100.0) / float64(totalTuples)
	}

	bloatSizeBytes := int64(float64(totalSizeBytes) * (bloatPercentage / 100.0))

	var recommendedFix string
	// If bloat > 30% and size > 1GB, online reorganization via pg_repack is strongly recommended
	if bloatPercentage > 30.0 && totalSizeBytes > 1024*1024*1024 {
		recommendedFix = fmt.Sprintf("pg_repack --table %s (online reorganization without table lock)", tableName)
	} else if bloatPercentage > 20.0 {
		recommendedFix = fmt.Sprintf("VACUUM FULL %s (locking) or pg_repack (online)", tableName)
	} else {
		recommendedFix = fmt.Sprintf("VACUUM ANALYZE %s", tableName)
	}

	return &TableBloatReport{
		TableName:       tableName,
		TotalSizeBytes:  totalSizeBytes,
		BloatSizeBytes:  bloatSizeBytes,
		BloatPercentage: bloatPercentage,
		DeadTuples:      deadTuples,
		LiveTuples:      liveTuples,
		RecommendedFix:  recommendedFix,
	}, nil
}

// IsCritical returns true if table bloat is critical (> 30% and size > 1GB).
func (r *TableBloatReport) IsCritical() bool {
	return r.BloatPercentage > 30.0 && r.TotalSizeBytes > 1024*1024*1024
}

// RedisFragmentationStatus contains Redis memory metrics and active defrag state.
type RedisFragmentationStatus struct {
	UsedMemoryBytes     int64   `json:"used_memory_bytes"`
	RSSMemoryBytes      int64   `json:"rss_memory_bytes"`
	FragmentationRatio  float64 `json:"fragmentation_ratio"` // RSS / Used
	ActiveDefragEnabled bool    `json:"active_defrag_enabled"`
	DefragRunning       bool    `json:"defrag_running"`
}

// NewRedisFragmentationStatus creates and validates Redis fragmentation status.
func NewRedisFragmentationStatus(
	usedMemoryBytes int64,
	rssMemoryBytes int64,
	activeDefragEnabled bool,
	defragRunning bool,
) (*RedisFragmentationStatus, error) {
	if usedMemoryBytes < 0 || rssMemoryBytes < 0 {
		return nil, fmt.Errorf("%w: memory metrics cannot be negative", ErrDatabaseSREInvalidInput)
	}

	var ratio float64
	if usedMemoryBytes > 0 {
		ratio = float64(rssMemoryBytes) / float64(usedMemoryBytes)
	}

	return &RedisFragmentationStatus{
		UsedMemoryBytes:     usedMemoryBytes,
		RSSMemoryBytes:      rssMemoryBytes,
		FragmentationRatio:  ratio,
		ActiveDefragEnabled: activeDefragEnabled,
		DefragRunning:       defragRunning,
	}, nil
}

// NeedsDefrag returns true if fragmentation ratio exceeds 1.4 and defrag is not yet enabled.
func (s *RedisFragmentationStatus) NeedsDefrag() bool {
	return s.FragmentationRatio > 1.4 && !s.ActiveDefragEnabled
}

// ShouldDisableDefrag returns true if fragmentation ratio has dropped below 1.15 and defrag is active.
func (s *RedisFragmentationStatus) ShouldDisableDefrag() bool {
	return s.FragmentationRatio < 1.15 && s.ActiveDefragEnabled
}

// DatabaseSREManager defines the port interface for database reliability and anti-bloat operations.
type DatabaseSREManager interface {
	InspectIdleTransactions(ctx context.Context, clusterID, dbName string) ([]IdleTransactionRecord, error)
	TerminateIdleTransaction(ctx context.Context, clusterID, dbName string, pid int) error
	AnalyzeTableBloat(ctx context.Context, clusterID, dbName string) ([]TableBloatReport, error)
	GetRedisFragmentation(ctx context.Context, clusterID string) (*RedisFragmentationStatus, error)
	ToggleRedisActiveDefrag(ctx context.Context, clusterID string, enable bool) error
}
