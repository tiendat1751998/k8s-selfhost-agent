// Package automation provides domain entities for workflow automation.
package automation

import (
	"time"

	"github.com/google/uuid"
)

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

// NewRule creates a new enabled Rule with default empty configurations.
func NewRule(name string, triggerType TriggerType, actionType ActionType) *Rule {
	now := time.Now().UTC()
	return &Rule{
		ID:            uuid.New().String(),
		Name:          name,
		TriggerType:   triggerType,
		TriggerConfig: make(map[string]string),
		ActionType:    actionType,
		ActionConfig:  make(map[string]string),
		Enabled:       true,
		Executions:    0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Enable activates the automation rule.
func (r *Rule) Enable() {
	r.Enabled = true
	r.UpdatedAt = time.Now().UTC()
}

// Disable deactivates the automation rule.
func (r *Rule) Disable() {
	r.Enabled = false
	r.UpdatedAt = time.Now().UTC()
}

// Toggle sets the enabled state of the automation rule.
func (r *Rule) Toggle(enabled bool) {
	r.Enabled = enabled
	r.UpdatedAt = time.Now().UTC()
}

// IncrementCounter bumps the execution count and records the last triggered timestamp.
func (r *Rule) IncrementCounter() {
	r.Executions++
	now := time.Now().UTC()
	r.LastTriggered = &now
	r.UpdatedAt = now
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

// NewExecution creates a new Execution record for a rule event and action.
func NewExecution(ruleID, event, action string) *Execution {
	return &Execution{
		ID:           uuid.New().String(),
		RuleID:       ruleID,
		TriggerEvent: event,
		ActionTaken:  action,
		Result:       "success",
		CreatedAt:    time.Now().UTC(),
	}
}

// MarkSuccess sets the execution result to success and clears any error details.
func (e *Execution) MarkSuccess() {
	e.Result = "success"
	e.ErrorDetail = ""
}

// MarkFailure sets the execution result to failure and records the error detail.
func (e *Execution) MarkFailure(errDetail string) {
	e.Result = "failure"
	e.ErrorDetail = errDetail
}

