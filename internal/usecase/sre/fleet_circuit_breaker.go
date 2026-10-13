package sre

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Circuit breaker status constants.
const (
	BreakerStatusOK                           = "OK"
	BreakerStatusClusterBlastRadiusTripped     = "CLUSTER_BLAST_RADIUS_TRIPPED"
	BreakerStatusFleetCorrelatedDisasterTripped = "FLEET_CORRELATED_DISASTER_TRIPPED"

	BreakerLevelNone    = "NONE"
	BreakerLevelCluster = "CLUSTER"
	BreakerLevelFleet   = "FLEET"
)

// CircuitBreakerStatus captures the state of cluster and fleet blast-radius circuit breakers.
type CircuitBreakerStatus struct {
	Tripped               bool    `json:"tripped"`
	BreakerLevel          string  `json:"breaker_level"`
	Status                string  `json:"status"`
	Reason                string  `json:"reason"`
	UnreadyRatio          float64 `json:"unready_ratio"`
	UnreadyNodes          int     `json:"unready_nodes"`
	TotalNodes            int     `json:"total_nodes"`
	ActiveFailingClusters int     `json:"active_failing_clusters"`
}

// FleetCircuitBreaker tracks failure incidents across clusters and trips circuit breakers
// to prevent catastrophic correlated remediation storms and cascading fleet outages.
type FleetCircuitBreaker struct {
	mu                  sync.RWMutex
	incidentWindow      time.Duration
	clusterFailureRatio float64
	fleetDisasterLimit  int
	incidents           map[string]time.Time
}

// NewFleetCircuitBreaker constructs a default FleetCircuitBreaker.
func NewFleetCircuitBreaker() *FleetCircuitBreaker {
	return &FleetCircuitBreaker{
		incidentWindow:      3 * time.Minute,
		clusterFailureRatio: 0.15, // 15% threshold
		fleetDisasterLimit:  5,    // > 5 clusters
		incidents:           make(map[string]time.Time),
	}
}

// RecordClusterIncident records a failure event for a cluster to track fleet-wide correlation.
func (b *FleetCircuitBreaker) RecordClusterIncident(clusterID string, timestamp time.Time) {
	if clusterID == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.incidents[clusterID] = timestamp
}

// ClearClusterIncident removes a resolved cluster from incident tracking.
func (b *FleetCircuitBreaker) ClearClusterIncident(clusterID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.incidents, clusterID)
}

// EvaluateBlastRadius evaluates both cluster unready ratio (>15%) and fleet correlated failures (>5 clusters).
func (b *FleetCircuitBreaker) EvaluateBlastRadius(clusterID string, unreadyNodes, totalNodes int) (*CircuitBreakerStatus, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	// Purge incidents outside the 3-minute window
	for id, ts := range b.incidents {
		if now.Sub(ts) > b.incidentWindow {
			delete(b.incidents, id)
		}
	}

	// If current cluster has unready nodes, record it
	if unreadyNodes > 0 && clusterID != "" {
		b.incidents[clusterID] = now
	}

	activeCount := len(b.incidents)

	// 1. Fleet Rule: if > 5 clusters report simultaneous failures within 3 minutes,
	// trip fleet circuit breaker, pausing autonomous fleet-wide remediation.
	if activeCount > b.fleetDisasterLimit {
		return &CircuitBreakerStatus{
			Tripped:               true,
			BreakerLevel:          BreakerLevelFleet,
			Status:                BreakerStatusFleetCorrelatedDisasterTripped,
			Reason:                fmt.Sprintf("Fleet correlated disaster tripped: %d clusters reporting failures within 3m window (limit %d)", activeCount, b.fleetDisasterLimit),
			UnreadyNodes:          unreadyNodes,
			TotalNodes:            totalNodes,
			ActiveFailingClusters: activeCount,
		}, nil
	}

	// 2. Cluster Rule: if unready ratio > 15% and unreadyNodes > 1, trip cluster circuit breaker,
	// freezing mass automated evictions while allowing isolated single-node recovery.
	var ratio float64
	if totalNodes > 0 {
		ratio = float64(unreadyNodes) / float64(totalNodes)
		ratio = math.Round(ratio*1000) / 1000
	}

	if totalNodes > 0 && unreadyNodes > 1 && ratio > b.clusterFailureRatio {
		return &CircuitBreakerStatus{
			Tripped:               true,
			BreakerLevel:          BreakerLevelCluster,
			Status:                BreakerStatusClusterBlastRadiusTripped,
			Reason:                fmt.Sprintf("Cluster blast radius tripped: %.1f%% nodes unready (>%.0f%% limit)", ratio*100, b.clusterFailureRatio*100),
			UnreadyRatio:          ratio,
			UnreadyNodes:          unreadyNodes,
			TotalNodes:            totalNodes,
			ActiveFailingClusters: activeCount,
		}, nil
	}

	return &CircuitBreakerStatus{
		Tripped:               false,
		BreakerLevel:          BreakerLevelNone,
		Status:                BreakerStatusOK,
		Reason:                "Cluster and fleet blast-radius metrics within safety bounds",
		UnreadyRatio:          ratio,
		UnreadyNodes:          unreadyNodes,
		TotalNodes:            totalNodes,
		ActiveFailingClusters: activeCount,
	}, nil
}
