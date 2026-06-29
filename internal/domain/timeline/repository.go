package timeline

import (
	"context"
	"time"
)

// Repository defines the data access interface for timeline events.
type Repository interface {
	List(ctx context.Context, eventType *EventType, since time.Time, limit, offset int) ([]Event, int, error)
	GetByID(ctx context.Context, id string) (*Event, error)
	Create(ctx context.Context, e *Event) error
}
