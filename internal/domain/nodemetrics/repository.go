// Package nodemetrics provides domain entities and repository ports for per-node historical time-series telemetry.
package nodemetrics

import (
	"context"
	"time"
)

// Repository defines the persistence port for node metric rollups and historical telemetry.
type Repository interface {
	// InsertBatch saves a batch of node metric rollups in a single transaction.
	InsertBatch(ctx context.Context, rollups []NodeMetricRollup) error

	// QueryHistory retrieves historical samples for a node matching the query criteria.
	QueryHistory(ctx context.Context, q NodeHistoryQuery) ([]NodeMetricRollup, error)

	// GetSummary calculates high-level statistical summaries for a node over a time window.
	GetSummary(ctx context.Context, q NodeHistoryQuery) (*NodeHistoricalSummary, error)

	// DownsampleAndPrune aggregates old 1m samples into 1h buckets and prunes expired data.
	DownsampleAndPrune(ctx context.Context, olderThan7Days, olderThan90Days time.Time) error
}
