package agent

import (
	"testing"
)

func TestTaskCreation(t *testing.T) {
	task := NewTask(
		"task-123",
		"Phase 11",
		"agents",
		"Testing",
		"Test Agent Task",
		"Verify unit test cases",
		[]string{"dep-1"},
	)

	if task.ID != "task-123" {
		t.Errorf("Expected ID 'task-123', got %s", task.ID)
	}

	if task.Status != TaskPending {
		t.Errorf("Expected Status 'pending', got %s", task.Status)
	}

	if len(task.Dependencies) != 1 || task.Dependencies[0] != "dep-1" {
		t.Errorf("Expected dependencies ['dep-1'], got %v", task.Dependencies)
	}

	// Test default ID generation
	autoTask := NewTask("", "Phase 11", "agents", "Testing", "Auto ID Task", "desc", nil)
	if autoTask.ID == "" {
		t.Errorf("Expected non-empty auto-generated ID")
	}
	if autoTask.Dependencies == nil {
		t.Errorf("Expected non-nil empty dependencies slice")
	}
}

func TestTaskLifecycleTransitions(t *testing.T) {
	task := NewTask("task-lifecycle", "Phase 11", "agents", "Testing", "Lifecycle", "desc", nil)

	// Start
	task.Start()
	if task.Status != TaskInProgress {
		t.Errorf("Expected status inprogress, got %s", task.Status)
	}

	// Block
	task.Block()
	if task.Status != TaskBlocked {
		t.Errorf("Expected status blocked, got %s", task.Status)
	}

	// Fail
	task.Fail()
	if task.Status != TaskFailed {
		t.Errorf("Expected status failed, got %s", task.Status)
	}

	// Complete
	task.Complete()
	if task.Status != TaskSuccess {
		t.Errorf("Expected status success, got %s", task.Status)
	}
	if task.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to be set")
	}
}

func TestSubtaskTransition(t *testing.T) {
	sub := NewSubtask("sub-1", "task-123", "Step 1", 2, 1)

	if sub.Status != SubtaskPending {
		t.Errorf("Expected subtask status 'pending', got %s", sub.Status)
	}

	sub.Start()
	if sub.Status != SubtaskInProgress {
		t.Errorf("Expected subtask status 'inprogress', got %s", sub.Status)
	}

	sub.Complete()
	if sub.Status != SubtaskSuccess {
		t.Errorf("Expected subtask status 'success', got %s", sub.Status)
	}
	if sub.CompletedAt == nil {
		t.Errorf("Expected subtask CompletedAt to be set")
	}

	subFail := NewSubtask("", "task-123", "Step 2", 1, 2)
	if subFail.ID == "" {
		t.Errorf("Expected auto-generated ID for subtask")
	}
	subFail.Fail()
	if subFail.Status != SubtaskFailed {
		t.Errorf("Expected subtask status 'failed', got %s", subFail.Status)
	}
}

func TestExecutionLifecycle(t *testing.T) {
	exec := NewExecution("task-100", BackendEngineer, "input payload")

	if exec.ID == "" {
		t.Errorf("Expected auto-generated execution ID")
	}
	if exec.TaskID != "task-100" {
		t.Errorf("Expected TaskID 'task-100', got %s", exec.TaskID)
	}
	if exec.AgentType != BackendEngineer {
		t.Errorf("Expected AgentType BackendEngineer, got %s", exec.AgentType)
	}
	if exec.Status != ExecutionRunning {
		t.Errorf("Expected status running, got %s", exec.Status)
	}
	if exec.Input != "input payload" {
		t.Errorf("Expected input 'input payload', got %s", exec.Input)
	}
	if exec.CompletedAt != nil {
		t.Errorf("Expected CompletedAt to be nil initially")
	}

	// Test Complete
	exec.Complete("execution output")
	if exec.Status != ExecutionSuccess {
		t.Errorf("Expected status success, got %s", exec.Status)
	}
	if exec.Output != "execution output" {
		t.Errorf("Expected output 'execution output', got %s", exec.Output)
	}
	if exec.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to be set upon completion")
	}

	// Test Fail
	exec2 := NewExecution("task-101", QAEngineer, "qa input")
	exec2.Fail("qa build failed")
	if exec2.Status != ExecutionFailed {
		t.Errorf("Expected status failed, got %s", exec2.Status)
	}
	if exec2.ErrorDetail != "qa build failed" {
		t.Errorf("Expected error detail 'qa build failed', got %s", exec2.ErrorDetail)
	}
	if exec2.CompletedAt == nil {
		t.Errorf("Expected CompletedAt to be set upon failure")
	}
}

func TestProjectStateMethods(t *testing.T) {
	state := &ProjectState{
		ID: "latest",
	}

	task := NewTask("task-abc", "Phase 11", "agents", "feature-x", "Title", "Desc", nil)
	state.SetCurrentTask(task)

	if state.CurrentPhase != "Phase 11" || state.CurrentModule != "agents" || state.CurrentFeature != "feature-x" || state.CurrentTaskID != "task-abc" {
		t.Errorf("SetCurrentTask did not set fields correctly: %+v", state)
	}

	state.ClearCurrentTask()
	if state.CurrentTaskID != "" || state.CurrentSubtaskID != "" {
		t.Errorf("ClearCurrentTask did not clear task IDs: %+v", state)
	}

	state.UpdateMetrics(95.0, 5.0, 90.0, 92.0)
	if state.RepositoryHealth != 95.0 || state.TechnicalDebt != 5.0 || state.ArchitectureScore != 90.0 || state.QualityScore != 92.0 {
		t.Errorf("UpdateMetrics did not update metrics correctly: %+v", state)
	}
}
