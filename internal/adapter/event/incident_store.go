// Package event provides the incident store handler that deduplicates and persists incidents.
package event

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

// IncidentStore handles incident deduplication, persistence, and event publishing.
type IncidentStore struct {
	repo      incident.Repository
	publisher *infraNats.Publisher
}

// NewIncidentStore creates a new IncidentStore.
func NewIncidentStore(repo incident.Repository, publisher *infraNats.Publisher) *IncidentStore {
	return &IncidentStore{
		repo:      repo,
		publisher: publisher,
	}
}

// Handle processes a detected incident: deduplicates, persists, and publishes an event.
// This implements the IncidentHandler type used by the EventWatcher.
func (s *IncidentStore) Handle(ctx context.Context, inc *incident.Incident) error {
	log := logger.WithContext(ctx)

	// Deduplication: check if an active incident already exists for this pod+type
	existing, err := s.repo.GetByPodAndType(ctx, inc.Namespace, inc.PodName, inc.Type)
	if err != nil {
		return fmt.Errorf("checking for existing incident: %w", err)
	}

	if existing != nil {
		log.Debug("incident already exists, skipping",
			zap.String("incident_id", existing.ID),
			zap.String("pod", inc.PodName),
			zap.String("type", inc.Type.String()),
		)
		return nil
	}

	// Persist new incident
	if err := s.repo.Create(ctx, inc); err != nil {
		return fmt.Errorf("creating incident: %w", err)
	}

	log.Info("new incident detected and stored",
		zap.String("incident_id", inc.ID),
		zap.String("namespace", inc.Namespace),
		zap.String("pod", inc.PodName),
		zap.String("type", inc.Type.String()),
		zap.String("severity", inc.Severity.String()),
	)

	// Publish event to NATS for downstream processing
	subject := fmt.Sprintf("incidents.%s.%s", inc.Type, inc.Severity)
	if err := s.publisher.Publish(ctx, subject, inc); err != nil {
		log.Error("failed to publish incident event",
			zap.String("incident_id", inc.ID),
			zap.Error(err),
		)
		// Non-fatal: incident is already persisted
	}

	return nil
}
