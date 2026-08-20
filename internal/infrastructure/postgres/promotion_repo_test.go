package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func cleanupPromotions(ctx context.Context, pool postgres.DBTX, service string) {
	_, _ = pool.Exec(ctx, "DELETE FROM promotions WHERE service = $1", service)
}

func TestPromotion_CreateAndGetByID(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewPromotionRepo(pool)
	serviceName := fmt.Sprintf("test-service-%d", time.Now().UnixNano())
	defer cleanupPromotions(ctx, pool, serviceName)

	promo, err := promotion.NewPromotion(serviceName, "v1.0.0", promotion.EnvDev, promotion.EnvQA, "test-user@k8sselfhost.local")
	if err != nil {
		t.Fatalf("failed to instantiate promotion: %v", err)
	}

	if err := repo.Create(ctx, promo); err != nil {
		t.Fatalf("failed to create promotion: %v", err)
	}

	if promo.ID == "" {
		t.Fatalf("expected generated promotion ID, got empty")
	}

	fetched, err := repo.GetByID(ctx, promo.ID)
	if err != nil {
		t.Fatalf("failed to get promotion by ID: %v", err)
	}

	if fetched.ID != promo.ID {
		t.Errorf("ID mismatch: got %s, want %s", fetched.ID, promo.ID)
	}
	if fetched.Service != serviceName {
		t.Errorf("Service mismatch: got %s, want %s", fetched.Service, serviceName)
	}
	if fetched.Version != "v1.0.0" {
		t.Errorf("Version mismatch: got %s, want v1.0.0", fetched.Version)
	}
	if fetched.FromEnv != promotion.EnvDev {
		t.Errorf("FromEnv mismatch: got %s, want dev", fetched.FromEnv)
	}
	if fetched.ToEnv != promotion.EnvQA {
		t.Errorf("ToEnv mismatch: got %s, want qa", fetched.ToEnv)
	}
	if fetched.Status != promotion.StatusPending {
		t.Errorf("Status mismatch: got %s, want pending", fetched.Status)
	}
}

func TestPromotion_Lifecycle_ApproveAndComplete(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewPromotionRepo(pool)
	serviceName := fmt.Sprintf("test-lifecycle-%d", time.Now().UnixNano())
	defer cleanupPromotions(ctx, pool, serviceName)

	promo, err := promotion.NewPromotion(serviceName, "v1.1.0", promotion.EnvQA, promotion.EnvStaging, "requester@k8sselfhost.local")
	if err != nil {
		t.Fatalf("failed to instantiate promotion: %v", err)
	}

	if err := repo.Create(ctx, promo); err != nil {
		t.Fatalf("failed to create promotion: %v", err)
	}

	// Approve
	approver := "admin@k8sselfhost.local"
	if err := repo.Approve(ctx, promo.ID, approver); err != nil {
		t.Fatalf("failed to approve promotion: %v", err)
	}

	fetched, err := repo.GetByID(ctx, promo.ID)
	if err != nil {
		t.Fatalf("failed to get approved promotion: %v", err)
	}
	if fetched.Status != promotion.StatusApproved {
		t.Errorf("expected status 'approved', got %s", fetched.Status)
	}
	if fetched.Approver != approver {
		t.Errorf("expected approver %s, got %s", approver, fetched.Approver)
	}
	if fetched.ApprovedAt == nil {
		t.Errorf("expected approved_at timestamp to be set")
	}

	// Complete
	if err := repo.Complete(ctx, promo.ID); err != nil {
		t.Fatalf("failed to complete promotion: %v", err)
	}

	completed, err := repo.GetByID(ctx, promo.ID)
	if err != nil {
		t.Fatalf("failed to get completed promotion: %v", err)
	}
	if completed.Status != promotion.StatusCompleted {
		t.Errorf("expected status 'completed', got %s", completed.Status)
	}
	if completed.CompletedAt == nil {
		t.Errorf("expected completed_at timestamp to be set")
	}
}

func TestPromotion_Lifecycle_Reject(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewPromotionRepo(pool)
	serviceName := fmt.Sprintf("test-reject-%d", time.Now().UnixNano())
	defer cleanupPromotions(ctx, pool, serviceName)

	promo, err := promotion.NewPromotion(serviceName, "v2.0.0", promotion.EnvStaging, promotion.EnvProduction, "dev@k8sselfhost.local")
	if err != nil {
		t.Fatalf("failed to instantiate promotion: %v", err)
	}

	if err := repo.Create(ctx, promo); err != nil {
		t.Fatalf("failed to create promotion: %v", err)
	}

	rejecter := "lead@k8sselfhost.local"
	if err := repo.Reject(ctx, promo.ID, rejecter); err != nil {
		t.Fatalf("failed to reject promotion: %v", err)
	}

	rejected, err := repo.GetByID(ctx, promo.ID)
	if err != nil {
		t.Fatalf("failed to get rejected promotion: %v", err)
	}
	if rejected.Status != promotion.StatusRejected {
		t.Errorf("expected status 'rejected', got %s", rejected.Status)
	}
	if rejected.Approver != rejecter {
		t.Errorf("expected approver %s, got %s", rejecter, rejected.Approver)
	}
}

func TestPromotion_ListAndPagination(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	ctx := context.Background()
	repo := postgres.NewPromotionRepo(pool)
	serviceName := fmt.Sprintf("test-list-%d", time.Now().UnixNano())
	defer cleanupPromotions(ctx, pool, serviceName)

	p1, _ := promotion.NewPromotion(serviceName, "v1.0.0", promotion.EnvDev, promotion.EnvQA, "u1@k8sselfhost.local")
	p2, _ := promotion.NewPromotion(serviceName, "v2.0.0", promotion.EnvQA, promotion.EnvStaging, "u2@k8sselfhost.local")
	_ = repo.Create(ctx, p1)
	_ = repo.Create(ctx, p2)
	_ = repo.Approve(ctx, p2.ID, "approver@k8sselfhost.local")

	// List all
	promos, total, err := repo.List(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("failed to list promotions: %v", err)
	}
	if total < 2 {
		t.Errorf("expected total >= 2, got %d", total)
	}
	if len(promos) < 2 {
		t.Errorf("expected at least 2 promotions returned, got %d", len(promos))
	}

	// List by status pending
	pendingList, pendingTotal, err := repo.List(ctx, promotion.StatusPending, 10, 0)
	if err != nil {
		t.Fatalf("failed to list pending promotions: %v", err)
	}
	if pendingTotal < 1 {
		t.Errorf("expected pendingTotal >= 1, got %d", pendingTotal)
	}
	for _, p := range pendingList {
		if p.Status != promotion.StatusPending {
			t.Errorf("expected status pending, got %s", p.Status)
		}
	}
}
