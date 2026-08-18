package promotion_test

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

func TestNewPromotion_Success(t *testing.T) {
	p, err := promotion.NewPromotion("order-api", "v1.2.3", promotion.EnvStaging, promotion.EnvProduction, "devops@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Service != "order-api" {
		t.Errorf("expected service 'order-api', got '%s'", p.Service)
	}
	if p.Version != "v1.2.3" {
		t.Errorf("expected version 'v1.2.3', got '%s'", p.Version)
	}
	if p.FromEnv != promotion.EnvStaging {
		t.Errorf("expected from_env 'staging', got '%s'", p.FromEnv)
	}
	if p.ToEnv != promotion.EnvProduction {
		t.Errorf("expected to_env 'production', got '%s'", p.ToEnv)
	}
	if p.Requester != "devops@example.com" {
		t.Errorf("expected requester 'devops@example.com', got '%s'", p.Requester)
	}
	if p.Status != promotion.StatusPending {
		t.Errorf("expected status 'pending', got '%s'", p.Status)
	}
	if p.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestNewPromotion_Validation(t *testing.T) {
	tests := []struct {
		name      string
		service   string
		version   string
		fromEnv   promotion.Environment
		toEnv     promotion.Environment
		requester string
	}{
		{
			name:      "empty service",
			service:   "",
			version:   "v1.0.0",
			fromEnv:   promotion.EnvDev,
			toEnv:     promotion.EnvQA,
			requester: "alice",
		},
		{
			name:      "empty version",
			service:   "svc",
			version:   "",
			fromEnv:   promotion.EnvDev,
			toEnv:     promotion.EnvQA,
			requester: "alice",
		},
		{
			name:      "empty from_env",
			service:   "svc",
			version:   "v1.0.0",
			fromEnv:   "",
			toEnv:     promotion.EnvQA,
			requester: "alice",
		},
		{
			name:      "empty to_env",
			service:   "svc",
			version:   "v1.0.0",
			fromEnv:   promotion.EnvDev,
			toEnv:     "",
			requester: "alice",
		},
		{
			name:      "same from_env and to_env",
			service:   "svc",
			version:   "v1.0.0",
			fromEnv:   promotion.EnvStaging,
			toEnv:     promotion.EnvStaging,
			requester: "alice",
		},
		{
			name:      "empty requester",
			service:   "svc",
			version:   "v1.0.0",
			fromEnv:   promotion.EnvDev,
			toEnv:     promotion.EnvQA,
			requester: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := promotion.NewPromotion(tt.service, tt.version, tt.fromEnv, tt.toEnv, tt.requester)
			if err == nil {
				t.Errorf("expected validation error for %s, got promotion: %+v", tt.name, p)
			}
		})
	}
}

func TestPromotion_Approve_Success(t *testing.T) {
	p, err := promotion.NewPromotion("auth-svc", "v2.0.0", promotion.EnvQA, promotion.EnvStaging, "developer")
	if err != nil {
		t.Fatalf("failed to create promotion: %v", err)
	}

	if err := p.Approve("lead-dev"); err != nil {
		t.Fatalf("unexpected error on approve: %v", err)
	}

	if p.Status != promotion.StatusApproved {
		t.Errorf("expected status 'approved', got '%s'", p.Status)
	}
	if p.Approver != "lead-dev" {
		t.Errorf("expected approver 'lead-dev', got '%s'", p.Approver)
	}
	if p.ApprovedAt == nil {
		t.Error("expected approved_at timestamp to be set")
	}
}

func TestPromotion_Approve_InvalidStatus(t *testing.T) {
	invalidStatuses := []string{
		promotion.StatusApproved,
		promotion.StatusPromoting,
		promotion.StatusCompleted,
		promotion.StatusRejected,
		promotion.StatusFailed,
	}

	for _, status := range invalidStatuses {
		t.Run("from "+status, func(t *testing.T) {
			p := &promotion.Promotion{
				Status: status,
			}
			err := p.Approve("admin")
			if err == nil {
				t.Errorf("expected error approving promotion from status '%s'", status)
			}
		})
	}
}

func TestPromotion_Reject_Success(t *testing.T) {
	p, err := promotion.NewPromotion("billing-svc", "v1.0.1", promotion.EnvDev, promotion.EnvQA, "developer")
	if err != nil {
		t.Fatalf("failed to create promotion: %v", err)
	}

	if err := p.Reject("qa-manager"); err != nil {
		t.Fatalf("unexpected error on reject: %v", err)
	}

	if p.Status != promotion.StatusRejected {
		t.Errorf("expected status 'rejected', got '%s'", p.Status)
	}
	if p.Approver != "qa-manager" {
		t.Errorf("expected approver 'qa-manager', got '%s'", p.Approver)
	}
}

