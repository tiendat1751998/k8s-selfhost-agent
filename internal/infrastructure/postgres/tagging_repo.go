package postgres

import (
	"context"

	"github.com/datdt/k8sselfhost/internal/domain/tagging"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// TaggingRepo implements tagging.Repository using PostgreSQL.
type TaggingRepo struct {
	pool DBTX
}

// NewTaggingRepo creates a new PostgreSQL-backed tagging repository.
func NewTaggingRepo(pool DBTX) *TaggingRepo {
	return &TaggingRepo{pool: pool}
}

func (r *TaggingRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *TaggingRepo) ListTags(ctx context.Context) ([]tagging.Tag, error) {
	ctx = EnsureTenantContext(ctx)
	query := `SELECT id, key, value, category, created_at FROM tags ORDER BY key, value`
	query, args := BuildTenantQuery(ctx, query)
	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "listing tags")
	}
	defer rows.Close()

	var tags []tagging.Tag
	for rows.Next() {
		var t tagging.Tag
		if err := rows.Scan(&t.ID, &t.Key, &t.Value, &t.Category, &t.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "scanning tag")
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (r *TaggingRepo) CreateTag(ctx context.Context, tag *tagging.Tag) error {
	ctx = EnsureTenantContext(ctx)
	tenantID := TenantIDOrDefault(ctx)

	err := r.getDB(ctx).QueryRow(ctx,
		`INSERT INTO tags (key, value, category, tenant_id) VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		tag.Key, tag.Value, tag.Category, tenantID,
	).Scan(&tag.ID, &tag.CreatedAt)
	if err != nil {
		return errors.Wrap(err, "creating tag")
	}
	return nil
}

func (r *TaggingRepo) DeleteTag(ctx context.Context, id string) error {
	ctx = EnsureTenantContext(ctx)
	query, args := BuildTenantQuery(ctx, `DELETE FROM tags WHERE id = $1`, id)
	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "deleting tag")
	}
	return nil
}

func (r *TaggingRepo) TagResource(ctx context.Context, resourceID, tagID string) error {
	ctx = EnsureTenantContext(ctx)
	tenantID := TenantIDOrDefault(ctx)

	_, err := r.getDB(ctx).Exec(ctx,
		`INSERT INTO resource_tags (resource_id, tag_id, tenant_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		resourceID, tagID, tenantID,
	)
	if err != nil {
		return errors.Wrap(err, "tagging resource")
	}
	return nil
}

func (r *TaggingRepo) UntagResource(ctx context.Context, resourceID, tagID string) error {
	ctx = EnsureTenantContext(ctx)
	query, args := BuildTenantQuery(ctx, `DELETE FROM resource_tags WHERE resource_id = $1 AND tag_id = $2`, resourceID, tagID)
	_, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "untagging resource")
	}
	return nil
}

func (r *TaggingRepo) GetResourceTags(ctx context.Context, resourceID string) ([]tagging.Tag, error) {
	ctx = EnsureTenantContext(ctx)
	query := `SELECT t.id, t.key, t.value, t.category, t.created_at
		 FROM tags t
		 JOIN resource_tags rt ON rt.tag_id = t.id
		 WHERE rt.resource_id = $1
		 ORDER BY t.key, t.value`
	query, args := BuildTenantQuery(ctx, query, resourceID)
	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "getting resource tags")
	}
	defer rows.Close()

	var tags []tagging.Tag
	for rows.Next() {
		var t tagging.Tag
		if err := rows.Scan(&t.ID, &t.Key, &t.Value, &t.Category, &t.CreatedAt); err != nil {
			return nil, errors.Wrap(err, "scanning resource tag")
		}
		tags = append(tags, t)
	}
	return tags, nil
}

var _ tagging.Repository = (*TaggingRepo)(nil)
