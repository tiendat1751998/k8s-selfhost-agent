// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// PRRepo implements gitops.Repository using PostgreSQL.
type PRRepo struct {
	pool DBTX
}

// NewPRRepo creates a new PostgreSQL-backed GitOps pull request repository.
func NewPRRepo(pool DBTX) *PRRepo {
	return &PRRepo{pool: pool}
}

func (r *PRRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new pull request record.
func (r *PRRepo) Create(ctx context.Context, pr *gitops.PullRequest) error {
	filesJSON, err := json.Marshal(pr.FilesChanged)
	if err != nil {
		return errors.Wrap(err, "marshaling files changed")
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "org-google"
	}

	query := `
		INSERT INTO gitops_prs (incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, tenant_id, created_at, updated_at, merged_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id`

	err = r.getDB(ctx).QueryRow(ctx, query,
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
		tenantID,
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

	query, args := BuildTenantQuery(ctx, query, id)

	var pr gitops.PullRequest
	var filesJSON []byte
	var provider string
	var status string

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
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

	query, args := BuildTenantQuery(ctx, query, incidentID)

	var pr gitops.PullRequest
	var filesJSON []byte
	var provider string
	var status string

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
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

	_, err = r.getDB(ctx).Exec(ctx, query,
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
		countQuery, countArgs := BuildTenantQuery(ctx, countQuery, string(*status))
		err = r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total)
	} else {
		countQuery = `SELECT COUNT(*) FROM gitops_prs`
		countQuery, countArgs := BuildTenantQuery(ctx, countQuery)
		err = r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total)
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
		query, args := BuildTenantQuery(ctx, query, string(*status), limit, offset)
		rows, err = r.getDB(ctx).Query(ctx, query, args...)
	} else {
		query := `
			SELECT id, incident_id, provider, repo_url, branch, base_branch, title, description, pr_url, pr_number, status, files_changed, created_at, updated_at, merged_at
			FROM gitops_prs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2`
		query, args := BuildTenantQuery(ctx, query, limit, offset)
		rows, err = r.getDB(ctx).Query(ctx, query, args...)
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
