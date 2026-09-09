package event

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

// DockerBroadcaster sends messages to real-time subscribers (e.g. WebSocket clients).
type DockerBroadcaster interface {
	Broadcast(msgType string, data interface{})
}


func (w *DockerEventWatcher) persistAndBroadcast(ctx context.Context, inc *incident.Incident) error {
	if w.incRepo != nil {
		// Deduplication check: do not recreate if active incident exists
		existing, err := w.incRepo.GetByPodAndType(ctx, inc.Namespace, inc.PodName, inc.Type)
		if err != nil {
			w.logger.Warn("Failed to check for existing incident", zap.Error(err))
		}
		if existing != nil && existing.Status != incident.StatusResolved && existing.Status != incident.StatusFailed {
			w.logger.Debug("Active incident already exists, deduplicating",
				zap.String("service", inc.PodName),
				zap.String("type", string(inc.Type)),
				zap.String("existing_id", existing.ID),
			)
			return nil
		}

		if err := w.incRepo.Create(ctx, inc); err != nil {
			return fmt.Errorf("creating incident in repository: %w", err)
		}
	}

	if w.broadcaster != nil {
		w.broadcaster.Broadcast("incident", inc)
	}

	if w.handler != nil {
		if err := w.handler(ctx, inc); err != nil {
			w.logger.Warn("Incident handler callback returned error", zap.Error(err))
		}
	}

	return nil
}

