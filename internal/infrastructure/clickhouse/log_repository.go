package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
)

// LogRepository implements domain.LogRepository backed by ClickHouse.
type LogRepository struct {
	client    *Client
	writer    *BatchWriter
	tableName string
}

// NewLogRepository creates a new ClickHouse log repository.
func NewLogRepository(client *Client, writer *BatchWriter) *LogRepository {
	return &LogRepository{
		client:    client,
		writer:    writer,
		tableName: "cluster_logs",
	}
}

// IngestBatch validates and persists log entries via BatchWriter or direct batch insert.
func (r *LogRepository) IngestBatch(ctx context.Context, entries []logging.LogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return err
		}
	}
	if r.writer != nil {
		return r.writer.WriteBatch(entries)
	}
	return r.insertBatchDirect(ctx, entries)
}

// insertBatchDirect performs an immediate native batch insert when BatchWriter is omitted.
func (r *LogRepository) insertBatchDirect(ctx context.Context, entries []logging.LogEntry) error {
	if r.client == nil {
		return errors.New("clickhouse client is nil in log repository")
	}
	conn, err := r.client.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection for direct ingest: %w", err)
	}
	query := fmt.Sprintf(
		"INSERT INTO %s (timestamp, tenant_id, cluster_id, namespace, pod_name, container_name, stream, log_level, message, attributes)",
		r.tableName,
	)
	batch, err := conn.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("preparing batch insert: %w", err)
	}
	for i := range entries {
		e := &entries[i]
		if err := batch.Append(
			e.Timestamp, e.TenantID, e.ClusterID, e.Namespace, e.PodName,
			e.ContainerName, e.Stream, string(e.LogLevel), e.Message, e.Attributes,
		); err != nil {
			return fmt.Errorf("appending to batch: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("sending direct batch: %w", err)
	}
	return nil
}

// BuildLogQuery constructs a parameterized SQL query utilizing ClickHouse sparse index pruning.
// Order: tenant_id -> cluster_id -> namespace -> log_level -> timestamp -> tokenbf_v1 -> attributes.
func BuildLogQuery(f logging.LogFilter, table string) (string, []any) {
	clauses := []string{"tenant_id = ?"}
	args := []any{f.TenantID}

	add := func(col, val string) {
		if strings.TrimSpace(val) != "" {
			clauses = append(clauses, col+" = ?")
			args = append(args, val)
		}
	}
	add("cluster_id", f.ClusterID)
	add("namespace", f.Namespace)
	add("log_level", string(f.LogLevel))

	if !f.StartTime.IsZero() {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, f.StartTime)
	}
	if !f.EndTime.IsZero() {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, f.EndTime)
	}
	add("pod_name", f.PodName)
	add("container_name", f.ContainerName)
	add("stream", f.Stream)
	if strings.TrimSpace(f.SearchText) != "" {
		clauses = append(clauses, "hasToken(message, ?)")
		args = append(args, f.SearchText)
	}
	for k, v := range f.Attributes {
		clauses = append(clauses, "attributes[?] = ?")
		args = append(args, k, v)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	query := fmt.Sprintf(
		"SELECT timestamp, tenant_id, cluster_id, namespace, pod_name, container_name, stream, log_level, message, attributes FROM %s WHERE %s ORDER BY timestamp DESC LIMIT ? OFFSET ?",
		table, strings.Join(clauses, " AND "),
	)
	args = append(args, limit, offset)
	return query, args
}

// QueryLogs executes a filtered search with sparse index pruning and returns paginated results.
func (r *LogRepository) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()
	if r.client == nil {
		return nil, errors.New("clickhouse client is nil in log repository")
	}
	conn, err := r.client.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection for query: %w", err)
	}
	query, args := BuildLogQuery(filter, r.tableName)
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing query: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			fmt.Printf("clickhouse rows close notice: %v\n", closeErr)
		}
	}()

	var entries []logging.LogEntry
	for rows.Next() {
		var e logging.LogEntry
		var lvl string
		if err := rows.Scan(
			&e.Timestamp, &e.TenantID, &e.ClusterID, &e.Namespace, &e.PodName,
			&e.ContainerName, &e.Stream, &lvl, &e.Message, &e.Attributes,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		e.LogLevel = logging.LogLevel(lvl)
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}
	return &logging.LogSearchResult{
		Entries:    entries,
		TotalCount: int64(len(entries)),
		HasMore:    len(entries) >= filter.Limit,
	}, nil
}

