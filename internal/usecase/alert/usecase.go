package alert

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
)

type Usecase struct {
	repo alert.Repository
}

func NewUsecase(repo alert.Repository) *Usecase {
	return &Usecase{repo: repo}
}

func (u *Usecase) CreateChannel(ctx context.Context, channel *alert.NotificationChannel) error {
	return u.repo.CreateChannel(ctx, channel)
}

func (u *Usecase) ListChannels(ctx context.Context, tenantID string) ([]*alert.NotificationChannel, error) {
	return u.repo.ListChannels(ctx, tenantID)
}

func (u *Usecase) DeleteChannel(ctx context.Context, id, tenantID string) error {
	return u.repo.DeleteChannel(ctx, id, tenantID)
}

func (u *Usecase) CreateRule(ctx context.Context, rule *alert.AlertRule) error {
	return u.repo.CreateRule(ctx, rule)
}

func (u *Usecase) ListRules(ctx context.Context, tenantID string) ([]*alert.AlertRule, error) {
	return u.repo.ListRules(ctx, tenantID)
}

func (u *Usecase) UpdateRule(ctx context.Context, rule *alert.AlertRule) error {
	return u.repo.UpdateRule(ctx, rule)
}

func (u *Usecase) DeleteRule(ctx context.Context, id, tenantID string) error {
	return u.repo.DeleteRule(ctx, id, tenantID)
}

func (u *Usecase) ListHistory(ctx context.Context, tenantID string) ([]*alert.AlertHistory, error) {
	return u.repo.ListHistory(ctx, tenantID)
}

func (u *Usecase) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error {
	return u.repo.AcknowledgeAlert(ctx, id, tenantID, userID)
}

func (u *Usecase) ResolveAlert(ctx context.Context, id, tenantID, userID string) error {
	return u.repo.ResolveAlert(ctx, id, tenantID, userID)
}

func (u *Usecase) CreateSilence(ctx context.Context, silence *alert.Silence) error {
	return u.repo.CreateSilence(ctx, silence)
}

func (u *Usecase) ListSilences(ctx context.Context, tenantID string) ([]*alert.Silence, error) {
	return u.repo.ListSilences(ctx, tenantID)
}

func (u *Usecase) IsAlertSilenced(ctx context.Context, tenantID, alertID string) (bool, error) {
	return u.repo.IsAlertSilenced(ctx, tenantID, alertID)
}

