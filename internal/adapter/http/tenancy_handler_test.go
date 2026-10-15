package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/tenancy"
)

type mockTenancyRepo struct {
	orgs     []tenancy.Organization
	projects []tenancy.Project
	members  []tenancy.Member
	rbac     map[string]map[string]bool
}

func newMockTenancyRepo() *mockTenancyRepo {
	return &mockTenancyRepo{
		orgs: []tenancy.Organization{
			{ID: "org-1", Name: "Engineering", Tier: "enterprise"},
		},
		projects: []tenancy.Project{
			{ID: "proj-1", OrgID: "org-1", Name: "Core API", Envs: []string{"prod"}, Workloads: 5},
		},
		members: []tenancy.Member{
			{ID: "mem-1", OrgID: "org-1", User: "alice", Role: "Admin", Scope: "global"},
		},
		rbac: map[string]map[string]bool{
			"Admin": {"pods:read": true},
		},
	}
}

func (m *mockTenancyRepo) GetOrganizations(ctx context.Context) ([]tenancy.Organization, error) {
	return m.orgs, nil
}

func (m *mockTenancyRepo) GetProjects(ctx context.Context) ([]tenancy.Project, error) {
	return m.projects, nil
}

func (m *mockTenancyRepo) GetMembers(ctx context.Context) ([]tenancy.Member, error) {
	return m.members, nil
}

func (m *mockTenancyRepo) GetRBAC(ctx context.Context) (map[string]map[string]bool, error) {
	return m.rbac, nil
}

func (m *mockTenancyRepo) CreateOrganization(ctx context.Context, org tenancy.Organization) error {
	m.orgs = append(m.orgs, org)
	return nil
}

func (m *mockTenancyRepo) CreateProject(ctx context.Context, proj tenancy.Project) error {
	m.projects = append(m.projects, proj)
	return nil
}

func TestTenancyHandler_MissingRoutes(t *testing.T) {
	repo := newMockTenancyRepo()
	handler := NewTenancyHandler(repo)

	r := chi.NewRouter()
	r.Route("/tenancy", handler.RegisterRoutes)

	// 1. POST /tenancy/members
	memPayload := `{"id":"mem-2","orgId":"org-1","user":"bob","role":"Viewer","scope":"project-1"}`
	req := httptest.NewRequest(http.MethodPost, "/tenancy/members", bytes.NewBufferString(memPayload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for POST /tenancy/members, got %d: %s", rr.Code, rr.Body.String())
	}

	// 2. GET /tenancy/members contains bob
	req = httptest.NewRequest(http.MethodGet, "/tenancy/members", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var members []tenancy.Member
	if err := json.Unmarshal(rr.Body.Bytes(), &members); err != nil {
		t.Fatalf("failed to decode members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// 3. DELETE /tenancy/members/{id}
	req = httptest.NewRequest(http.MethodDelete, "/tenancy/members/mem-2", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE /tenancy/members/mem-2, got %d: %s", rr.Code, rr.Body.String())
	}

	// 4. PUT /tenancy/rbac
	rbacPayload := `{"DevOps":{"pods:write":true,"pods:read":true}}`
	req = httptest.NewRequest(http.MethodPut, "/tenancy/rbac", bytes.NewBufferString(rbacPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for PUT /tenancy/rbac, got %d: %s", rr.Code, rr.Body.String())
	}

	// 5. PUT /tenancy/organizations/{id}/quota
	quotaPayload := `{"max_cpu":16,"max_memory_gb":64}`
	req = httptest.NewRequest(http.MethodPut, "/tenancy/organizations/org-1/quota", bytes.NewBufferString(quotaPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for PUT /tenancy/organizations/org-1/quota, got %d: %s", rr.Code, rr.Body.String())
	}

	// 6. DELETE /tenancy/organizations/{id}
	req = httptest.NewRequest(http.MethodDelete, "/tenancy/organizations/org-1", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for DELETE /tenancy/organizations/org-1, got %d: %s", rr.Code, rr.Body.String())
	}

	// 7. GET /tenancy/organizations should now exclude org-1
	req = httptest.NewRequest(http.MethodGet, "/tenancy/organizations", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var orgs []tenancy.Organization
	if err := json.Unmarshal(rr.Body.Bytes(), &orgs); err != nil {
		t.Fatalf("failed to decode orgs: %v", err)
	}
	if len(orgs) != 0 {
		t.Fatalf("expected 0 orgs after delete, got %d", len(orgs))
	}
}
