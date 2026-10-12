package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/database"
)

// DBExecutor represents the interface for executing PostgreSQL statements and queries.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ClusterDBResolver resolves a DBExecutor for a specific cluster and database name.
type ClusterDBResolver func(ctx context.Context, clusterID, dbName string) (DBExecutor, error)

// PostgresSREManager handles deep database SRE inspections and autonomous remediations.
type PostgresSREManager struct {
	defaultDB DBExecutor
	resolver  ClusterDBResolver
	logger    *zap.Logger
}

// NewPostgresSREManager creates a new PostgresSREManager instance.
func NewPostgresSREManager(defaultDB DBExecutor, resolver ClusterDBResolver, logger *zap.Logger) *PostgresSREManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &PostgresSREManager{
		defaultDB: defaultDB,
		resolver:  resolver,
		logger:    logger,
	}
}

func (m *PostgresSREManager) getDB(ctx context.Context, clusterID, dbName string) (DBExecutor, error) {
	if m.resolver != nil {
		db, err := m.resolver(ctx, clusterID, dbName)
		if err == nil && db != nil {
			return db, nil
		}
	}
	if m.defaultDB != nil {
		return m.defaultDB, nil
	}
	return nil, fmt.Errorf("no database connection available for cluster %q, database %q", clusterID, dbName)
}

// InspectIdleTransactions queries pg_stat_activity to find connections in 'idle in transaction' > 10s.
func (m *PostgresSREManager) InspectIdleTransactions(ctx context.Context, clusterID, dbName string) ([]database.IdleTransactionRecord, error) {
	db, err := m.getDB(ctx, clusterID, dbName)
	if err != nil {
		return nil, fmt.Errorf("resolving database connection: %w", err)
	}

	const query = `
		SELECT pid,
		       COALESCE(datname, ''),
		       COALESCE(usename, ''),
		       COALESCE(client_addr::text, ''),
		       COALESCE(application_name, ''),
		       COALESCE(EXTRACT(EPOCH FROM (NOW() - xact_start)), 0)::float8 AS duration_sec,
		       COALESCE(EXTRACT(EPOCH FROM (NOW() - state_change)), 0)::float8 AS state_change_sec,
		       COALESCE(query, '')
		FROM pg_stat_activity
		WHERE state = 'idle in transaction'
		  AND (NOW() - state_change) > INTERVAL '10 seconds'
		  AND ($1 = '' OR datname = $1)
		ORDER BY state_change ASC`

	rows, err := db.Query(ctx, query, dbName)
	if err != nil {
		return nil, fmt.Errorf("querying idle transactions: %w", err)
	}
	defer rows.Close()

	records := make([]database.IdleTransactionRecord, 0)
	for rows.Next() {
		var (
			pid            int
			databaseName   string
			username       string
			clientIP       string
			appName        string
			durationSec    float64
			stateChangeSec float64
			currentQuery   string
		)

		if err := rows.Scan(
			&pid,
			&databaseName,
			&username,
			&clientIP,
			&appName,
			&durationSec,
			&stateChangeSec,
			&currentQuery,
		); err != nil {
			return nil, fmt.Errorf("scanning idle transaction row: %w", err)
		}

		duration := time.Duration(durationSec * float64(time.Second))
		stateChangeAge := time.Duration(stateChangeSec * float64(time.Second))

		rec, err := database.NewIdleTransactionRecord(
			pid,
			databaseName,
			username,
			clientIP,
			appName,
			duration,
			stateChangeAge,
			currentQuery,
		)
		if err != nil {
			m.logger.Warn("skipping invalid idle transaction record", zap.Int("pid", pid), zap.Error(err))
			continue
		}

		records = append(records, *rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating idle transaction rows: %w", err)
	}

	return records, nil
}

// TerminateIdleTransaction terminates a lingering idle-in-transaction connection while strictly protecting active transactions.
func (m *PostgresSREManager) TerminateIdleTransaction(ctx context.Context, clusterID, dbName string, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("%w: PID must be positive, got %d", database.ErrDatabaseSREInvalidInput, pid)
	}

	db, err := m.getDB(ctx, clusterID, dbName)
	if err != nil {
		return fmt.Errorf("resolving database connection: %w", err)
	}

	// Guardrail: verify the transaction state first. Refuse to terminate active transactions.
	const checkQuery = `SELECT state, COALESCE(query, '') FROM pg_stat_activity WHERE pid = $1`
	var state, queryText string
	err = db.QueryRow(ctx, checkQuery, pid).Scan(&state, &queryText)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.ErrTransactionNotFound
		}
		return fmt.Errorf("verifying transaction state for pid %d: %w", pid, err)
	}

	if state != "idle in transaction" {
		m.logger.Warn("refusing to terminate transaction: state is not idle in transaction",
			zap.Int("pid", pid),
			zap.String("state", state),
			zap.String("query", queryText),
		)
		return database.ErrActiveTransactionProtected
	}

	// Terminate the backend connection
	const termQuery = `SELECT pg_terminate_backend($1)`
	var terminated bool
	err = db.QueryRow(ctx, termQuery, pid).Scan(&terminated)
	if err != nil {
		return fmt.Errorf("terminating backend pid %d: %w", pid, err)
	}

	if !terminated {
		return fmt.Errorf("failed to terminate backend %d: pg_terminate_backend returned false", pid)
	}

	m.logger.Info("terminated idle-in-transaction backend",
		zap.Int("pid", pid),
		zap.String("cluster", clusterID),
		zap.String("database", dbName),
	)

	return nil
}

// AnalyzeTableBloat calculates bloat ratios and recommends vacuum/repack actions.
func (m *PostgresSREManager) AnalyzeTableBloat(ctx context.Context, clusterID, dbName string) ([]database.TableBloatReport, error) {
	db, err := m.getDB(ctx, clusterID, dbName)
	if err != nil {
		return nil, fmt.Errorf("resolving database connection: %w", err)
	}

	const query = `
		SELECT relname::text,
		       COALESCE(pg_total_relation_size(relid), 0)::int8 AS total_bytes,
		       COALESCE(n_dead_tup, 0)::int8 AS dead_tuples,
		       COALESCE(n_live_tup, 0)::int8 AS live_tuples
		FROM pg_stat_user_tables
		WHERE (n_live_tup + n_dead_tup) > 0
		ORDER BY n_dead_tup DESC`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying table bloat statistics: %w", err)
	}
	defer rows.Close()

	reports := make([]database.TableBloatReport, 0)
	for rows.Next() {
		var (
			tableName  string
			totalBytes int64
			deadTuples int64
			liveTuples int64
		)

		if err := rows.Scan(&tableName, &totalBytes, &deadTuples, &liveTuples); err != nil {
			return nil, fmt.Errorf("scanning table bloat row: %w", err)
		}

		report, err := database.NewTableBloatReport(tableName, totalBytes, deadTuples, liveTuples)
		if err != nil {
			m.logger.Warn("skipping invalid table bloat report", zap.String("table", tableName), zap.Error(err))
			continue
		}

		reports = append(reports, *report)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating table bloat rows: %w", err)
	}

	return reports, nil
}
