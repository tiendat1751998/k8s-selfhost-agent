package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/search"
)

type searchRepo struct {
	db *pgxpool.Pool
}

// NewSearchRepo creates a new Postgres-backed Search repository.
func NewSearchRepo(db *pgxpool.Pool) search.Repository {
	return &searchRepo{db: db}
}

func (r *searchRepo) Search(ctx context.Context, q string, searchType string) ([]search.Result, error) {
	var results []search.Result

	// 1. Incidents
	if searchType == "all" || searchType == "incident" {
		query := `
			SELECT 'incident' as type, 'Incident in Pod ' || pod_name as title, message as desc
			FROM incidents
			WHERE pod_name ILIKE '%' || $1 || '%' OR message ILIKE '%' || $1 || '%' OR cluster_name ILIKE '%' || $1 || '%'
			LIMIT 10
		`
		rows, err := r.db.Query(ctx, query, q)
		if err != nil {
			return nil, fmt.Errorf("searching incidents: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var res search.Result
			if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
				return nil, fmt.Errorf("scanning incident row: %w", err)
			}
			results = append(results, res)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating incident rows: %w", err)
		}
	}

	// 2. Clusters (Kubernetes)
	if searchType == "all" || searchType == "kubernetes" {
		query := `
			SELECT 'kubernetes' as type, 'Cluster: ' || name as title, 'Group: ' || "group" || ' · Region: ' || region || ' · Provider: ' || provider as desc
			FROM fleet_clusters
			WHERE name ILIKE '%' || $1 || '%' OR region ILIKE '%' || $1 || '%' OR provider ILIKE '%' || $1 || '%'
			LIMIT 10
		`
		rows, err := r.db.Query(ctx, query, q)
		if err != nil {
			return nil, fmt.Errorf("searching clusters: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var res search.Result
			if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
				return nil, fmt.Errorf("scanning cluster row: %w", err)
			}
			results = append(results, res)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating cluster rows: %w", err)
		}
	}

	// 3. Audit Logs (Log)
	if searchType == "all" || searchType == "log" {
		query := `
			SELECT 'log' as type, 'Audit: ' || action || ' on ' || target_name as title, 'Actor: ' || actor || ' · Result: ' || result as desc
			FROM audit_logs
			WHERE action ILIKE '%' || $1 || '%' OR target_name ILIKE '%' || $1 || '%' OR actor ILIKE '%' || $1 || '%'
			LIMIT 10
		`
		rows, err := r.db.Query(ctx, query, q)
		if err != nil {
			return nil, fmt.Errorf("searching audit logs: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var res search.Result
			if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
				return nil, fmt.Errorf("scanning audit log row: %w", err)
			}
			results = append(results, res)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating audit log rows: %w", err)
		}
	}

	// 4. GitOps Pull Requests (Git)
	if searchType == "all" || searchType == "git" {
		query := `
			SELECT 'git' as type, 'PR: ' || title as title, 'Repo: ' || repo_url || ' · Branch: ' || branch || ' · Status: ' || status as desc
			FROM gitops_prs
			WHERE title ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%' OR repo_url ILIKE '%' || $1 || '%'
			LIMIT 10
		`
		rows, err := r.db.Query(ctx, query, q)
		if err != nil {
			return nil, fmt.Errorf("searching gitops prs: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var res search.Result
			if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
				return nil, fmt.Errorf("scanning gitops pr row: %w", err)
			}
			results = append(results, res)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterating gitops pr rows: %w", err)
		}
	}

	return results, nil
}
