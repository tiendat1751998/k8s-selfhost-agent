package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/database"
)

// AuditRecorder defines an interface for recording operational audit logs.
type AuditRecorder interface {
	RecordAction(
		ctx context.Context,
		actor string,
		action string,
		targetType string,
		targetID string,
		targetName string,
		result string,
		details map[string]interface{},
		ipAddress string,
		userAgent string,
	) error
}

// RemediationSummary contains an aggregated operational snapshot of deep SRE actions.
type RemediationSummary struct {
	ClusterID                string                          `json:"cluster_id"`
	DatabaseName             string                          `json:"database_name"`
	TerminatedTransactions   []domain.IdleTransactionRecord  `json:"terminated_transactions"`
	CriticalBloatTables      []domain.TableBloatReport       `json:"critical_bloat_tables"`
	RedisFragmentationStatus *domain.RedisFragmentationStatus `json:"redis_fragmentation_status,omitempty"`
	RedisDefragToggled       bool                            `json:"redis_defrag_toggled"`
	ExecutedAt               time.Time                       `json:"executed_at"`
	DurationMs               int64                           `json:"duration_ms"`
}

// DatabaseSREUsecase orchestrates database reliability policies, safety guardrails, and audit tracking.
type DatabaseSREUsecase struct {
	manager       domain.DatabaseSREManager
	auditRecorder AuditRecorder
	logger        *zap.Logger
}

