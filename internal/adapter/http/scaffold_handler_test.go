package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
	usecaseScaffold "github.com/datdt/k8sselfhost/internal/usecase/scaffold"
)

type mockScaffoldRepoForHandler struct {
	templates map[string]*scaffold.Template
	failErr   error
}

func newMockScaffoldRepo() *mockScaffoldRepoForHandler {
	m := &mockScaffoldRepoForHandler{
		templates: make(map[string]*scaffold.Template),
	}
	for _, b := range scaffold.GetBuiltinTemplates() {
		cp := b
		m.templates[b.ID] = &cp
	}
	return m
}

func (m *mockScaffoldRepoForHandler) Create(ctx context.Context, t *scaffold.Template) error {
	if m.failErr != nil {
		return m.failErr
	}
	if t.ID == "" {
		t.ID = "tpl-mock-123"
	}
	m.templates[t.ID] = t
	return nil
}

func (m *mockScaffoldRepoForHandler) GetByID(ctx context.Context, id string) (*scaffold.Template, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	tmpl, ok := m.templates[id]
	if !ok {
		return nil, nil
	}
	cp := *tmpl
	return &cp, nil
}

func (m *mockScaffoldRepoForHandler) List(ctx context.Context, tenantID string, filter scaffold.ListFilter) ([]scaffold.Template, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	list := make([]scaffold.Template, 0)
	for _, t := range m.templates {
		if filter.Category != "" && t.Category != filter.Category {
			continue
		}
		if filter.Framework != "" && t.Framework != filter.Framework {
			continue
		}
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(t.Name), term) && !strings.Contains(strings.ToLower(t.Description), term) {
				continue
			}
		}
		list = append(list, *t)
	}
	return list, nil
}

func (m *mockScaffoldRepoForHandler) Update(ctx context.Context, t *scaffold.Template) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.templates[t.ID] = t
	return nil
}

func (m *mockScaffoldRepoForHandler) Delete(ctx context.Context, id string) error {
	if m.failErr != nil {
		return m.failErr
	}
	delete(m.templates, id)
	return nil
}

func setupScaffoldTestHandler() (*ScaffoldHandler, *mockScaffoldRepoForHandler) {
	repo := newMockScaffoldRepo()
	service := usecaseScaffold.NewService(repo, nil, usecaseScaffold.NewEngine())
	handler := NewScaffoldHandler(service, zap.NewNop())
	return handler, repo
}

func TestScaffoldHandler_ListTemplates(t *testing.T) {
	handler, _ := setupScaffoldTestHandler()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var list []scaffold.Template
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(list) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(list))
	}
}

func TestScaffoldHandler_GetTemplate_FoundAndNotFound(t *testing.T) {
	handler, _ := setupScaffoldTestHandler()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Found
	req := httptest.NewRequest(http.MethodGet, "/templates/"+scaffold.BuiltinIDGoAPI, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var tmpl scaffold.Template
	if err := json.NewDecoder(rec.Body).Decode(&tmpl); err != nil {
		t.Fatalf("failed to decode template: %v", err)
	}
	if tmpl.ID != scaffold.BuiltinIDGoAPI {
		t.Errorf("expected ID %s, got %s", scaffold.BuiltinIDGoAPI, tmpl.ID)
	}

	// Not Found
	reqNF := httptest.NewRequest(http.MethodGet, "/templates/non-existent-id", nil)
	recNF := httptest.NewRecorder()
	r.ServeHTTP(recNF, reqNF)

	if recNF.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", recNF.Code)
	}
}

func TestScaffoldHandler_CreateTemplate_SuccessAndValidation(t *testing.T) {
	handler, _ := setupScaffoldTestHandler()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Valid creation
	payload := createTemplateRequest{
		Name:        "Vue SPA Frontend",
		Description: "Vue 3 Vite frontend application",
		Category:    "web",
		Framework:   "vue",
		ManifestYAML: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: {{.app_name}}",
		Variables: []scaffold.TemplateVariable{
			{Name: "app_name", Default: "my-vue-app", Required: true},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created scaffold.Template
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created template: %v", err)
	}
	if created.Name != "Vue SPA Frontend" {
		t.Errorf("expected name 'Vue SPA Frontend', got %s", created.Name)
	}

	// Validation error: missing name
	invalidPayload := createTemplateRequest{
		Description: "Missing name",
	}
	invalidBody, _ := json.Marshal(invalidPayload)
	reqInv := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewReader(invalidBody))
	reqInv.Header.Set("Content-Type", "application/json")
	recInv := httptest.NewRecorder()
	r.ServeHTTP(recInv, reqInv)

	if recInv.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing name, got %d", recInv.Code)
	}
}

func TestScaffoldHandler_UpdateAndDeleteTemplate(t *testing.T) {
	handler, repo := setupScaffoldTestHandler()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	custom := &scaffold.Template{
		ID:          "custom-123",
		Name:        "Original Name",
		Description: "Original Desc",
		Category:    "api",
		BuiltIn:     false,
	}
	_ = repo.Create(context.Background(), custom)

	// Update custom template
	updatePayload := updateTemplateRequest{
		Name:        "Updated Custom Name",
		Description: "Updated Desc",
		Category:    "api",
	}
	body, _ := json.Marshal(updatePayload)
	req := httptest.NewRequest(http.MethodPut, "/templates/custom-123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on update, got %d: %s", rec.Code, rec.Body.String())
	}

	// Reject updating built-in
	reqBuiltin := httptest.NewRequest(http.MethodPut, "/templates/"+scaffold.BuiltinIDGoAPI, bytes.NewReader(body))
	reqBuiltin.Header.Set("Content-Type", "application/json")
	recBuiltin := httptest.NewRecorder()
	r.ServeHTTP(recBuiltin, reqBuiltin)

	if recBuiltin.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when updating built-in, got %d", recBuiltin.Code)
	}

	// Delete custom template
	reqDel := httptest.NewRequest(http.MethodDelete, "/templates/custom-123", nil)
	recDel := httptest.NewRecorder()
	r.ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 on delete, got %d", recDel.Code)
	}

	// Reject deleting built-in
	reqDelBuiltin := httptest.NewRequest(http.MethodDelete, "/templates/"+scaffold.BuiltinIDGoAPI, nil)
	recDelBuiltin := httptest.NewRecorder()
	r.ServeHTTP(recDelBuiltin, reqDelBuiltin)

	if recDelBuiltin.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 when deleting built-in, got %d", recDelBuiltin.Code)
	}
}

func TestScaffoldHandler_Render(t *testing.T) {
	handler, _ := setupScaffoldTestHandler()

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Successful render of built-in Go API template
	renderReq := renderRequestPayload{
		TemplateID: scaffold.BuiltinIDGoAPI,
		Variables: map[string]string{
			"app_name": "payments-svc",
			"port":     "8080",
			"replicas": "3",
		},
	}
	body, _ := json.Marshal(renderReq)
	req := httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 on render, got %d: %s", rec.Code, rec.Body.String())
	}

	var res usecaseScaffold.RenderResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode render response: %v", err)
	}
	if !strings.Contains(res.RenderedYAML, "name: payments-svc") {
		t.Errorf("rendered YAML missing app_name: %s", res.RenderedYAML)
	}
}
