package capacity

import "context"

// Repository defines the data access interface for capacity forecasts and historical snapshots.
type Repository interface {
	List(ctx context.Context, cluster string) ([]Forecast, error)
	Record(ctx context.Context, f *Forecast) error
	ListSnapshots(ctx context.Context, cluster string) ([]Snapshot, error)
}

