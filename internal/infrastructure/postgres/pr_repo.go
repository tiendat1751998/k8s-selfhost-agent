// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/pkg/errors"
)

// PRRepo implements gitops.Repository using PostgreSQL.
type PRRepo struct {
	pool *pgxpool.Pool
}

// NewPRRepo creates a new PostgreSQL-backed GitOps pull request repository.
func NewPRRepo(pool *pgxpool.Pool) *PRRepo {
	return &PRRepo{pool: pool}
}

// Create persists a new pull request record.
func (r *PRRepo) Create(ctx context.Context, pr *gitops.PullRequest) error {
	filesJSON, err := json.Marshal(pr.FilesChanged)
	if err != nil {
		return errors.Wrap(err, "marshaling files changed")
	}

	query := `
		INSERT INTO gitops_prs (incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, created_at, updated_at, merged_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id`

	err = r.pool.QueryRow(ctx, query,
		pr.IncidentID,
		string(pr.Provider),
		pr.RepoURL,
		pr.Branch,
		pr.BaseBranch,
		pr.Title,
		pr.Description,
		pr.PRURL,
		pr.PRNumber,
		string(pr.Status),
		filesJSON,
		pr.CreatedAt,
		pr.UpdatedAt,
		pr.MergedAt,
	).Scan(&pr.ID)

	if err != nil {
		return errors.Wrap(err, "inserting pull request")
	}

	return nil
}

// GetByID retrieves a pull request by ID.
func (r *PRRepo) GetByID(ctx context.Context, id string) (*gitops.PullRequest, error) {
	query := `
		SELECT id, incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, created_at, updated_at, merged_at
		FROM gitops_prs
		WHERE id = $1`

	var pr gitops.PullRequest
	var filesJSON []byte
	var provider string
	var status string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&pr.ID,
		&pr.IncidentID,
		&provider,
		&pr.RepoURL,
		&pr.Branch,
		&pr.BaseBranch,
		&pr.Title,
		&pr.Description,
		&pr.PRURL,
		&pr.PRNumber,
		&status,
		&filesJSON,
		&pr.CreatedAt,
		&pr.UpdatedAt,
		&pr.MergedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, errors.Wrap(err, "getting pull request by id")
	}

	if err := json.Unmarshal(filesJSON, &pr.FilesChanged); err != nil {
		return nil, errors.Wrap(err, "unmarshaling files changed")
	}
	pr.Provider = gitops.Provider(provider)
	pr.Status = gitops.PRStatus(status)

	return &pr, nil
}

// GetByIncidentID retrieves a pull request by incident ID.
func (r *PRRepo) GetByIncidentID(ctx context.Context, incidentID string) (*gitops.PullRequest, error) {
	query := `
		SELECT id, incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, created_at, updated_at, merged_at
		FROM gitops_prs
		WHERE incident_id = $1`

	var pr gitops.PullRequest
	var filesJSON []byte
	var provider string
	var status string

	err := r.pool.QueryRow(ctx, query, incidentID).Scan(
		&pr.ID,
		&pr.IncidentID,
		&provider,
		&pr.RepoURL,
		&pr.Branch,
		&pr.BaseBranch,
		&pr.Title,
		&pr.Description,
		&pr.PRURL,
		&pr.PRNumber,
		&status,
		&filesJSON,
		&pr.CreatedAt,
		&pr.UpdatedAt,
		&pr.MergedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, errors.Wrap(err, "getting pull request by incident id")
	}

	if err := json.Unmarshal(filesJSON, &pr.FilesChanged); err != nil {
		return nil, errors.Wrap(err, "unmarshaling files changed")
	}
	pr.Provider = gitops.Provider(provider)
	pr.Status = gitops.PRStatus(status)

	return &pr, nil
}

// Update saves changes to an existing pull request record.
func (r *PRRepo) Update(ctx context.Context, pr *gitops.PullRequest) error {
	filesJSON, err := json.Marshal(pr.FilesChanged)
	if err != nil {
		return errors.Wrap(err, "marshaling files changed")
	}

	query := `
		UPDATE gitops_prs
		SET status = $1, pr_url = $2, pr_number = $3, files_changed = $4, updated_at = $5, merged_at = $6
		WHERE id = $7`

	_, err = r.pool.Exec(ctx, query,
		string(pr.Status),
		pr.PRURL,
		pr.PRNumber,
		filesJSON,
		pr.UpdatedAt,
		pr.MergedAt,
		pr.ID,
	)

	if err != nil {
		return errors.Wrap(err, "updating pull request")
	}

	return nil
}

// List retrieves pull requests with pagination.
func (r *PRRepo) List(ctx context.Context, status *gitops.PRStatus, limit, offset int) ([]*gitops.PullRequest, int64, error) {
	var total int64
	var countQuery string
	var err error

	if status != nil {
		countQuery = `SELECT COUNT(*) FROM gitops_prs WHERE status = $1`
		err = r.pool.QueryRow(ctx, countQuery, string(*status)).Scan(&total)
	} else {
		countQuery = `SELECT COUNT(*) FROM gitops_prs`
		err = r.pool.QueryRow(ctx, countQuery).Scan(&total)
	}

	if err != nil {
		return nil, 0, errors.Wrap(err, "counting pull requests")
	}

	var rows pgx.Rows
	if status != nil {
		query := `
			SELECT id, incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, created_at, updated_at, merged_at
			FROM gitops_prs
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3`
		rows, err = r.pool.Query(ctx, query, string(*status), limit, offset)
	} else {
		query := `
			SELECT id, incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, created_at, updated_at, merged_at
			FROM gitops_prs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2`
		rows, err = r.pool.Query(ctx, query, limit, offset)
	}

	if err != nil {
		return nil, 0, errors.Wrap(err, "listing pull requests")
	}
	defer rows.Close()

	var prs []*gitops.PullRequest
	for rows.Next() {
		var pr gitops.PullRequest
		var filesJSON []byte
		var provider string
		var prStatus string

		err := rows.Scan(
			&pr.ID,
			&pr.IncidentID,
			&provider,
			&pr.RepoURL,
			&pr.Branch,
			&pr.BaseBranch,
			&pr.Title,
			&pr.Description,
			&pr.PRURL,
			&pr.PRNumber,
			&prStatus,
			&filesJSON,
			&pr.CreatedAt,
			&pr.UpdatedAt,
			&pr.MergedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "scanning pull request")
		}

		if err := json.Unmarshal(filesJSON, &pr.FilesChanged); err != nil {
			return nil, 0, errors.Wrap(err, "unmarshaling files changed")
		}
		pr.Provider = gitops.Provider(provider)
		pr.Status = gitops.PRStatus(prStatus)

		prs = append(prs, &pr)
	}

	return prs, total, nil
}
