package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type nodeMetricsRepo struct {
	pool DBTX
}

// NewNodeMetricsRepo creates a new PostgreSQL-backed Node Metrics repository.
func NewNodeMetricsRepo(pool DBTX) nodemetrics.Repository {
	repo := &nodeMetricsRepo{pool: pool}
	if pool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = repo.ensureSchema(ctx)
	}
	return repo
}

func (r *nodeMetricsRepo) ensureSchema(ctx context.Context) error {
	schemaSQL := `
	CREATE TABLE IF NOT EXISTS node_metric_rollups (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		tenant_id UUID NOT NULL,
		node_id TEXT NOT NULL,
		node_name TEXT NOT NULL,
		cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
		cpu_peak DOUBLE PRECISION NOT NULL DEFAULT 0,
		mem_used_bytes BIGINT NOT NULL DEFAULT 0,
		mem_total_bytes BIGINT NOT NULL DEFAULT 0,
		mem_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
		disk_used_bytes BIGINT NOT NULL DEFAULT 0,
		disk_total_bytes BIGINT NOT NULL DEFAULT 0,
		disk_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
		rx_bytes_per_sec BIGINT NOT NULL DEFAULT 0,
		tx_bytes_per_sec BIGINT NOT NULL DEFAULT 0,
		process_count INT NOT NULL DEFAULT 0,
		container_count INT NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'online',
		resolution TEXT NOT NULL DEFAULT '1m',
		recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_node_rollups_lookup ON node_metric_rollups(node_id, resolution, recorded_at DESC);
	CREATE INDEX IF NOT EXISTS idx_node_rollups_tenant ON node_metric_rollups(tenant_id, recorded_at DESC);
	CREATE INDEX IF NOT EXISTS idx_node_rollups_recorded_at ON node_metric_rollups(recorded_at DESC);
	`
	_, err := r.pool.Exec(ctx, schemaSQL)
	return err
}

func (r *nodeMetricsRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *nodeMetricsRepo) InsertBatch(ctx context.Context, rollups []nodemetrics.NodeMetricRollup) error {
	if len(rollups) == 0 {
		return nil
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	defaultTenantUUID := "00000000-0000-0000-0000-000000000001"
	if tenantID == "" {
		tenantID = defaultTenantUUID
	}

	query := `
		INSERT INTO node_metric_rollups (
			id, tenant_id, node_id, node_name,
			cpu_percent, cpu_peak, mem_used_bytes, mem_total_bytes, mem_percent,
			disk_used_bytes, disk_total_bytes, disk_percent,
			rx_bytes_per_sec, tx_bytes_per_sec, process_count, container_count,
			status, resolution, recorded_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
		)
	`
	db := r.getDB(ctx)
	for _, nr := range rollups {
		id := nr.ID
		if id == "" {
			id = uuid.NewString()
		}
		tID := nr.TenantID
		if tID == "" {
			tID = tenantID
		}
		tUUID, err := uuid.Parse(tID)
		if err != nil {
			tUUID = uuid.MustParse(defaultTenantUUID)
		}
		recAt := nr.RecordedAt
		if recAt.IsZero() {
			recAt = time.Now().UTC()
		}
		res := nr.Resolution
		if res == "" {
			res = "1m"
		}
		stat := nr.Status
		if stat == "" {
			stat = "online"
		}

		_, err = db.Exec(ctx, query,
			id, tUUID, nr.NodeID, nr.NodeName,
			nr.CPUPercent, nr.CPUPeak, nr.MemUsedBytes, nr.MemTotalBytes, nr.MemPercent,
			nr.DiskUsedBytes, nr.DiskTotalBytes, nr.DiskPercent,
			nr.RxBytesPerSec, nr.TxBytesPerSec, nr.ProcessCount, nr.ContainerCount,
			stat, res, recAt,
		)
		if err != nil {
			return fmt.Errorf("inserting node metric rollup: %w", err)
		}
	}
	return nil
}

func (r *nodeMetricsRepo) QueryHistory(ctx context.Context, q nodemetrics.NodeHistoryQuery) ([]nodemetrics.NodeMetricRollup, error) {
	db := r.getDB(ctx)

	resolution := q.Resolution
	if resolution == "" {
		resolution = "1m"
	}
	limit := q.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1500
	}

	query := `
		SELECT id, tenant_id::text, node_id, node_name,
		       cpu_percent, cpu_peak, mem_used_bytes, mem_total_bytes, mem_percent,
		       disk_used_bytes, disk_total_bytes, disk_percent,
		       rx_bytes_per_sec, tx_bytes_per_sec, process_count, container_count,
		       status, resolution, recorded_at
		FROM node_metric_rollups
		WHERE (node_id = $1 OR LOWER(node_name) = LOWER($1))
		  AND resolution = $2
		  AND recorded_at >= $3
		  AND recorded_at <= $4
		ORDER BY recorded_at ASC
		LIMIT $5
	`
	startTime := q.StartTime
	if startTime.IsZero() {
		startTime = time.Now().UTC().Add(-24 * time.Hour)
	}
	endTime := q.EndTime
	if endTime.IsZero() {
		endTime = time.Now().UTC()
	}

	rows, err := db.Query(ctx, query, q.NodeID, resolution, startTime, endTime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying node history: %w", err)
	}
	defer rows.Close()

	var list []nodemetrics.NodeMetricRollup
	for rows.Next() {
		var nr nodemetrics.NodeMetricRollup
		if err := rows.Scan(
			&nr.ID, &nr.TenantID, &nr.NodeID, &nr.NodeName,
			&nr.CPUPercent, &nr.CPUPeak, &nr.MemUsedBytes, &nr.MemTotalBytes, &nr.MemPercent,
			&nr.DiskUsedBytes, &nr.DiskTotalBytes, &nr.DiskPercent,
			&nr.RxBytesPerSec, &nr.TxBytesPerSec, &nr.ProcessCount, &nr.ContainerCount,
			&nr.Status, &nr.Resolution, &nr.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning node metric rollup: %w", err)
		}
		list = append(list, nr)
	}

	return list, nil
}

