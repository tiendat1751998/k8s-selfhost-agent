package postgres_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

// TestBuildTenantQuery_EdgeCases tests BuildTenantQuery under various SQL query structures.
func TestBuildTenantQuery_EdgeCases(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-test")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "user")

	tests := []struct {
		name         string
		inputQuery   string
		inputArgs    []interface{}
		wantQuery    string
		wantArgsLen  int
		wantLastArg  interface{}
	}{
		{
			name:        "Simple SELECT without WHERE",
			inputQuery:  "SELECT * FROM incidents",
			inputArgs:   []interface{}{},
			wantQuery:   "SELECT * FROM incidents WHERE tenant_id = $1",
			wantArgsLen: 1,
			wantLastArg: "org-test",
		},
		{
			name:        "SELECT with existing WHERE",
			inputQuery:  "SELECT * FROM incidents WHERE status = $1",
			inputArgs:   []interface{}{"OPEN"},
			wantQuery:   "SELECT * FROM incidents WHERE status = $1 AND tenant_id = $2",
			wantArgsLen: 2,
			wantLastArg: "org-test",
		},
		{
			name:        "SELECT with ORDER BY and no WHERE",
			inputQuery:  "SELECT * FROM incidents ORDER BY created_at DESC",
			inputArgs:   []interface{}{},
			wantQuery:   "SELECT * FROM incidents WHERE tenant_id = $1 ORDER BY created_at DESC",
			wantArgsLen: 1,
			wantLastArg: "org-test",
		},
		{
			name:        "SELECT with WHERE and ORDER BY LIMIT",
			inputQuery:  "SELECT * FROM incidents WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
			inputArgs:   []interface{}{"OPEN", 10, 0},
			wantQuery:   "SELECT * FROM incidents WHERE status = $1 AND tenant_id = $4 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
			wantArgsLen: 4,
			wantLastArg: "org-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotArgs := postgres.BuildTenantQuery(ctx, tt.inputQuery, tt.inputArgs...)
			if gotQuery != tt.wantQuery {
				t.Errorf("Query mismatch:\n got:  %s\n want: %s", gotQuery, tt.wantQuery)
			}
			if len(gotArgs) != tt.wantArgsLen {
				t.Errorf("Args length mismatch: got %d, want %d", len(gotArgs), tt.wantArgsLen)
			}
			if len(gotArgs) > 0 && gotArgs[len(gotArgs)-1] != tt.wantLastArg {
				t.Errorf("Last arg mismatch: got %v, want %v", gotArgs[len(gotArgs)-1], tt.wantLastArg)
			}
		})
	}
}

// TestBuildTenantQuery_PlatformAdminBypass tests that platform_admin skips tenant filtering.
func TestBuildTenantQuery_PlatformAdminBypass(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.TenantIDKey, "org-test")
	ctx = context.WithValue(ctx, middleware.UserRoleKey, "platform_admin")
	query := "SELECT * FROM incidents"
	args := []interface{}{"param1"}

	gotQuery, gotArgs := postgres.BuildTenantQuery(ctx, query, args...)
	if gotQuery != query {
		t.Errorf("platform_admin modified query: got %s, want %s", gotQuery, query)
	}
	if len(gotArgs) != 1 {
		t.Errorf("platform_admin modified args length: got %d, want 1", len(gotArgs))
	}
}

// TestAllPostgresRepos_QueryCallsUseBuildTenantQuery checks every Query and QueryRow call in postgres package.
func TestAllPostgresRepos_QueryCallsUseBuildTenantQuery(t *testing.T) {
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	knownExceptions := map[string][]string{
		"agent_repo.go":         {"*"},                             // Non-tenant tables (agent_tasks, agent_subtasks, agent_executions, agent_project_state)
		"audit_repo.go":         {"*"},                             // Non-tenant tables (audit_findings, audit_runs, audit_logs)
		"automation_repo.go":    {"*"},                             // Non-tenant tables (automation_rules, automation_executions)
		"backup_repo.go":        {"*"},                             // Non-tenant tables (backup_history)
		"changes_repo.go":       {"*"},                             // Non-tenant tables (change_requests, maintenance_windows)
		"capacity_repo.go":      {"*"},                             // Non-tenant tables (capacity_forecasts)
		"compliance_repo.go":    {"*"},                             // Non-tenant tables (compliance_frameworks, compliance_violations)
		"correlation_repo.go":   {"*"},                             // Non-tenant tables (correlated_events)
		"cost_repo.go":          {"*"},                             // Non-tenant tables (cluster_costs, namespace_costs, resource_waste)
		"drift_repo.go":         {"*"},                             // Non-tenant tables (drift_records)
		"notification_repo.go":  {"ListNotifications", "MarkRead", "MarkAllRead", "CreateNotification"}, // Non-tenant notifications table; notification_channels is now tenant-isolated
		"observability_repo.go": {"*"},                             // Non-tenant tables (slo_definitions, slo_snapshots)
		"promotion_repo.go":     {"*"},                             // Non-tenant tables (promotions)
		"reporting_repo.go":     {"*"},                             // Non-tenant tables (reports)
		"search_repo.go":        {"Search"},                        // audit_logs step in Search has no tenant_id
		"tagging_repo.go":       {"*"},                             // Non-tenant tables (tags, resource_tags)
		"tenancy_repo.go":       {"*"},                             // Non-tenant tables (organizations, projects, tenant_members, rbac_matrix)
		"timeline_repo.go":      {"*"},                             // Non-tenant tables (timeline_events)
		"user_repo.go":          {"GetByEmail", "GetByID"},          // Authentication and re-authentication prior to/during user context lookup
	}

	var violations []string

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		if file.Name() == "client.go" || file.Name() == "tenant_query.go" {
			continue
		}

		filename := file.Name()
		fset := token.NewFileSet()
		node, parseErr := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if parseErr != nil {
			t.Errorf("Error parsing %s: %v", filename, parseErr)
			continue
		}

		ast.Inspect(node, func(n ast.Node) bool {
			fnDecl, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}

			funcName := fnDecl.Name.Name
			if exceptions, found := knownExceptions[filename]; found {
				for _, exc := range exceptions {
					if exc == "*" || funcName == exc {
						return false // skip checking this function
					}
				}
			}

			// Inspect every Query or QueryRow call
			ast.Inspect(fnDecl.Body, func(bn ast.Node) bool {
				call, isCall := bn.(*ast.CallExpr)
				if !isCall {
					return true
				}

				sel, isSel := call.Fun.(*ast.SelectorExpr)
				if !isSel {
					return true
				}

				if sel.Sel.Name == "Query" || sel.Sel.Name == "QueryRow" {
					if len(call.Args) >= 2 {
						// Check second argument (query string / var)
						queryArg := call.Args[1]
						if lit, isLit := queryArg.(*ast.BasicLit); isLit && lit.Kind == token.STRING {
							if strings.Contains(strings.ToUpper(lit.Value), "SELECT") {
								violations = append(violations, filename+" -> "+funcName+": direct raw SELECT string in Query/QueryRow: "+lit.Value)
							}
						}
					}
				}
				return true
			})

			return true
		})
	}

	for _, v := range violations {
		t.Errorf("Violation: %s", v)
	}
}
