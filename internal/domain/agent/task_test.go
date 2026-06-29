package agent

import (
	"testing"
	"time"
)

func TestTaskCreation(t *testing.T) {
	task := Task{
		ID:          "task-123",
		Phase:       "Phase 11",
		Module:      "agents",
		Feature:     "Testing",
		Title:       "Test Agent Task",
		Description: "Verify unit test cases",
		Status:      TaskPending,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if task.ID != "task-123" {
		t.Errorf("Expected ID 'task-123', got %s", task.ID)
	}

	if task.Status != TaskPending {
		t.Errorf("Expected Status 'pending', got %s", task.Status)
	}
}

func TestSubtaskTransition(t *testing.T) {
	sub := Subtask{
		ID:         "sub-1",
		TaskID:     "task-123",
		Title:      "Step 1",
		Status:     SubtaskPending,
		Complexity: 2,
		ExecOrder:  1,
	}

	if sub.Status != SubtaskPending {
		t.Errorf("Expected subtask status 'pending', got %s", sub.Status)
	}

	sub.Status = SubtaskSuccess
	now := time.Now().UTC()
	sub.CompletedAt = &now

	if sub.Status != SubtaskSuccess {
		t.Errorf("Expected subtask status 'success', got %s", sub.Status)
	}

	if sub.CompletedAt == nil {
		t.Errorf("Expected subtask CompletedAt to be set")
	}
}
