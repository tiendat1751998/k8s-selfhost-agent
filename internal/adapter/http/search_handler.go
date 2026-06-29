package http

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SearchHandler handles global search HTTP requests.
type SearchHandler struct {
	pool *pgxpool.Pool
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(pool *pgxpool.Pool) *SearchHandler {
	return &SearchHandler{pool: pool}
}

type searchResult struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

// Search handles GET /api/v1/search
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	searchType := r.URL.Query().Get("type")

	if q == "" {
		writeJSON(w, http.StatusOK, []searchResult{})
		return
	}

	results := []searchResult{}

	// 1. Incidents
	if searchType == "all" || searchType == "incident" {
		query := `
			SELECT 'incident' as type, 'Incident in Pod ' || pod_name as title, message as desc
			FROM incidents
			WHERE pod_name ILIKE '%' || $1 || '%' OR message ILIKE '%' || $1 || '%' OR cluster_name ILIKE '%' || $1 || '%'
			LIMIT 10
		`
		rows, err := h.pool.Query(r.Context(), query, q)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var res searchResult
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err == nil {
					results = append(results, res)
				}
			}
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
		rows, err := h.pool.Query(r.Context(), query, q)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var res searchResult
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err == nil {
					results = append(results, res)
				}
			}
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
		rows, err := h.pool.Query(r.Context(), query, q)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var res searchResult
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err == nil {
					results = append(results, res)
				}
			}
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
		rows, err := h.pool.Query(r.Context(), query, q)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var res searchResult
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err == nil {
					results = append(results, res)
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, results)
}
