package automation_test

import (
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/automation"
)

func TestNewRule(t *testing.T) {
	rule := automation.NewRule("pod-restart-rca", automation.TriggerPodRestart, automation.ActionGenerateRCA)

	if rule.ID == "" {
		t.Error("expected non-empty ID")
	}
	if rule.Name != "pod-restart-rca" {
		t.Errorf("expected name 'pod-restart-rca', got '%s'", rule.Name)
	}
	if rule.TriggerType != automation.TriggerPodRestart {
		t.Errorf("expected trigger type '%s', got '%s'", automation.TriggerPodRestart, rule.TriggerType)
	}
	if rule.ActionType != automation.ActionGenerateRCA {
		t.Errorf("expected action type '%s', got '%s'", automation.ActionGenerateRCA, rule.ActionType)
	}
	if !rule.Enabled {
		t.Error("expected new rule to be enabled by default")
	}
	if rule.Executions != 0 {
		t.Errorf("expected 0 executions, got %d", rule.Executions)
	}
	if rule.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if rule.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestRule_ToggleAndCounter(t *testing.T) {
	rule := automation.NewRule("high-cpu-scale", automation.TriggerHighCPU, automation.ActionScaleDeployment)

	// 1. Disable
	rule.Disable()
	if rule.Enabled {
		t.Error("expected rule to be disabled")
	}

	// 2. Enable
	rule.Enable()
	if !rule.Enabled {
		t.Error("expected rule to be enabled")
	}

	// 3. Toggle
	rule.Toggle(false)
	if rule.Enabled {
		t.Error("expected rule to be toggled off")
	}
	rule.Toggle(true)
	if !rule.Enabled {
		t.Error("expected rule to be toggled on")
	}

	// 4. IncrementCounter
	if rule.Executions != 0 {
		t.Errorf("expected initial 0 executions, got %d", rule.Executions)
	}
	if rule.LastTriggered != nil {
		t.Error("expected LastTriggered to be nil initially")
	}

	rule.IncrementCounter()
	if rule.Executions != 1 {
		t.Errorf("expected 1 execution, got %d", rule.Executions)
	}
	if rule.LastTriggered == nil {
		t.Error("expected LastTriggered to be set after IncrementCounter")
	}

	rule.IncrementCounter()
	if rule.Executions != 2 {
		t.Errorf("expected 2 executions, got %d", rule.Executions)
	}
}

func TestNewExecution(t *testing.T) {
	exec := automation.NewExecution("rule-1", "pod_crash_loop", "restart_pod")

	if exec.ID == "" {
		t.Error("expected non-empty ID")
	}
	if exec.RuleID != "rule-1" {
		t.Errorf("expected rule ID 'rule-1', got '%s'", exec.RuleID)
	}
	if exec.TriggerEvent != "pod_crash_loop" {
		t.Errorf("expected trigger event 'pod_crash_loop', got '%s'", exec.TriggerEvent)
	}
	if exec.ActionTaken != "restart_pod" {
		t.Errorf("expected action taken 'restart_pod', got '%s'", exec.ActionTaken)
	}
	if exec.Result != "success" {
		t.Errorf("expected default result 'success', got '%s'", exec.Result)
	}
	if exec.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestExecution_StatusMethods(t *testing.T) {
	exec := automation.NewExecution("rule-1", "node_pressure", "cordon_node")

	// 1. MarkFailure
	exec.MarkFailure("insufficient permissions to cordon node")
	if exec.Result != "failure" {
		t.Errorf("expected result 'failure', got '%s'", exec.Result)
	}
	if exec.ErrorDetail != "insufficient permissions to cordon node" {
		t.Errorf("expected ErrorDetail 'insufficient permissions to cordon node', got '%s'", exec.ErrorDetail)
	}

	// 2. MarkSuccess
	exec.MarkSuccess()
	if exec.Result != "success" {
		t.Errorf("expected result 'success', got '%s'", exec.Result)
	}
	if exec.ErrorDetail != "" {
		t.Errorf("expected empty ErrorDetail, got '%s'", exec.ErrorDetail)
	}
}
