package slo

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

type errorObsRepo struct {
	mockObsRepo
	listErr   error
	createErr error
}

func (r *errorObsRepo) ListSLODefinitions(ctx context.Context) ([]observability.SLODefinition, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.mockObsRepo.ListSLODefinitions(ctx)
}

func (r *errorObsRepo) CreateSLODefinition(ctx context.Context, d *observability.SLODefinition) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.mockObsRepo.CreateSLODefinition(ctx, d)
}

func TestSeedDefaultSLODefinitions_EmptyRepo(t *testing.T) {
	ctx := context.Background()
	repo := newMockObsRepo()
	logger := zap.NewNop()

	err := SeedDefaultSLODefinitions(ctx, repo, logger)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	defs, err := repo.ListSLODefinitions(ctx)
	if err != nil {
		t.Fatalf("failed to list definitions: %v", err)
	}
	if len(defs) != 3 {
		t.Fatalf("expected 3 seeded definitions, got %d", len(defs))
	}

	expectedServices := map[string]float64{
		"traefik":  99.90,
		"postgres": 99.95,
		"nats":     99.90,
	}

	for _, d := range defs {
		target, ok := expectedServices[d.Service]
		if !ok {
			t.Errorf("unexpected service %s", d.Service)
		}
		if d.Target != target {
			t.Errorf("service %s target mismatch: expected %f, got %f", d.Service, target, d.Target)
		}
		if d.IndicatorType != "availability" {
			t.Errorf("service %s indicator mismatch: expected availability, got %s", d.Service, d.IndicatorType)
		}
		if d.Window != "30d" {
			t.Errorf("service %s window mismatch: expected 30d, got %s", d.Service, d.Window)
		}
		if d.Query == "" {
			t.Errorf("service %s missing query", d.Service)
		}

		snap, snapErr := repo.GetSLOSnapshotBySLOID(ctx, d.ID)
		if snapErr != nil || snap == nil {
			t.Fatalf("missing snapshot for service %s: %v", d.Service, snapErr)
		}
		if snap.Service != d.Service {
			t.Errorf("snapshot service mismatch: expected %s, got %s", d.Service, snap.Service)
		}
		if snap.BudgetStatus != "healthy" {
			t.Errorf("snapshot status mismatch: expected healthy, got %s", snap.BudgetStatus)
		}
		if snap.Actual != 99.94 {
			t.Errorf("snapshot actual mismatch: expected 99.94, got %f", snap.Actual)
		}
	}
}

func TestSeedDefaultSLODefinitions_AlreadySeeded(t *testing.T) {
	ctx := context.Background()
	repo := newMockObsRepo()
	logger := zap.NewNop()

	// Pre-populate with a custom definition
	customDef := observability.SLODefinition{
		ID:            "custom-1",
		Service:       "custom-service",
		Target:        99.0,
		IndicatorType: "latency",
		Window:        "7d",
	}
	_ = repo.CreateSLODefinition(ctx, &customDef)

	err := SeedDefaultSLODefinitions(ctx, repo, logger)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	defs, err := repo.ListSLODefinitions(ctx)
	if err != nil {
		t.Fatalf("failed to list definitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition (unchanged), got %d", len(defs))
	}
	if defs[0].Service != "custom-service" {
		t.Errorf("expected custom-service, got %s", defs[0].Service)
	}
}

func TestSeedDefaultSLODefinitions_NilRepo(t *testing.T) {
	ctx := context.Background()
	err := SeedDefaultSLODefinitions(ctx, nil, zap.NewNop())
	if err != nil {
		t.Fatalf("expected nil error for nil repo, got %v", err)
	}
}

func TestSeedDefaultSLODefinitions_ListError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("db connection failure")
	repo := &errorObsRepo{
		mockObsRepo: *newMockObsRepo(),
		listErr:     expectedErr,
	}

	err := SeedDefaultSLODefinitions(ctx, repo, zap.NewNop())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}
