package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// EnterpriseMigrations holds the list of sequential idempotent DDL statements.
var EnterpriseMigrations = []string{
	`CREATE TABLE IF NOT EXISTS cluster_logs (
		timestamp DateTime64(3, 'UTC') CODEC(DoubleDelta, ZSTD(1)),
		tenant_id LowCardinality(String),
		cluster_id LowCardinality(String),
		namespace LowCardinality(String),
		pod_name String,
		container_name LowCardinality(String),
		stream LowCardinality(String),
		log_level LowCardinality(String),
		message String CODEC(ZSTD(3)),
		attributes Map(String, String) CODEC(ZSTD(1)),
		INDEX idx_msg message TYPE tokenbf_v1(30720, 2, 0) GRANULARITY 1
	) ENGINE = MergeTree
	PARTITION BY toYYYYMMDD(timestamp)
	ORDER BY (tenant_id, cluster_id, namespace, timestamp, log_level)
	TTL timestamp + INTERVAL 30 DAY DELETE;`,
	`ALTER TABLE cluster_logs ADD COLUMN IF NOT EXISTS trace_id LowCardinality(String);`,
	`ALTER TABLE cluster_logs ADD COLUMN IF NOT EXISTS span_id String;`,
	`ALTER TABLE cluster_logs ADD COLUMN IF NOT EXISTS error_fingerprint LowCardinality(String);`,
	`ALTER TABLE cluster_logs ADD INDEX IF NOT EXISTS idx_ngram message TYPE ngrambf_v1(4, 32768, 2, 0) GRANULARITY 1;`,
	`ALTER TABLE cluster_logs ADD INDEX IF NOT EXISTS idx_trace trace_id TYPE bloom_filter(0.01) GRANULARITY 1;`,
}

// RunMigrations executes idempotent DDL statements safely on startup.
func RunMigrations(ctx context.Context, client driver.Conn) error {
	if client == nil {
		return errors.New("clickhouse connection is nil for migrations")
	}

	for _, ddl := range EnterpriseMigrations {
		stmt := strings.TrimSpace(ddl)
		if stmt == "" {
			continue
		}
		if err := client.Exec(ctx, stmt); err != nil {
			// If column or index already exists or alter error occurred, check if benign
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "column already exists") {
				continue
			}
			return fmt.Errorf("failed executing migration statement [%s]: %w", stmt, err)
		}
	}

	return nil
}
