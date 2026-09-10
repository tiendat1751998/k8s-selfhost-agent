package logging

import (
	"context"
	"fmt"
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// Service coordinates log querying and ingestion with strict multi-tenant isolation.
type Service struct {
	repo logging.LogRepository
}

// NewService constructs a logging usecase service.
func NewService(repo logging.LogRepository) *Service {
	return &Service{
		repo: repo,
	}
}

// resolveTenant extracts and verifies the tenant ID from context.
func (s *Service) resolveTenant(ctx context.Context) (string, error) {
	tenantID := tenancy.TenantIDFromContext(ctx)
	if strings.TrimSpace(tenantID) == "" {
		return "", fmt.Errorf("%w: tenant_id required in request context", logging.ErrInvalidLogQuery)
	}
	return tenantID, nil
}

// sanitizeFilter enforces tenant isolation and query constraints.
func (s *Service) sanitizeFilter(ctx context.Context, filter logging.LogFilter) (logging.LogFilter, error) {
	tenantID, err := s.resolveTenant(ctx)
	if err != nil {
		return logging.LogFilter{}, err
	}

	// Overwrite any tenant ID to prevent cross-tenant enumeration attacks
	filter.TenantID = tenantID

	if err := filter.Validate(); err != nil {
		return logging.LogFilter{}, err
	}

	filter.Sanitize()
	return filter, nil
}

// QueryLogs searches logs for the tenant within time boundaries.
func (s *Service) QueryLogs(ctx context.Context, filter logging.LogFilter) (*logging.LogSearchResult, error) {
	sanitized, err := s.sanitizeFilter(ctx, filter)
	if err != nil {
		return nil, err
	}
	return s.repo.QueryLogs(ctx, sanitized)
}

// GetHistogram aggregates log counts into discrete time intervals for the authenticated tenant.
func (s *Service) GetHistogram(
	ctx context.Context,
	filter logging.LogFilter,
	intervalSeconds int,
) ([]logging.LogAggregationBucket, error) {
	sanitized, err := s.sanitizeFilter(ctx, filter)
	if err != nil {
		return nil, err
	}
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	return s.repo.GetHistogram(ctx, sanitized, intervalSeconds)
}

// TailLogs opens a live streaming channel filtered by tenant context.
func (s *Service) TailLogs(ctx context.Context, filter logging.LogFilter) (<-chan logging.LogEntry, error) {
	sanitized, err := s.sanitizeFilter(ctx, filter)
	if err != nil {
		return nil, err
	}
	return s.repo.TailLogs(ctx, sanitized)
}

// Ingest sanitizes and streams logs into storage, overriding entry tenant_id with context tenant.
func (s *Service) Ingest(ctx context.Context, entries []logging.LogEntry) error {
	tenantID, err := s.resolveTenant(ctx)
	if err != nil {
		return err
	}

	for i := range entries {
		entries[i].TenantID = tenantID
		if err := entries[i].Validate(); err != nil {
			return err
		}
	}

	return s.repo.IngestBatch(ctx, entries)
}