// BuildHistogramQuery builds an aggregation query grouped by interval buckets.
func BuildHistogramQuery(f logging.LogFilter, table string, intervalSeconds int) (string, []any) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	clauses := []string{"tenant_id = ?"}
	args := []any{intervalSeconds, f.TenantID}
	if strings.TrimSpace(f.ClusterID) != "" {
		clauses = append(clauses, "cluster_id = ?")
		args = append(args, f.ClusterID)
	}
	if strings.TrimSpace(f.Namespace) != "" {
		clauses = append(clauses, "namespace = ?")
		args = append(args, f.Namespace)
	}
	if !f.StartTime.IsZero() {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, f.StartTime)
	}
	if !f.EndTime.IsZero() {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, f.EndTime)
	}
	query := fmt.Sprintf(
		"SELECT toStartOfInterval(timestamp, toIntervalSecond(?)) AS bucket, log_level, count() AS total FROM %s WHERE %s GROUP BY bucket, log_level ORDER BY bucket ASC",
		table, strings.Join(clauses, " AND "),
	)
	return query, args
}

// GetHistogram retrieves bucketed aggregation counts for timeline charts.
func (r *LogRepository) GetHistogram(
	ctx context.Context, filter logging.LogFilter, intervalSeconds int,
) ([]logging.LogAggregationBucket, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()
	if r.client == nil {
		return nil, errors.New("clickhouse client is nil in log repository")
	}
	conn, err := r.client.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection for histogram: %w", err)
	}
	query, args := BuildHistogramQuery(filter, r.tableName, intervalSeconds)
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("executing histogram query: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			fmt.Printf("clickhouse histogram rows close notice: %v\n", closeErr)
		}
	}()

	bucketMap := make(map[time.Time]*logging.LogAggregationBucket)
	var orderedBuckets []time.Time
	for rows.Next() {
		var (
			bucketTime time.Time
			level      string
			count      uint64
		)
		if err := rows.Scan(&bucketTime, &level, &count); err != nil {
			return nil, fmt.Errorf("scanning histogram row: %w", err)
		}
		b, exists := bucketMap[bucketTime]
		if !exists {
			b = &logging.LogAggregationBucket{
				TimeBucket: bucketTime,
				LevelCount: make(map[string]uint64),
			}
			bucketMap[bucketTime] = b
			orderedBuckets = append(orderedBuckets, bucketTime)
		}
		b.TotalCount += count
		b.LevelCount[level] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating histogram rows: %w", err)
	}
	result := make([]logging.LogAggregationBucket, 0, len(orderedBuckets))
	for _, bt := range orderedBuckets {
		result = append(result, *bucketMap[bt])
	}
	return result, nil
}

// TailLogs establishes an active polling stream that channels newly arriving logs until ctx is canceled.
func (r *LogRepository) TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	filter.Sanitize()

	outCh := make(chan logging.LogEntry, 100)
	go func() {
		defer close(outCh)
		lastSeen := filter.StartTime
		if lastSeen.IsZero() {
			lastSeen = time.Now().UTC()
		}
		pollTicker := time.NewTicker(time.Second)
		defer pollTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-pollTicker.C:
				currentFilter := filter
				currentFilter.StartTime = lastSeen
				currentFilter.EndTime = time.Now().UTC()
				currentFilter.Limit = 100

				res, err := r.QueryLogs(ctx, currentFilter)
				if err != nil {
					continue
				}
				for i := len(res.Entries) - 1; i >= 0; i-- {
					entry := res.Entries[i]
					if entry.Timestamp.After(lastSeen) {
						lastSeen = entry.Timestamp
					}
					select {
					case outCh <- entry:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return outCh, nil
}
