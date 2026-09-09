package postgres

import (
	"context"
	"fmt"
)

// AllowedSearchTypes is the strict allowlist of permitted searchType values.
// searchType is never interpolated into SQL, but validating it early provides
// defense-in-depth against regressions.
var AllowedSearchTypes = map[string]bool{
	"all":        true,
	"incident":   true,
	"kubernetes": true,
	"log":        true,
	"git":        true,
}

// SearchDomain represents a searchable entity domain in the system.
type SearchDomain string

const (
	SearchDomainIncident   SearchDomain = "incident"
	SearchDomainKubernetes SearchDomain = "kubernetes"
	SearchDomainLog        SearchDomain = "log"
	SearchDomainGit        SearchDomain = "git"
)

// CompiledSearchQuery holds a generated SQL statement and its parameter arguments.
type CompiledSearchQuery struct {
	Domain SearchDomain
	SQL    string
	Args   []interface{}
}

// SearchQueryBuilder compiles search queries with multi-tenancy rules and parameterized filters.
type SearchQueryBuilder struct{}

// NewSearchQueryBuilder creates a new search query builder.
func NewSearchQueryBuilder() *SearchQueryBuilder {
	return &SearchQueryBuilder{}
}

// ValidateSearchType verifies if the given searchType is within the allowlist.
func (b *SearchQueryBuilder) ValidateSearchType(searchType string) error {
	if !AllowedSearchTypes[searchType] {
		return fmt.Errorf("unsupported search type: %q", searchType)
	}
	return nil
}

// BuildIncidentQuery compiles full-text search SQL for incidents.
func (b *SearchQueryBuilder) BuildIncidentQuery(ctx context.Context, q string) CompiledSearchQuery {
	rawQuery := `
		SELECT 'incident' as type, 'Incident in Pod ' || pod_name as title, message as desc
		FROM incidents
		WHERE pod_name ILIKE '%' || $1 || '%' OR message ILIKE '%' || $1 || '%' OR cluster_name ILIKE '%' || $1 || '%'
		LIMIT 10
	`
	query, args := BuildTenantQuery(ctx, rawQuery, q)
	return CompiledSearchQuery{
		Domain: SearchDomainIncident,
		SQL:    query,
		Args:   args,
	}
}

// BuildClusterQuery compiles full-text search SQL for fleet clusters (Kubernetes).
func (b *SearchQueryBuilder) BuildClusterQuery(ctx context.Context, q string) CompiledSearchQuery {
	rawQuery := `
		SELECT 'kubernetes' as type, 'Cluster: ' || name as title, 'Group: ' || "group" || ' · Region: ' || region || ' · Provider: ' || provider as desc
		FROM fleet_clusters
		WHERE name ILIKE '%' || $1 || '%' OR region ILIKE '%' || $1 || '%' OR provider ILIKE '%' || $1 || '%'
		LIMIT 10
	`
	query, args := BuildTenantQuery(ctx, rawQuery, q)
	return CompiledSearchQuery{
		Domain: SearchDomainKubernetes,
		SQL:    query,
		Args:   args,
	}
}

// BuildAuditLogQuery compiles full-text search SQL for audit logs.
func (b *SearchQueryBuilder) BuildAuditLogQuery(ctx context.Context, q string) CompiledSearchQuery {
	rawQuery := `
		SELECT 'log' as type, 'Audit: ' || action || ' on ' || target_name as title, 'Actor: ' || actor || ' · Result: ' || result as desc
		FROM audit_logs
		WHERE action ILIKE '%' || $1 || '%' OR target_name ILIKE '%' || $1 || '%' OR actor ILIKE '%' || $1 || '%'
		LIMIT 10
	`
	return CompiledSearchQuery{
		Domain: SearchDomainLog,
		SQL:    rawQuery,
		Args:   []interface{}{q},
	}
}

// BuildGitOpsPRQuery compiles full-text search SQL for GitOps pull requests.
func (b *SearchQueryBuilder) BuildGitOpsPRQuery(ctx context.Context, q string) CompiledSearchQuery {
	rawQuery := `
		SELECT 'git' as type, 'PR: ' || title as title, 'Repo: ' || repo_url || ' · Branch: ' || branch || ' · Status: ' || status as desc
		FROM gitops_prs
		WHERE title ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%' OR repo_url ILIKE '%' || $1 || '%'
		LIMIT 10
	`
	query, args := BuildTenantQuery(ctx, rawQuery, q)
	return CompiledSearchQuery{
		Domain: SearchDomainGit,
		SQL:    query,
		Args:   args,
	}
}

// BuildQueries returns all compiled search queries required for the requested searchType.
func (b *SearchQueryBuilder) BuildQueries(ctx context.Context, q string, searchType string) ([]CompiledSearchQuery, error) {
	if err := b.ValidateSearchType(searchType); err != nil {
		return nil, err
	}

	var queries []CompiledSearchQuery

	if searchType == "all" || searchType == "incident" {
		queries = append(queries, b.BuildIncidentQuery(ctx, q))
	}
	if searchType == "all" || searchType == "kubernetes" {
		queries = append(queries, b.BuildClusterQuery(ctx, q))
	}
	if searchType == "all" || searchType == "log" {
		queries = append(queries, b.BuildAuditLogQuery(ctx, q))
	}
	if searchType == "all" || searchType == "git" {
		queries = append(queries, b.BuildGitOpsPRQuery(ctx, q))
	}

	return queries, nil
}
