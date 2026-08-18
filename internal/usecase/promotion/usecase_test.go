package promotion

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	domainPromo "github.com/datdt/k8sselfhost/internal/domain/promotion"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockPromotionRepo struct {
	items map[string]*domainPromo.Promotion
}

func newMockPromotionRepo() *mockPromotionRepo {
	return &mockPromotionRepo{
		items: make(map[string]*domainPromo.Promotion),
	}
}

func (m *mockPromotionRepo) List(ctx context.Context, status string, limit, offset int) ([]domainPromo.Promotion, int, error) {
	var result []domainPromo.Promotion
	for _, p := range m.items {
		if status == "" || status == "all" || p.Status == status {
			result = append(result, *p)
		}
	}
	return result, len(result), nil
}

func (m *mockPromotionRepo) GetByID(ctx context.Context, id string) (*domainPromo.Promotion, error) {
	p, ok := m.items[id]
	if !ok {
		return nil, domainerrors.NewNotFound("promotion", id)
	}
	// return copy
	cp := *p
	return &cp, nil
}

func (m *mockPromotionRepo) Create(ctx context.Context, p *domainPromo.Promotion) error {
	if p.ID == "" {
		p.ID = fmt.Sprintf("promo-%d", len(m.items)+1)
	}
	m.items[p.ID] = p
	return nil
}

func (m *mockPromotionRepo) Approve(ctx context.Context, id, approver string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusApproved
	p.Approver = approver
	now := time.Now().UTC()
	p.ApprovedAt = &now
	return nil
}

func (m *mockPromotionRepo) Reject(ctx context.Context, id, rejecter string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusRejected
	p.Approver = rejecter
	now := time.Now().UTC()
	p.ApprovedAt = &now
	return nil
}

func (m *mockPromotionRepo) Complete(ctx context.Context, id string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusCompleted
	now := time.Now().UTC()
	p.CompletedAt = &now
	return nil
}

func TestUsecase_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("success create promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		input := &domainPromo.Promotion{
			Service:   "payment-service",
			Version:   "v1.2.0",
			FromEnv:   domainPromo.EnvDev,
			ToEnv:     domainPromo.EnvQA,
			Requester: "alice",
		}

		res, err := uc.Create(ctx, input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.ID == "" {
			t.Errorf("expected generated ID, got empty")
		}
		if res.Status != domainPromo.StatusPending {
			t.Errorf("expected initial status pending, got %s", res.Status)
		}
	})

	t.Run("fails when source and target env are equal", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		input := &domainPromo.Promotion{
			Service:   "payment-service",
			Version:   "v1.2.0",
			FromEnv:   domainPromo.EnvStaging,
			ToEnv:     domainPromo.EnvStaging,
			Requester: "alice",
		}

		_, err := uc.Create(ctx, input)
		if err == nil {
			t.Fatal("expected validation error for identical environments, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("fails when mandatory fields are missing", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		// nil input
		if _, err := uc.Create(ctx, nil); err == nil {
			t.Errorf("expected error for nil input")
		}

		// missing service
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Version: "v1", FromEnv: "dev", ToEnv: "qa"}); err == nil {
			t.Errorf("expected error for missing service")
		}

		// missing version
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Service: "svc", FromEnv: "dev", ToEnv: "qa"}); err == nil {
			t.Errorf("expected error for missing version")
		}

		// missing from_env
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Service: "svc", Version: "v1", ToEnv: "qa"}); err == nil {
			t.Errorf("expected error for missing from_env")
		}

		// missing to_env
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Service: "svc", Version: "v1", FromEnv: "dev"}); err == nil {
			t.Errorf("expected error for missing to_env")
		}
	})
}

func TestUsecase_StateTransitions(t *testing.T) {
	ctx := context.Background()

	t.Run("approve pending promotion succeeds", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, err := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		err = uc.Approve(ctx, promo.ID, "bob")
		if err != nil {
			t.Fatalf("approve failed: %v", err)
		}

		stored, err := uc.GetByID(ctx, promo.ID)
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if stored.Status != domainPromo.StatusApproved {
			t.Errorf("expected status approved, got %s", stored.Status)
		}
		if stored.Approver != "bob" {
			t.Errorf("expected approver bob, got %s", stored.Approver)
		}
	})

	t.Run("cannot approve already approved or rejected promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		_ = uc.Approve(ctx, promo.ID, "bob")

		// Re-approving must fail
		err := uc.Approve(ctx, promo.ID, "bob")
		if err == nil {
			t.Fatal("expected error approving already approved promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("reject pending promotion succeeds", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		err := uc.Reject(ctx, promo.ID, "charlie")
		if err != nil {
			t.Fatalf("reject failed: %v", err)
		}

		stored, _ := uc.GetByID(ctx, promo.ID)
		if stored.Status != domainPromo.StatusRejected {
			t.Errorf("expected status rejected, got %s", stored.Status)
		}
	})

	t.Run("cannot reject non-pending promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		_ = uc.Approve(ctx, promo.ID, "bob")

		// Rejecting approved promotion must fail
		err := uc.Reject(ctx, promo.ID, "charlie")
		if err == nil {
			t.Fatal("expected error rejecting approved promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("complete approved promotion succeeds", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		_ = uc.Approve(ctx, promo.ID, "bob")

		err := uc.Complete(ctx, promo.ID)
		if err != nil {
			t.Fatalf("complete failed: %v", err)
		}

		stored, _ := uc.GetByID(ctx, promo.ID)
		if stored.Status != domainPromo.StatusCompleted {
			t.Errorf("expected status completed, got %s", stored.Status)
		}
	})

	t.Run("cannot complete pending or rejected promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		// Complete while still pending
		err := uc.Complete(ctx, promo.ID)
		if err == nil {
			t.Fatal("expected error completing pending promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}

		// Reject then complete
		_ = uc.Reject(ctx, promo.ID, "charlie")
		err = uc.Complete(ctx, promo.ID)
		if err == nil {
			t.Fatal("expected error completing rejected promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("validation on empty IDs", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		if err := uc.Approve(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in Approve")
		}
		if err := uc.Reject(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in Reject")
		}
		if err := uc.Complete(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in Complete")
		}
		if _, err := uc.GetByID(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in GetByID")
		}
	})

	t.Run("list promotions", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		_, _ = uc.Create(ctx, &domainPromo.Promotion{
			Service: "svc-1",
			Version: "v1",
			FromEnv: "dev",
			ToEnv:   "qa",
		})
		_, _ = uc.Create(ctx, &domainPromo.Promotion{
			Service: "svc-2",
			Version: "v2",
			FromEnv: "qa",
			ToEnv:   "staging",
		})

		items, total, err := uc.List(ctx, "", 10, 0)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if total != 2 || len(items) != 2 {
			t.Errorf("expected 2 items, got total=%d len=%d", total, len(items))
		}

		all, err := uc.ListAll(ctx)
		if err != nil {
			t.Fatalf("listAll failed: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("expected 2 items, got %d", len(all))
		}
	})
}
