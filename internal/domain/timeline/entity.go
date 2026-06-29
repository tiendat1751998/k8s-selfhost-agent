// Package timeline provides domain entities for the deployment timeline.
package timeline

import "time"

// EventType represents a deployment event type.
type EventType string

const (
	EventDeployment EventType = "deployment"
	EventRollback   EventType = "rollback"
	EventIncident   EventType = "incident"
	EventCommit     EventType = "commit"
	EventConfig     EventType = "config"
)

// Event represents a deployment timeline event.
type Event struct {
	ID        string            `json:"id"`
	Type      EventType         `json:"type"`
	Title     string            `json:"title"`
	Detail    string            `json:"detail"`
	Namespace string            `json:"namespace,omitempty"`
	Cluster   string            `json:"cluster,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}
