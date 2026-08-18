package telegram

import (
	"time"
)

type AlertPayload struct {
	IncidentID  string    `json:"incident_id"`
	Fingerprint string    `json:"fingerprint"`
	Severity    string    `json:"severity"` // CRITICAL, WARNING, INFO
	Cluster     string    `json:"cluster"`
	Namespace   string    `json:"namespace"`
	Service     string    `json:"service"`
	Pod         string    `json:"pod"`
	TargetType  string    `json:"target_type"` // kubernetes, docker
	Message     string    `json:"message"`
	RCAAnalysis string    `json:"rca_analysis,omitempty"`
	BackupJobID string    `json:"backup_job_id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Count       int       `json:"count"`
}

type InlineActionButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type TelegramConfig struct {
	BotToken            string
	AdminChatIDs        []int64
	EnableDeduplication bool
	DebounceWindow      time.Duration
}

type CallbackActionPayload struct {
	Action      string `json:"action"` // restart, rollback, restore_db
	Cluster     string `json:"cluster"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	TargetType  string `json:"target_type"`
	BackupJobID string `json:"backup_job_id,omitempty"`
}
