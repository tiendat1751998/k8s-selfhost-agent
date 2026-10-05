package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestSearchQueryBuilder_ValidateSearchType(t *testing.T) {
	builder := NewSearchQueryBuilder()

	validTypes := []string{"all", "incident", "kubernetes", "log", "git"}
	for _, st := range validTypes {
		if err := builder.ValidateSearchType(st); err != nil {
			t.Errorf("expected valid search type %s, got error: %v", st, err)
		}
	}

	invalidTypes := []string{"", "user", "drop_table", "injection'--"}
	for _, st := range invalidTypes {
		if err := builder.ValidateSearchType(st); err == nil {
			t.Errorf("expected error for invalid search type %s, got nil", st)
		}
	}
}

func TestSearchQueryBuilder_BuildQueries_TenantIsolation(t *testing.T) {
	builder := NewSearchQueryBuilder()
	ctx := context.WithValue(context.Background(), tenancy.TenantIDKey, "tenant-alpha")

	queries, err := builder.BuildQueries(ctx, "billing-service", "all")
	if err != nil {
		t.Fatalf("unexpected error building queries: %v", err)
	}

	if len(queries) != 4 {
		t.Fatalf("expected 4 compiled queries for searchType 'all', got %d", len(queries))
	}

	domains := make(map[SearchDomain]CompiledSearchQuery)
	for _, q := range queries {
		domains[q.Domain] = q
	}

	// Verify Incident query contains tenant filter
	incQuery, ok := domains[SearchDomainIncident]
	if !ok {
		t.Fatal("expected incident query compiled")
	}
	if incQuery.SQL == "" || len(incQuery.Args) < 2 {
		t.Errorf("incident query missing tenant args: %+v", incQuery)
	}

	// Verify Cluster query contains tenant filter
	clusterQuery, ok := domains[SearchDomainKubernetes]
	if !ok {
		t.Fatal("expected cluster query compiled")
	}
	if clusterQuery.SQL == "" || len(clusterQuery.Args) < 2 {
		t.Errorf("cluster query missing tenant args: %+v", clusterQuery)
	}

	// Verify Audit Log query contains tenant filter
	auditQuery, ok := domains[SearchDomainLog]
	if !ok {
		t.Fatal("expected audit log query compiled")
	}
	expectedAuditPredicate := "WHERE (action ILIKE '%' || $1 || '%' OR target_name ILIKE '%' || $1 || '%' OR actor ILIKE '%' || $1 || '%') AND tenant_id = $2"
	if !strings.Contains(auditQuery.SQL, expectedAuditPredicate) {
		t.Errorf("audit query missing parenthesized tenant predicate:\n got:  %s\n want predicate: %s", auditQuery.SQL, expectedAuditPredicate)
	}
	if len(auditQuery.Args) != 2 || auditQuery.Args[1] != "tenant-alpha" {
		t.Errorf("expected tenant arg 'tenant-alpha', got %v", auditQuery.Args)
	}

	// Verify GitOps PR query contains tenant filter
	gitQuery, ok := domains[SearchDomainGit]
	if !ok {
		t.Fatal("expected git query compiled")
	}
	if gitQuery.SQL == "" || len(gitQuery.Args) < 2 {
		t.Errorf("git query missing tenant args: %+v", gitQuery)
	}
}

func TestSearchQueryBuilder_SingleDomain(t *testing.T) {
	builder := NewSearchQueryBuilder()
	ctx := context.Background()

	queries, err := builder.BuildQueries(ctx, "auth-pod", "incident")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queries) != 1 {
		t.Fatalf("expected 1 query for domain incident, got %d", len(queries))
	}
	if queries[0].Domain != SearchDomainIncident {
		t.Errorf("expected domain incident, got %s", queries[0].Domain)
	}
}

func TestSearchQueryBuilder_PlatformAdmin(t *testing.T) {
	builder := NewSearchQueryBuilder()
	ctx := context.WithValue(context.Background(), tenancy.UserRoleKey, "platform_admin")
	ctx = context.WithValue(ctx, tenancy.TenantIDKey, "admin-tenant")

	queries, err := builder.BuildQueries(ctx, "billing-service", "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, q := range queries {
		if q.Domain == SearchDomainLog {
			if strings.Contains(q.SQL, "tenant_id =") {
				t.Errorf("platform_admin audit query should not filter by tenant_id: %s", q.SQL)
			}
			if len(q.Args) != 1 {
				t.Errorf("platform_admin audit query expected 1 arg, got %d", len(q.Args))
			}
		}
	}
}
