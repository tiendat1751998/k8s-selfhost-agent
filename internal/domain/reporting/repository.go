package reporting

import "context"

// Repository defines data access for the reporting center.
type Repository interface {
	ListReports(ctx context.Context, limit, offset int) ([]Report, int, error)
	GetReport(ctx context.Context, id string) (*Report, error)
	CreateReport(ctx context.Context, r *Report) error
	UpdateStatus(ctx context.Context, id, status, fileURL string) error
	DeleteReport(ctx context.Context, id string) error
}
