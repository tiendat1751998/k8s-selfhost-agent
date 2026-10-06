package kubernetes

import (
	"context"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/capacity"
)

func TestBuildLiveForecasts_InsufficientHistoricalData(t *testing.T) {
	cluster := "test-k8s"
	cpuPercent := 42.5
	memPercent := 68.0
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

	forecasts := buildLiveForecasts(cluster, cpuPercent, memPercent, now)

	if len(forecasts) != 2 {
		t.Fatalf("expected 2 forecasts, got %d", len(forecasts))
	}

	// 1. CPU Forecast verification
	cpuFC := forecasts[0]
	if cpuFC.ResourceType != capacity.ResourceCPU {
		t.Errorf("expected ResourceCPU, got %s", cpuFC.ResourceType)
	}
	if cpuFC.CurrentUsage != cpuPercent {
		t.Errorf("expected CurrentUsage %.2f, got %.2f", cpuPercent, cpuFC.CurrentUsage)
	}
	if cpuFC.Forecast7d != 0 {
		t.Errorf("expected Forecast7d = 0 for insufficient data, got %.2f", cpuFC.Forecast7d)
	}
	if cpuFC.Forecast30d != 0 {
		t.Errorf("expected Forecast30d = 0 for insufficient data, got %.2f", cpuFC.Forecast30d)
	}
	if cpuFC.Forecast90d != 0 {
		t.Errorf("expected Forecast90d = 0 for insufficient data, got %.2f", cpuFC.Forecast90d)
	}
	if cpuFC.ExhaustionAt != nil {
		t.Errorf("expected nil ExhaustionAt without historical data, got %v", cpuFC.ExhaustionAt)
	}
	if cpuFC.Status != "insufficient_data" {
		t.Errorf("expected status 'insufficient_data', got %s", cpuFC.Status)
	}
	if !cpuFC.RecordedAt.Equal(now) {
		t.Errorf("expected RecordedAt %v, got %v", now, cpuFC.RecordedAt)
	}

	// 2. Memory Forecast verification
	memFC := forecasts[1]
	if memFC.ResourceType != capacity.ResourceMemory {
		t.Errorf("expected ResourceMemory, got %s", memFC.ResourceType)
	}
	if memFC.CurrentUsage != memPercent {
		t.Errorf("expected CurrentUsage %.2f, got %.2f", memPercent, memFC.CurrentUsage)
	}
	if memFC.Forecast7d != 0 {
		t.Errorf("expected Forecast7d = 0 for insufficient data, got %.2f", memFC.Forecast7d)
	}
	if memFC.Forecast30d != 0 {
		t.Errorf("expected Forecast30d = 0 for insufficient data, got %.2f", memFC.Forecast30d)
	}
	if memFC.Forecast90d != 0 {
		t.Errorf("expected Forecast90d = 0 for insufficient data, got %.2f", memFC.Forecast90d)
	}
	if memFC.ExhaustionAt != nil {
		t.Errorf("expected nil ExhaustionAt without historical data, got %v", memFC.ExhaustionAt)
	}
	if memFC.Status != "insufficient_data" {
		t.Errorf("expected status 'insufficient_data', got %s", memFC.Status)
	}
}

func TestCapacityRepo_NilClient(t *testing.T) {
	repo := NewCapacityRepo(nil, nil)
	_, err := repo.List(context.Background(), "cluster-x")
	if err == nil {
		t.Fatal("expected error with uninitialized client, got nil")
	}

	err = repo.Record(context.Background(), &capacity.Forecast{})
	if err == nil {
		t.Fatal("expected error for Record, got nil")
	}
}
