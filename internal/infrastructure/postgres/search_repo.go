package postgres

import (
	"context"
	"fmt"


	"github.com/datdt/k8sselfhost/internal/domain/search"
)

type searchRepo struct {
	db DBTX
}

// NewSearchRepo creates a new Postgres-backed Search repository.
func NewSearchRepo(db DBTX) search.Repository {
	return &searchRepo{db: db}
}

func (r *searchRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

// allowedSearchTypes is the strict allowlist of permitted searchType values.
// searchType is never interpolated into SQL, but validating it early provides
// defense-in-depth against future regressions.
var allowedSearchTypes = map[string]bool{
	"all":        true,
	"incident":   true,
	"kubernetes": true,
	"log":        true,
	"git":        true,
}

func (r *searchRepo) Search(ctx context.Context, q string, searchType string) ([]search.Result, error) {
	if !allowedSearchTypes[searchType] {
		return nil, fmt.Errorf("unsupported search type: %q", searchType)
	}

	var results []search.Result

	// 1. Incidents
	if searchType == "all" || searchType == "incident" {
		err := func() error {
			query := `
				SELECT 'incident' as type, 'Incident in Pod ' || pod_name as title, message as desc
				FROM incidents
				WHERE pod_name ILIKE '%' || $1 || '%' OR message ILIKE '%' || $1 || '%' OR cluster_name ILIKE '%' || $1 || '%'
				LIMIT 10
			`
			query, args := BuildTenantQuery(ctx, query, q)
			rows, err := r.getDB(ctx).Query(ctx, query, args...)
			if err != nil {
				return fmt.Errorf("searching incidents: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var res search.Result
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
					return fmt.Errorf("scanning incident row: %w", err)
				}
				results = append(results, res)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating incident rows: %w", err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	// 2. Clusters (Kubernetes)
	if searchType == "all" || searchType == "kubernetes" {
		err := func() error {
			query := `
				SELECT 'kubernetes' as type, 'Cluster: ' || name as title, 'Group: ' || "group" || ' · Region: ' || region || ' · Provider: ' || provider as desc
				FROM fleet_clusters
				WHERE name ILIKE '%' || $1 || '%' OR region ILIKE '%' || $1 || '%' OR provider ILIKE '%' || $1 || '%'
				LIMIT 10
			`
			query, args := BuildTenantQuery(ctx, query, q)
			rows, err := r.getDB(ctx).Query(ctx, query, args...)
			if err != nil {
				return fmt.Errorf("searching clusters: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var res search.Result
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
					return fmt.Errorf("scanning cluster row: %w", err)
				}
				results = append(results, res)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating cluster rows: %w", err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	// 3. Audit Logs (Log)
	if searchType == "all" || searchType == "log" {
		err := func() error {
			query := `
				SELECT 'log' as type, 'Audit: ' || action || ' on ' || target_name as title, 'Actor: ' || actor || ' · Result: ' || result as desc
				FROM audit_logs
				WHERE action ILIKE '%' || $1 || '%' OR target_name ILIKE '%' || $1 || '%' OR actor ILIKE '%' || $1 || '%'
				LIMIT 10
			`
			rows, err := r.getDB(ctx).Query(ctx, query, q)
			if err != nil {
				return fmt.Errorf("searching audit logs: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var res search.Result
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
					return fmt.Errorf("scanning audit log row: %w", err)
				}
				results = append(results, res)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating audit log rows: %w", err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	// 4. GitOps Pull Requests (Git)
	if searchType == "all" || searchType == "git" {
		err := func() error {
			query := `
				SELECT 'git' as type, 'PR: ' || title as title, 'Repo: ' || repo_url || ' · Branch: ' || branch || ' · Status: ' || status as desc
				FROM gitops_prs
				WHERE title ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%' OR repo_url ILIKE '%' || $1 || '%'
				LIMIT 10
			`
			query, args := BuildTenantQuery(ctx, query, q)
			rows, err := r.getDB(ctx).Query(ctx, query, args...)
			if err != nil {
				return fmt.Errorf("searching gitops prs: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				var res search.Result
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
					return fmt.Errorf("scanning gitops pr row: %w", err)
				}
				results = append(results, res)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating gitops pr rows: %w", err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
