package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	adapterHttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/automation"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type testAutomationRepo struct {
	rule           *automation.Rule
	createdExec    *automation.Execution
	createdRuleErr error
	createExecErr  error
}

func (m *testAutomationRepo) ListRules(ctx context.Context) ([]automation.Rule, error) {
	return nil, nil
}

func (m *testAutomationRepo) GetRule(ctx context.Context, id string) (*automation.Rule, error) {
	if m.rule != nil {
		return m.rule, nil
	}
	return nil, errors.New("rule not found")
}

func (m *testAutomationRepo) CreateRule(ctx context.Context, r *automation.Rule) error {
	return m.createdRuleErr
}

func (m *testAutomationRepo) UpdateRule(ctx context.Context, r *automation.Rule) error {
	return nil
}

func (m *testAutomationRepo) DeleteRule(ctx context.Context, id string) error {
	return nil
}

func (m *testAutomationRepo) ToggleRule(ctx context.Context, id string, enabled bool) error {
	return nil
}

func (m *testAutomationRepo) ListExecutions(ctx context.Context, limit, offset int) ([]automation.Execution, int, error) {
	return nil, 0, nil
}

func (m *testAutomationRepo) CreateExecution(ctx context.Context, e *automation.Execution) error {
	m.createdExec = e
	return m.createExecErr
}

func TestAutomationHandler_TriggerRule_GeneratesUUID(t *testing.T) {
	ruleID := "e1000001-0000-0000-0000-000000000001"
	repo := &testAutomationRepo{
		rule: &automation.Rule{
			ID:         ruleID,
			Name:       "Auto-Reap Zombie Processes",
			ActionType: automation.ActionRestartPod,
			Enabled:    true,
		},
	}
	handler := adapterHttp.NewAutomationHandler(repo)

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/rules/"+ruleID+"/trigger", nil)
	ctx := tenancy.WithTenantID(req.Context(), "default-tenant")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response JSON: %v", err)
	}

	if resp["status"] != "triggered" {
		t.Fatalf("expected status 'triggered', got %v", resp["status"])
	}

	execID, ok := resp["execution_id"].(string)
	if !ok || execID == "" {
		t.Fatalf("expected execution_id in response, got %v", resp["execution_id"])
	}

	// Verify execution_id is a valid UUID
	if _, err := uuid.Parse(execID); err != nil {
		t.Fatalf("expected execution_id to be valid UUID, got %q: %v", execID, err)
	}

	// Verify repository received an execution with valid UUID ID
	if repo.createdExec == nil {
		t.Fatal("expected CreateExecution to be called")
	}
	if repo.createdExec.ID != execID {
		t.Fatalf("expected repo execution ID %q to match response %q", repo.createdExec.ID, execID)
	}
	if repo.createdExec.RuleID != ruleID {
		t.Fatalf("expected RuleID %q, got %q", ruleID, repo.createdExec.RuleID)
	}
}
