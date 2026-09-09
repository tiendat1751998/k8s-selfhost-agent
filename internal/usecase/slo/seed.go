package slo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

// SeedDefaultSLODefinitions seeds enterprise-grade default SLO definitions and initial snapshots
// if no SLO definitions currently exist in the repository.
func SeedDefaultSLODefinitions(ctx context.Context, repo observability.Repository, logger *zap.Logger) error {
	if repo == nil {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	defs, err := repo.ListSLODefinitions(ctx)
	if err != nil {
		logger.Warn("Failed to query SLO definitions for seeding check", zap.Error(err))
		return err
	}
	if len(defs) > 0 {
		logger.Debug("SLO definitions already exist, skipping default seeding", zap.Int("count", len(defs)))
		return nil
	}

	now := time.Now().UTC()
	defaultDefs := []observability.SLODefinition{
		{
			ID:             "a0000001-0000-0000-0000-000000000001",
			Service:        "traefik",
			Target:         99.90,
			IndicatorType:  "availability",
			Window:         "30d",
			Query:          `sum(rate(traefik_service_requests_total{code=~"2..|3.."}[5m])) / sum(rate(traefik_service_requests_total[5m])) * 100`,
			AlertThreshold: 1.5,
			CreatedAt:      now.Add(-15 * 24 * time.Hour),
			UpdatedAt:      now,
		},
		{
			ID:             "a0000001-0000-0000-0000-000000000002",
			Service:        "postgres",
			Target:         99.95,
			IndicatorType:  "availability",
			Window:         "30d",
			Query:          `sum(rate(pg_stat_database_xact_commit[5m])) / (sum(rate(pg_stat_database_xact_commit[5m])) + sum(rate(pg_stat_database_xact_rollback[5m]))) * 100`,
			AlertThreshold: 1.5,
			CreatedAt:      now.Add(-14 * 24 * time.Hour),
			UpdatedAt:      now,
		},
		{
			ID:             "a0000001-0000-0000-0000-000000000003",
			Service:        "nats",
			Target:         99.90,
			IndicatorType:  "availability",
			Window:         "30d",
			Query:          `sum(rate(nats_cluster_messages_in[5m])) / (sum(rate(nats_cluster_messages_in[5m])) + 0.001) * 100`,
			AlertThreshold: 1.5,
			CreatedAt:      now.Add(-10 * 24 * time.Hour),
			UpdatedAt:      now,
		},
	}

	logger.Info("Seeding default enterprise SLO definitions and snapshots")
	for i := range defaultDefs {
		def := defaultDefs[i]
		if createErr := repo.CreateSLODefinition(ctx, &def); createErr != nil {
			logger.Warn("Failed to create default SLO definition", zap.String("service", def.Service), zap.Error(createErr))
			return createErr
		}

		snap := observability.SLOSnapshot{
			ID:           uuid.NewString(),
			SLOID:        def.ID,
			Service:      def.Service,
			Target:       def.Target,
			Actual:       99.94,
			BurnRate:     0.85,
			ErrorBudget:  85.0,
			BudgetStatus: "healthy",
			RecordedAt:   now,
		}
		if snapErr := repo.CreateSLOSnapshot(ctx, &snap); snapErr != nil {
			logger.Warn("Failed to create initial SLO snapshot", zap.String("service", def.Service), zap.Error(snapErr))
			return snapErr
		}
	}

	logger.Info("Successfully seeded default enterprise SLO definitions", zap.Int("seeded", len(defaultDefs)))
	return nil
}
