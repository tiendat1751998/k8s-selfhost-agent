// Package promotion coordinates deployment promotion business logic and state transitions.
package promotion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// Usecase coordinates deployment promotion business logic and state transitions.
type Usecase struct {
	repo promotion.Repository
}

// NewUsecase creates a new promotion usecase with the injected repository dependency.
func NewUsecase(repo promotion.Repository) *Usecase {
	return &Usecase{repo: repo}
}

// Create validates source/target environments and mandatory fields, sets the initial pending status, and persists the record.
func (u *Usecase) Create(ctx context.Context, input *promotion.Promotion) (*promotion.Promotion, error) {
	if input == nil {
		return nil, domainerrors.NewValidation("promotion", "promotion data cannot be nil")
	}

	if strings.TrimSpace(input.Service) == "" {
		return nil, domainerrors.NewValidation("service", "service is required")
	}
	if strings.TrimSpace(input.Version) == "" {
		return nil, domainerrors.NewValidation("version", "version is required")
	}
	if strings.TrimSpace(string(input.FromEnv)) == "" {
		return nil, domainerrors.NewValidation("from_env", "from_env is required")
	}
	if strings.TrimSpace(string(input.ToEnv)) == "" {
		return nil, domainerrors.NewValidation("to_env", "to_env is required")
	}

	// Business rule: source and target environments must be different
	if input.FromEnv == input.ToEnv {
		return nil, domainerrors.NewValidation("to_env", "source and target environments must be different")
	}

	// Enforce initial status to pending
	input.Status = promotion.StatusPending
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	if err := u.repo.Create(ctx, input); err != nil {
		return nil, fmt.Errorf("creating promotion: %w", err)
	}

	return input, nil
}

// Approve validates that the promotion is in Pending status before transitioning to Approved.
func (u *Usecase) Approve(ctx context.Context, id string, approver ...string) error {
	if strings.TrimSpace(id) == "" {
		return domainerrors.NewValidation("id", "promotion id is required")
	}

	promo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting promotion %s: %w", id, err)
	}
	if promo == nil {
		return domainerrors.NewNotFound("promotion", id)
	}

	// Business rule: can only approve from Pending status
	if !promo.CanApprove() {
		return domainerrors.NewValidation("status", fmt.Sprintf("cannot approve promotion in '%s' status (must be pending)", promo.Status))
	}

	var app string
	if len(approver) > 0 {
		app = approver[0]
	}

	if err := u.repo.Approve(ctx, id, app); err != nil {
		return fmt.Errorf("approving promotion: %w", err)
	}
	return nil
}

// Reject validates that the promotion is in Pending status before transitioning to Rejected.
func (u *Usecase) Reject(ctx context.Context, id string, rejecter ...string) error {
	if strings.TrimSpace(id) == "" {
		return domainerrors.NewValidation("id", "promotion id is required")
	}

	promo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting promotion %s: %w", id, err)
	}
	if promo == nil {
		return domainerrors.NewNotFound("promotion", id)
	}

	// Business rule: can only reject from Pending status
	if !promo.CanReject() {
		return domainerrors.NewValidation("status", fmt.Sprintf("cannot reject promotion in '%s' status (must be pending)", promo.Status))
	}

	var rej string
	if len(rejecter) > 0 {
		rej = rejecter[0]
	}

	if err := u.repo.Reject(ctx, id, rej); err != nil {
		return fmt.Errorf("rejecting promotion: %w", err)
	}
	return nil
}

// Complete validates that the promotion is in Approved or Promoting status before transitioning to Completed.
func (u *Usecase) Complete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return domainerrors.NewValidation("id", "promotion id is required")
	}

	promo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting promotion %s: %w", id, err)
	}
	if promo == nil {
		return domainerrors.NewNotFound("promotion", id)
	}

	// Business rule: can only complete from Approved status (or promoting)
	if !promo.CanComplete() {
		return domainerrors.NewValidation("status", fmt.Sprintf("cannot complete promotion in '%s' status (must be approved)", promo.Status))
	}

	if err := u.repo.Complete(ctx, id); err != nil {
		return fmt.Errorf("completing promotion: %w", err)
	}
	return nil
}

// List retrieves promotions matching the optional status and pagination parameters.
func (u *Usecase) List(ctx context.Context, status string, limit, offset int) ([]promotion.Promotion, int, error) {
	return u.repo.List(ctx, status, limit, offset)
}

// ListAll retrieves all promotions.
func (u *Usecase) ListAll(ctx context.Context) ([]promotion.Promotion, error) {
	items, _, err := u.repo.List(ctx, "", 0, 0)
	return items, err
}

// GetByID retrieves a promotion by its ID.
func (u *Usecase) GetByID(ctx context.Context, id string) (*promotion.Promotion, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domainerrors.NewValidation("id", "promotion id is required")
	}
	return u.repo.GetByID(ctx, id)
}
