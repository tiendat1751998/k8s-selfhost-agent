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
	quotas   map[string]map[string]interface{}
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
		quotas: make(map[string]map[string]interface{}),
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

func (m *mockTenancyRepo) DeleteOrganization(ctx context.Context, id string) error {
	var remaining []tenancy.Organization
	for _, o := range m.orgs {
		if o.ID != id {
			remaining = append(remaining, o)
		}
	}
	m.orgs = remaining
	return nil
}

func (m *mockTenancyRepo) CreateProject(ctx context.Context, proj tenancy.Project) error {
	m.projects = append(m.projects, proj)
	return nil
}

func (m *mockTenancyRepo) CreateMember(ctx context.Context, member tenancy.Member) error {
	m.members = append(m.members, member)
	return nil
}

func (m *mockTenancyRepo) UpdateMemberRole(ctx context.Context, id string, role string) error {
	for i := range m.members {
		if m.members[i].ID == id {
			m.members[i].Role = role
			return nil
		}
	}
	return nil
}

func (m *mockTenancyRepo) DeleteMember(ctx context.Context, id string) error {
	var remaining []tenancy.Member
	for _, mem := range m.members {
		if mem.ID != id {
			remaining = append(remaining, mem)
		}
	}
	m.members = remaining
	return nil
}

func (m *mockTenancyRepo) UpdateRBAC(ctx context.Context, rbac map[string]map[string]bool) error {
	m.rbac = rbac
	return nil
}

func (m *mockTenancyRepo) UpdateQuota(ctx context.Context, orgID string, quota map[string]interface{}) error {
	m.quotas[orgID] = quota
	return nil
}

func (m *mockTenancyRepo) GetQuota(ctx context.Context, orgID string) (map[string]interface{}, error) {
	if q, ok := m.quotas[orgID]; ok {
		return q, nil
	}
	return make(map[string]interface{}), nil
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

	// 2b. PUT /tenancy/members/{id}/role
	rolePayload := `{"role":"Developer"}`
	req = httptest.NewRequest(http.MethodPut, "/tenancy/members/mem-2/role", bytes.NewBufferString(rolePayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for PUT /tenancy/members/mem-2/role, got %d: %s", rr.Code, rr.Body.String())
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

	// 5b. GET /tenancy/organizations/{id}/quota
	req = httptest.NewRequest(http.MethodGet, "/tenancy/organizations/org-1/quota", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET quota, got %d: %s", rr.Code, rr.Body.String())
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

func TestTenancyHandler_FallbackWhenNilRepo(t *testing.T) {
	handler := NewTenancyHandler(nil)
	r := chi.NewRouter()
	r.Route("/tenancy", handler.RegisterRoutes)

	req := httptest.NewRequest(http.MethodGet, "/tenancy/organizations", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with nil repo fallback, got %d", rr.Code)
	}

	// Create and delete member in fallback
	memPayload := `{"id":"mem-fb","orgId":"default-tenant","user":"alice","role":"admin","scope":"all"}`
	req = httptest.NewRequest(http.MethodPost, "/tenancy/members", bytes.NewBufferString(memPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 created in fallback, got %d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodDelete, "/tenancy/members/mem-fb", nil)
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 deleted in fallback, got %d", rr.Code)
	}
}

func TestTenancyHandler_CreateMember_AutoAssignsUUID(t *testing.T) {
	handler := NewTenancyHandler(nil)
	r := chi.NewRouter()
	r.Route("/tenancy", handler.RegisterRoutes)

	// 1. Without id -> auto-assigned valid UUID
	payloadNoID := `{"user":"charlie","role":"Developer","scope":"project-wide"}`
	req := httptest.NewRequest(http.MethodPost, "/tenancy/members", bytes.NewBufferString(payloadNoID))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for member without id, got %d: %s", rr.Code, rr.Body.String())
	}
	var created1 tenancy.Member
	if err := json.Unmarshal(rr.Body.Bytes(), &created1); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if created1.ID == "" {
		t.Fatal("expected non-empty ID")
	}

	// 2. With invalid non-UUID -> replaced with valid UUID
	payloadNonUUID := `{"id":"legacy-member-string","user":"dave","role":"Viewer","scope":"global"}`
	req = httptest.NewRequest(http.MethodPost, "/tenancy/members", bytes.NewBufferString(payloadNonUUID))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 for non-uuid member, got %d: %s", rr.Code, rr.Body.String())
	}
	var created2 tenancy.Member
	if err := json.Unmarshal(rr.Body.Bytes(), &created2); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if created2.ID == "" || created2.ID == "legacy-member-string" {
		t.Fatalf("expected non-empty UUID replacing legacy string, got %s", created2.ID)
	}
}