func TestPromotion_Reject_InvalidStatus(t *testing.T) {
	invalidStatuses := []string{
		promotion.StatusApproved,
		promotion.StatusPromoting,
		promotion.StatusCompleted,
		promotion.StatusRejected,
		promotion.StatusFailed,
	}

	for _, status := range invalidStatuses {
		t.Run("from "+status, func(t *testing.T) {
			p := &promotion.Promotion{
				Status: status,
			}
			err := p.Reject("qa-manager")
			if err == nil {
				t.Errorf("expected error rejecting promotion from status '%s'", status)
			}
		})
	}
}

func TestPromotion_Complete_Success(t *testing.T) {
	t.Run("from approved", func(t *testing.T) {
		p, _ := promotion.NewPromotion("frontend", "v3.1.0", promotion.EnvStaging, promotion.EnvProduction, "dev")
		_ = p.Approve("release-manager")

		if err := p.Complete(); err != nil {
			t.Fatalf("unexpected error completing promotion: %v", err)
		}
		if p.Status != promotion.StatusCompleted {
			t.Errorf("expected status 'completed', got '%s'", p.Status)
		}
		if p.CompletedAt == nil {
			t.Error("expected completed_at timestamp to be set")
		}
	})

	t.Run("from promoting", func(t *testing.T) {
		p, _ := promotion.NewPromotion("frontend", "v3.1.0", promotion.EnvStaging, promotion.EnvProduction, "dev")
		_ = p.Approve("release-manager")
		_ = p.MarkPromoting()

		if err := p.Complete(); err != nil {
			t.Fatalf("unexpected error completing promotion from promoting: %v", err)
		}
		if p.Status != promotion.StatusCompleted {
			t.Errorf("expected status 'completed', got '%s'", p.Status)
		}
		if p.CompletedAt == nil {
			t.Error("expected completed_at timestamp to be set")
		}
	})
}

func TestPromotion_Complete_InvalidStatus(t *testing.T) {
	invalidStatuses := []string{
		promotion.StatusPending,
		promotion.StatusCompleted,
		promotion.StatusRejected,
		promotion.StatusFailed,
	}

	for _, status := range invalidStatuses {
		t.Run("from "+status, func(t *testing.T) {
			p := &promotion.Promotion{
				Status: status,
			}
			err := p.Complete()
			if err == nil {
				t.Errorf("expected error completing promotion from status '%s'", status)
			}
		})
	}
}

func TestPromotion_CanApprove_CanReject_CanComplete(t *testing.T) {
	tests := []struct {
		status      string
		canApprove  bool
		canReject   bool
		canComplete bool
	}{
		{promotion.StatusPending, true, true, false},
		{promotion.StatusApproved, false, false, true},
		{promotion.StatusPromoting, false, false, true},
		{promotion.StatusCompleted, false, false, false},
		{promotion.StatusRejected, false, false, false},
		{promotion.StatusFailed, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			p := &promotion.Promotion{Status: tt.status}
			if p.CanApprove() != tt.canApprove {
				t.Errorf("CanApprove() for status %s expected %v, got %v", tt.status, tt.canApprove, p.CanApprove())
			}
			if p.CanReject() != tt.canReject {
				t.Errorf("CanReject() for status %s expected %v, got %v", tt.status, tt.canReject, p.CanReject())
			}
			if p.CanComplete() != tt.canComplete {
				t.Errorf("CanComplete() for status %s expected %v, got %v", tt.status, tt.canComplete, p.CanComplete())
			}
		})
	}
}

func TestPromotion_MarkPromoting_And_Fail(t *testing.T) {
	p, _ := promotion.NewPromotion("search-svc", "v1.1.0", promotion.EnvQA, promotion.EnvStaging, "dev")

	// Cannot mark promoting from pending
	if err := p.MarkPromoting(); err == nil {
		t.Error("expected error marking promoting from pending status")
	}

	_ = p.Approve("lead")
	if err := p.MarkPromoting(); err != nil {
		t.Fatalf("unexpected error marking promoting from approved status: %v", err)
	}
	if p.Status != promotion.StatusPromoting {
		t.Errorf("expected status 'promoting', got '%s'", p.Status)
	}

	p.Fail()
	if p.Status != promotion.StatusFailed {
		t.Errorf("expected status 'failed', got '%s'", p.Status)
	}
}