func (r *nodeMetricsRepo) GetSummary(ctx context.Context, q nodemetrics.NodeHistoryQuery) (*nodemetrics.NodeHistoricalSummary, error) {
	db := r.getDB(ctx)

	startTime := q.StartTime
	if startTime.IsZero() {
		startTime = time.Now().UTC().Add(-24 * time.Hour)
	}
	endTime := q.EndTime
	if endTime.IsZero() {
		endTime = time.Now().UTC()
	}

	query := `
		SELECT 
			COALESCE(node_name, $1),
			COALESCE(AVG(cpu_percent), 0),
			COALESCE(MAX(cpu_peak), 0),
			COALESCE(AVG(mem_percent), 0),
			COALESCE(MAX(mem_percent), 0),
			COALESCE(MAX(rx_bytes_per_sec), 0),
			COALESCE(MAX(tx_bytes_per_sec), 0),
			COUNT(*),
			COALESCE(SUM(CASE WHEN status != 'online' THEN 1 ELSE 0 END), 0)
		FROM node_metric_rollups
		WHERE (node_id = $1 OR LOWER(node_name) = LOWER($1))
		  AND recorded_at >= $2
		  AND recorded_at <= $3
		GROUP BY node_name
		LIMIT 1
	`
	var nodeName string
	var avgCPU, peakCPU, avgMem, peakMem float64
	var peakRx, peakTx int64
	var totalSamples, offlineCount int

	err := db.QueryRow(ctx, query, q.NodeID, startTime, endTime).Scan(
		&nodeName, &avgCPU, &peakCPU, &avgMem, &peakMem, &peakRx, &peakTx, &totalSamples, &offlineCount,
	)
	if err != nil {
		return &nodemetrics.NodeHistoricalSummary{
			NodeID:      q.NodeID,
			NodeName:    q.NodeID,
			WindowStart: startTime,
			WindowEnd:   endTime,
		}, nil
	}

	uptimePercent := 100.0
	if totalSamples > 0 {
		uptimePercent = float64(totalSamples-offlineCount) / float64(totalSamples) * 100.0
	}

	return &nodemetrics.NodeHistoricalSummary{
		NodeID:         q.NodeID,
		NodeName:       nodeName,
		AvgCPUPercent:  avgCPU,
		PeakCPUPercent: peakCPU,
		AvgMemPercent:  avgMem,
		PeakMemPercent: peakMem,
		PeakRxBytesSec: peakRx,
		PeakTxBytesSec: peakTx,
		UptimePercent:  uptimePercent,
		OfflineCount:   offlineCount,
		TotalSamples:   totalSamples,
		WindowStart:    startTime,
		WindowEnd:      endTime,
	}, nil
}

func (r *nodeMetricsRepo) DownsampleAndPrune(ctx context.Context, olderThan7Days, olderThan90Days time.Time) error {
	db := r.getDB(ctx)

	// 1. Rollup 1m data older than 7 days into 1h averages
	rollupQuery := `
		INSERT INTO node_metric_rollups (
			tenant_id, node_id, node_name,
			cpu_percent, cpu_peak, mem_used_bytes, mem_total_bytes, mem_percent,
			disk_used_bytes, disk_total_bytes, disk_percent,
			rx_bytes_per_sec, tx_bytes_per_sec, process_count, container_count,
			status, resolution, recorded_at
		)
		SELECT 
			tenant_id, node_id, node_name,
			AVG(cpu_percent), MAX(cpu_peak), AVG(mem_used_bytes)::bigint, AVG(mem_total_bytes)::bigint, AVG(mem_percent),
			AVG(disk_used_bytes)::bigint, AVG(disk_total_bytes)::bigint, AVG(disk_percent),
			AVG(rx_bytes_per_sec)::bigint, AVG(tx_bytes_per_sec)::bigint, AVG(process_count)::int, AVG(container_count)::int,
			'online', '1h', date_trunc('hour', recorded_at) as hour_bucket
		FROM node_metric_rollups
		WHERE resolution = '1m' AND recorded_at < $1
		GROUP BY tenant_id, node_id, node_name, hour_bucket
	`
	_, _ = db.Exec(ctx, rollupQuery, olderThan7Days)

	// 2. Delete raw 1m data older than 7 days
	prune1mQuery := `DELETE FROM node_metric_rollups WHERE resolution = '1m' AND recorded_at < $1`
	if _, err := db.Exec(ctx, prune1mQuery, olderThan7Days); err != nil {
		return fmt.Errorf("pruning 1m rollups: %w", err)
	}

	// 3. Delete any rollups older than 90 days
	prune90dQuery := `DELETE FROM node_metric_rollups WHERE recorded_at < $1`
	if _, err := db.Exec(ctx, prune90dQuery, olderThan90Days); err != nil {
		return fmt.Errorf("pruning 90d rollups: %w", err)
	}

	return nil
}
