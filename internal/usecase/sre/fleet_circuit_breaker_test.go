package sre_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/usecase/sre"
)

func TestEvaluateBlastRadius_ClusterRule(t *testing.T) {
	cb := sre.NewFleetCircuitBreaker()

	// 1. Healthy cluster: 1 unready out of 10 nodes (10% <= 15%)
	status, err := cb.EvaluateBlastRadius("cluster-01", 1, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Tripped {
		t.Errorf("expected circuit breaker NOT tripped for 10%% unready ratio, got %+v", status)
	}
	if status.Status != sre.BreakerStatusOK {
		t.Errorf("expected status %s, got %s", sre.BreakerStatusOK, status.Status)
	}

	// 2. Unready ratio > 15%: 2 unready out of 10 nodes (20% > 15%)
	status, err = cb.EvaluateBlastRadius("cluster-01", 2, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Tripped {
		t.Errorf("expected circuit breaker tripped for 20%% unready ratio")
	}
	if status.Status != sre.BreakerStatusClusterBlastRadiusTripped {
		t.Errorf("expected status %s, got %s", sre.BreakerStatusClusterBlastRadiusTripped, status.Status)
	}
	if status.BreakerLevel != sre.BreakerLevelCluster {
		t.Errorf("expected breaker level %s, got %s", sre.BreakerLevelCluster, status.BreakerLevel)
	}
}

func TestEvaluateBlastRadius_FleetRule(t *testing.T) {
	cb := sre.NewFleetCircuitBreaker()
	now := time.Now()

	// Record failures for 5 distinct clusters within 3 minutes
	for i := 1; i <= 5; i++ {
		cb.RecordClusterIncident(fmt.Sprintf("cluster-%02d", i), now.Add(-time.Duration(i)*10*time.Second))
	}

	// 5 clusters failing -> fleet breaker not yet tripped (> 5 clusters required)
	status, err := cb.EvaluateBlastRadius("cluster-05", 1, 20) // unready ratio 5% <= 15%
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Tripped {
		t.Errorf("expected fleet breaker NOT tripped with exactly 5 failing clusters, got %+v", status)
	}

	// Add 6th failing cluster within 3 minutes (> 5 clusters)
	status, err = cb.EvaluateBlastRadius("cluster-06", 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Tripped {
		t.Fatalf("expected fleet circuit breaker TRIPPED when > 5 clusters report simultaneous failures")
	}
	if status.Status != sre.BreakerStatusFleetCorrelatedDisasterTripped {
		t.Errorf("expected status %s, got %s", sre.BreakerStatusFleetCorrelatedDisasterTripped, status.Status)
	}
	if status.BreakerLevel != sre.BreakerLevelFleet {
		t.Errorf("expected level %s, got %s", sre.BreakerLevelFleet, status.BreakerLevel)
	}

	// Old failures (> 3 minutes) should expire and untrip fleet breaker
	oldTime := now.Add(-4 * time.Minute)
	cbClean := sre.NewFleetCircuitBreaker()
	for i := 1; i <= 6; i++ {
		cbClean.RecordClusterIncident(fmt.Sprintf("cluster-%02d", i), oldTime)
	}
	status, err = cbClean.EvaluateBlastRadius("cluster-07", 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Tripped {
		t.Errorf("expected fleet breaker untripped after 3m expiry, got %+v", status)
	}
}
