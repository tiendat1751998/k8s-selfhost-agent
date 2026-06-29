package changes

import "context"

// Repository defines data access for change management.
type Repository interface {
	ListRequests(ctx context.Context, status *ChangeStatus, limit, offset int) ([]ChangeRequest, int, error)
	GetRequest(ctx context.Context, id string) (*ChangeRequest, error)
	CreateRequest(ctx context.Context, r *ChangeRequest) error
	ApproveRequest(ctx context.Context, id, approver string) error
	RejectRequest(ctx context.Context, id, approver string) error

	ListWindows(ctx context.Context) ([]MaintenanceWindow, error)
	CreateWindow(ctx context.Context, w *MaintenanceWindow) error
}
