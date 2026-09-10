package logging_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainLog "github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	usecaseLog "github.com/datdt/k8sselfhost/internal/usecase/logging"
)

type memoryLogRepo struct {
	lastFilter    domainLog.LogFilter
	lastEntries   []domainLog.LogEntry
	intervalSecs  int
	searchResult  *domainLog.LogSearchResult
	histogramRes  []domainLog.LogAggregationBucket
	tailCh        chan domainLog.LogEntry
}

func (m *memoryLogRepo) IngestBatch(ctx context.Context, entries []domainLog.LogEntry) error {
	m.lastEntries = entries
	return nil
}

func (m *memoryLogRepo) QueryLogs(ctx context.Context, filter domainLog.LogFilter) (*domainLog.LogSearchResult, error) {
	m.lastFilter = filter
	if m.searchResult != nil {
		return m.searchResult, nil
	}
	return &domainLog.LogSearchResult{}, nil
}

func (m *memoryLogRepo) GetHistogram(ctx context.Context, filter domainLog.LogFilter, intervalSeconds int) ([]domainLog.LogAggregationBucket, error) {
	m.lastFilter = filter
	m.intervalSecs = intervalSeconds
	return m.histogramRes, nil
}

func (m *memoryLogRepo) TailLogs(ctx context.Context, filter domainLog.LogFilter) (<-chan domainLog.LogEntry, error) {
	m.lastFilter = filter
	if m.tailCh == nil {
		m.tailCh = make(chan domainLog.LogEntry)
	}
	return m.tailCh, nil
}

func TestService_TenantContextMissing(t *testing.T) {
	repo := &memoryLogRepo{}
	svc := usecaseLog.NewService(repo)
	ctx := context.Background() // No tenant in context

	_, err := svc.QueryLogs(ctx, domainLog.LogFilter{})
	require.ErrorIs(t, err, domainLog.ErrInvalidLogQuery)

	_, err = svc.GetHistogram(ctx, domainLog.LogFilter{}, 60)
	require.ErrorIs(t, err, domainLog.ErrInvalidLogQuery)

	_, err = svc.TailLogs(ctx, domainLog.LogFilter{})
	require.ErrorIs(t, err, domainLog.ErrInvalidLogQuery)

	err = svc.Ingest(ctx, []domainLog.LogEntry{{ClusterID: "c1", Message: "test"}})
	require.ErrorIs(t, err, domainLog.ErrInvalidLogQuery)
}

func TestService_TenantIsolationAndLimitClamping(t *testing.T) {
	repo := &memoryLogRepo{
		searchResult: &domainLog.LogSearchResult{
			Entries:    []domainLog.LogEntry{{ClusterID: "c1", Message: "found"}},
			TotalCount: 1,
		},
	}
	svc := usecaseLog.NewService(repo)

	// Context contains tenant-alpha
	ctx := tenancy.WithTenantID(context.Background(), "tenant-alpha")

	// Filter attempts to query tenant-bravo (spoofing attempt) with excessive limit
	filter := domainLog.LogFilter{
		TenantID:  "tenant-bravo",
		ClusterID: "c1",
		Limit:     500, // Should be clamped to 100 by Sanitize
	}

	res, err := svc.QueryLogs(ctx, filter)
	require.NoError(t, err)
	require.NotNil(t, res)

	// Verify tenant isolation forced tenant-alpha and clamped limit to 100
	require.Equal(t, "tenant-alpha", repo.lastFilter.TenantID)
	require.Equal(t, 100, repo.lastFilter.Limit)
}

func TestService_IngestEnforcesTenantID(t *testing.T) {
	repo := &memoryLogRepo{}
	svc := usecaseLog.NewService(repo)

	ctx := tenancy.WithTenantID(context.Background(), "tenant-alpha")

	entries := []domainLog.LogEntry{
		{
			ClusterID: "c1",
			Message:   "msg 1",
			TenantID:  "wrong-tenant",
		},
		{
			ClusterID: "c1",
			Message:   "msg 2",
		},
	}

	err := svc.Ingest(ctx, entries)
	require.NoError(t, err)

	require.Len(t, repo.lastEntries, 2)
	require.Equal(t, "tenant-alpha", repo.lastEntries[0].TenantID)
	require.Equal(t, "tenant-alpha", repo.lastEntries[1].TenantID)
}

func TestService_HistogramAndTail(t *testing.T) {
	repo := &memoryLogRepo{
		histogramRes: []domainLog.LogAggregationBucket{
			{TimeBucket: time.Now().UTC(), TotalCount: 42},
		},
	}
	svc := usecaseLog.NewService(repo)
	ctx := tenancy.WithTenantID(context.Background(), "tenant-prod")

	buckets, err := svc.GetHistogram(ctx, domainLog.LogFilter{Limit: 200}, 30)
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	require.Equal(t, "tenant-prod", repo.lastFilter.TenantID)
	require.Equal(t, 30, repo.intervalSecs)

	tailCh, err := svc.TailLogs(ctx, domainLog.LogFilter{})
	require.NoError(t, err)
	require.NotNil(t, tailCh)
	require.Equal(t, "tenant-prod", repo.lastFilter.TenantID)
}