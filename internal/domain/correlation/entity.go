// Package correlation provides domain entities for the event correlation engine.
package correlation

import "time"

// CorrelatedEvent groups related signals into a probable root cause chain.
type CorrelatedEvent struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	RootCause   string   `json:"root_cause"`
	Severity    string   `json:"severity"` // critical | high | medium | low
	EventIDs    []string `json:"event_ids"`
	EventCount  int      `json:"event_count"`
	Cluster     string   `json:"cluster"`
	Namespace   string   `json:"namespace"`
	Status      string   `json:"status"` // active | resolved
	CorrelatedAt time.Time `json:"correlated_at"`
}

// Signal represents a raw event signal (log, metric, K8s event, trace).
type Signal struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // log | metric | k8s_event | trace | incident
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}
