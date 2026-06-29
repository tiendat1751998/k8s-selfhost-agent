// Package automation provides domain entities for workflow automation.
package automation

import "time"

// TriggerType represents a rule trigger type.
type TriggerType string

const (
	TriggerPodRestart        TriggerType = "pod_restart"
	TriggerNodePressure      TriggerType = "node_pressure"
	TriggerDeploymentFailure TriggerType = "deployment_failure"
	TriggerHighCPU           TriggerType = "high_cpu"
	TriggerHighMemory        TriggerType = "high_memory"
	TriggerSLOBreach         TriggerType = "slo_breach"
	TriggerErrorRate         TriggerType = "error_rate"
)

// ActionType represents a rule action type.
type ActionType string

const (
	ActionGenerateRCA      ActionType = "generate_rca"
	ActionSendNotification ActionType = "send_notification"
	ActionRollback         ActionType = "rollback"
	ActionScaleDeployment  ActionType = "scale_deployment"
	ActionCreateIncident   ActionType = "create_incident"
	ActionCordonNode       ActionType = "cordon_node"
	ActionRestartPod       ActionType = "restart_pod"
)

// Rule represents an automation rule.
type Rule struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	TriggerType   TriggerType       `json:"trigger_type"`
	TriggerConfig map[string]string `json:"trigger_config"`
	ActionType    ActionType        `json:"action_type"`
	ActionConfig  map[string]string `json:"action_config"`
	Enabled       bool              `json:"enabled"`
	Executions    int               `json:"executions"`
	LastTriggered *time.Time        `json:"last_triggered,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// Execution represents an automation rule execution record.
type Execution struct {
	ID           string    `json:"id"`
	RuleID       string    `json:"rule_id"`
	RuleName     string    `json:"rule_name"`
	TriggerEvent string    `json:"trigger_event"`
	ActionTaken  string    `json:"action_taken"`
	Result       string    `json:"result"` // success | failure
	ErrorDetail  string    `json:"error_detail,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
