package backup

import (
	"encoding/json"
	"time"
)

// BackupLog represents a single disaster recovery task record in history.
type BackupLog struct {
	ID        string          `json:"id"`
	Timestamp time.Time       `json:"timestamp"`
	Action    string          `json:"action"` // backup | restore
	Target    string          `json:"target"`
	Status    string          `json:"status"` // success | failed | running
	Duration  string          `json:"duration"`
	Size      string          `json:"size"`
	Details   json.RawMessage `json:"details"`
}
