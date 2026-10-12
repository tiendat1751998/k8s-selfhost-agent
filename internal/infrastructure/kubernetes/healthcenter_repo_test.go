package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/healthcenter"
)

func TestHealthCenterRepo_ConcurrentReadWrite(t *testing.T) {
	repo := NewHealthCenterRepo(nil, nil)
	ctx := context.Background()

	const readers = 50
	const writers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(readers + writers)

	// Launch 50 writer goroutines
	for i := 0; i < writers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				status := &healthcenter.ComponentStatus{
					ID:        fmt.Sprintf("component-%d", id),
					Component: fmt.Sprintf("service-%d", id),
					Status:    "healthy",
					Message:   fmt.Sprintf("iter-%d", j),
				}
				_ = repo.UpdateStatus(ctx, status)
			}
		}(i)
	}

	// Launch 50 reader goroutines
	for i := 0; i < readers; i++ {
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Read via GetStatuses
				statuses, err := repo.GetStatuses(ctx)
				if err != nil {
					t.Errorf("unexpected error from GetStatuses: %v", err)
				}
				if len(statuses) < 4 {
					t.Errorf("expected at least 4 statuses, got %d", len(statuses))
				}

				// Also read via GetAllComponentStatuses and GetComponentStatus if concrete repo
				if hc, ok := repo.(*healthCenterRepo); ok {
					_ = hc.GetAllComponentStatuses()
					_, _ = hc.GetComponentStatus(fmt.Sprintf("component-%d", readerID))
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestHealthCenterRepo_Methods(t *testing.T) {
	repo := NewHealthCenterRepo(nil, nil)
	hc, ok := repo.(*healthCenterRepo)
	if !ok {
		t.Fatalf("expected *healthCenterRepo")
	}

	// 1. Initial get
	st, found := hc.GetComponentStatus("test-comp")
	if found {
		t.Fatalf("expected not found, got %v", st)
	}

	// 2. Update via UpdateComponentStatus
	cs := &healthcenter.ComponentStatus{
		ID:        "test-comp",
		Component: "service-test",
		Status:    "healthy",
		Message:   "all good",
	}
	if err := hc.UpdateComponentStatus(cs); err != nil {
		t.Fatalf("UpdateComponentStatus failed: %v", err)
	}

	// 3. Verify GetComponentStatus
	got, found := hc.GetComponentStatus("test-comp")
	if !found {
		t.Fatalf("expected found")
	}
	if got.Status != "healthy" || got.Message != "all good" {
		t.Errorf("unexpected status: %+v", got)
	}

	// 4. Verify GetAllComponentStatuses
	all := hc.GetAllComponentStatuses()
	if len(all) != 1 {
		t.Fatalf("expected 1 cached status, got %d", len(all))
	}
}