// NewDatabaseSREUsecase creates a new DatabaseSREUsecase.
func NewDatabaseSREUsecase(
	manager domain.DatabaseSREManager,
	auditRecorder AuditRecorder,
	logger *zap.Logger,
) (*DatabaseSREUsecase, error) {
	if manager == nil {
		return nil, errors.New("database SRE manager is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DatabaseSREUsecase{
		manager:       manager,
		auditRecorder: auditRecorder,
		logger:        logger,
	}, nil
}

// InspectIdleTransactions lists all currently stale connections lingering in 'idle in transaction'.
func (u *DatabaseSREUsecase) InspectIdleTransactions(ctx context.Context, clusterID, dbName string) ([]domain.IdleTransactionRecord, error) {
	return u.manager.InspectIdleTransactions(ctx, clusterID, dbName)
}

// TerminateIdleTransaction terminates an idle transaction after safety checks, creating an audit trail.
func (u *DatabaseSREUsecase) TerminateIdleTransaction(ctx context.Context, clusterID, dbName string, pid int, reason string) error {
	if pid <= 0 {
		return fmt.Errorf("%w: PID must be positive, got %d", domain.ErrDatabaseSREInvalidInput, pid)
	}

	err := u.manager.TerminateIdleTransaction(ctx, clusterID, dbName, pid)
	if err != nil {
		u.logger.Warn("failed to terminate idle transaction",
			zap.String("cluster", clusterID),
			zap.String("database", dbName),
			zap.Int("pid", pid),
			zap.Error(err),
		)
		if u.auditRecorder != nil {
			_ = u.auditRecorder.RecordAction(
				ctx,
				"sre-watchdog",
				"terminate_idle_transaction",
				"database_connection",
				fmt.Sprintf("%d", pid),
				dbName,
				"failed",
				map[string]interface{}{
					"cluster_id": clusterID,
					"database":   dbName,
					"pid":        pid,
					"error":      err.Error(),
					"reason":     reason,
				},
				"127.0.0.1",
				"k8s-sre-engine/1.0",
			)
		}
		return err
	}

	u.logger.Info("successfully terminated idle transaction",
		zap.String("cluster", clusterID),
		zap.String("database", dbName),
		zap.Int("pid", pid),
		zap.String("reason", reason),
	)

	if u.auditRecorder != nil {
		_ = u.auditRecorder.RecordAction(
			ctx,
			"sre-watchdog",
			"terminate_idle_transaction",
			"database_connection",
			fmt.Sprintf("%d", pid),
			dbName,
			"success",
			map[string]interface{}{
				"cluster_id": clusterID,
				"database":   dbName,
				"pid":        pid,
				"reason":     reason,
			},
			"127.0.0.1",
			"k8s-sre-engine/1.0",
		)
	}

	return nil
}

// EvaluateAndTerminateIdleTransactions evaluates all idle transactions and terminates those exceeding threshold.
func (u *DatabaseSREUsecase) EvaluateAndTerminateIdleTransactions(
	ctx context.Context,
	clusterID string,
	dbName string,
	idleThreshold time.Duration,
) ([]domain.IdleTransactionRecord, error) {
	if idleThreshold <= 0 {
		idleThreshold = 10 * time.Second
	}

	idleTxs, err := u.manager.InspectIdleTransactions(ctx, clusterID, dbName)
	if err != nil {
		return nil, fmt.Errorf("inspecting idle transactions: %w", err)
	}

	terminated := make([]domain.IdleTransactionRecord, 0)
	for _, tx := range idleTxs {
		if tx.StateChangeAge >= idleThreshold {
			reason := fmt.Sprintf("idle in transaction duration (%s) exceeded threshold (%s)", tx.StateChangeAge, idleThreshold)
			if termErr := u.TerminateIdleTransaction(ctx, clusterID, dbName, tx.PID, reason); termErr == nil {
				terminated = append(terminated, tx)
			} else {
				u.logger.Warn("skipping idle transaction termination due to error",
					zap.Int("pid", tx.PID),
					zap.Error(termErr),
				)
			}
		}
	}

	return terminated, nil
}

// AnalyzeTableBloat returns all table bloat reports and identifies critical bloat requiring immediate vacuum or pg_repack.
func (u *DatabaseSREUsecase) AnalyzeTableBloat(
	ctx context.Context,
	clusterID string,
	dbName string,
	bloatThreshold float64,
) ([]domain.TableBloatReport, []domain.TableBloatReport, error) {
	if bloatThreshold <= 0 {
		bloatThreshold = 20.0
	}

	reports, err := u.manager.AnalyzeTableBloat(ctx, clusterID, dbName)
	if err != nil {
		return nil, nil, fmt.Errorf("analyzing table bloat: %w", err)
	}

	filtered := make([]domain.TableBloatReport, 0)
	critical := make([]domain.TableBloatReport, 0)

	for _, rep := range reports {
		if rep.BloatPercentage >= bloatThreshold {
			filtered = append(filtered, rep)
		}
		if rep.IsCritical() {
			critical = append(critical, rep)
			u.logger.Warn("critical table bloat detected",
				zap.String("table", rep.TableName),
				zap.Float64("bloat_percentage", rep.BloatPercentage),
				zap.Int64("total_size_bytes", rep.TotalSizeBytes),
				zap.String("recommended_fix", rep.RecommendedFix),
			)
			if u.auditRecorder != nil {
				_ = u.auditRecorder.RecordAction(
					ctx,
					"sre-watchdog",
					"table_bloat_critical",
					"database_table",
					rep.TableName,
					rep.TableName,
					"warning",
					map[string]interface{}{
						"cluster_id":       clusterID,
						"database":         dbName,
						"table_name":       rep.TableName,
						"bloat_percentage": rep.BloatPercentage,
						"total_size_bytes": rep.TotalSizeBytes,
						"recommended_fix":  rep.RecommendedFix,
					},
					"127.0.0.1",
					"k8s-sre-engine/1.0",
				)
			}
		}
	}

	return filtered, critical, nil
}

// EvaluateRedisDefrag inspects Redis fragmentation and auto-toggles active defragmentation.
func (u *DatabaseSREUsecase) EvaluateRedisDefrag(
	ctx context.Context,
	clusterID string,
) (*domain.RedisFragmentationStatus, bool, error) {
	status, err := u.manager.GetRedisFragmentation(ctx, clusterID)
	if err != nil {
		return nil, false, fmt.Errorf("fetching redis fragmentation: %w", err)
	}

	toggled := false
	if status.NeedsDefrag() {
		u.logger.Info("enabling active defragmentation on Redis",
			zap.String("cluster", clusterID),
			zap.Float64("ratio", status.FragmentationRatio),
		)
		if err := u.manager.ToggleRedisActiveDefrag(ctx, clusterID, true); err != nil {
			return status, false, fmt.Errorf("auto-enabling redis active defrag: %w", err)
		}
		status.ActiveDefragEnabled = true
		toggled = true

		if u.auditRecorder != nil {
			_ = u.auditRecorder.RecordAction(
				ctx,
				"sre-watchdog",
				"enable_redis_defrag",
				"redis_cluster",
				clusterID,
				clusterID,
				"success",
				map[string]interface{}{
					"cluster_id":          clusterID,
					"fragmentation_ratio": status.FragmentationRatio,
					"used_memory_bytes":   status.UsedMemoryBytes,
					"rss_memory_bytes":    status.RSSMemoryBytes,
				},
				"127.0.0.1",
				"k8s-sre-engine/1.0",
			)
		}
	} else if status.ShouldDisableDefrag() {
		u.logger.Info("disabling active defragmentation on Redis",
			zap.String("cluster", clusterID),
			zap.Float64("ratio", status.FragmentationRatio),
		)
		if err := u.manager.ToggleRedisActiveDefrag(ctx, clusterID, false); err != nil {
			return status, false, fmt.Errorf("auto-disabling redis active defrag: %w", err)
		}
		status.ActiveDefragEnabled = false
		toggled = true

		if u.auditRecorder != nil {
			_ = u.auditRecorder.RecordAction(
				ctx,
				"sre-watchdog",
				"disable_redis_defrag",
				"redis_cluster",
				clusterID,
				clusterID,
				"success",
				map[string]interface{}{
					"cluster_id":          clusterID,
					"fragmentation_ratio": status.FragmentationRatio,
					"used_memory_bytes":   status.UsedMemoryBytes,
					"rss_memory_bytes":    status.RSSMemoryBytes,
				},
				"127.0.0.1",
				"k8s-sre-engine/1.0",
			)
		}
	}

	return status, toggled, nil
}

// AutoRemediateDatabase coordinates automated SRE maintenance across Postgres and Redis.
func (u *DatabaseSREUsecase) AutoRemediateDatabase(ctx context.Context, clusterID, dbName string) (*RemediationSummary, error) {
	start := time.Now()

	terminatedTxs, err := u.EvaluateAndTerminateIdleTransactions(ctx, clusterID, dbName, 10*time.Second)
	if err != nil {
		u.logger.Warn("non-fatal error evaluating idle transactions during auto remediation", zap.Error(err))
	}

	_, criticalBloat, err := u.AnalyzeTableBloat(ctx, clusterID, dbName, 20.0)
	if err != nil {
		u.logger.Warn("non-fatal error analyzing table bloat during auto remediation", zap.Error(err))
	}

	redisStatus, redisToggled, err := u.EvaluateRedisDefrag(ctx, clusterID)
	if err != nil {
		u.logger.Warn("non-fatal error evaluating redis defrag during auto remediation", zap.Error(err))
	}

	return &RemediationSummary{
		ClusterID:                clusterID,
		DatabaseName:             dbName,
		TerminatedTransactions:   terminatedTxs,
		CriticalBloatTables:      criticalBloat,
		RedisFragmentationStatus: redisStatus,
		RedisDefragToggled:       redisToggled,
		ExecutedAt:               start.UTC(),
		DurationMs:               time.Since(start).Milliseconds(),
	}, nil
}
