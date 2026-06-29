package healthcenter

import "time"

// ComponentStatus represents the health status of a platform component.
type ComponentStatus struct {
	ID            string    `json:"id"`
	Component     string    `json:"component"`      // frontend | backend | websocket | ai | gitops | database
	Status        string    `json:"status"`         // healthy | degraded | down
	Message       string    `json:"message"`
	LastCheckedAt time.Time `json:"last_checked_at"`
}
