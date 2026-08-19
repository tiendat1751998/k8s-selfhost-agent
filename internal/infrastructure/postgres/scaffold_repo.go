package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/scaffold"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type scaffoldRepo struct {
	pool DBTX
}

// NewScaffoldRepo creates a new PostgreSQL-backed repository for scaffold templates.
func NewScaffoldRepo(pool DBTX) scaffold.Repository {
	return &scaffoldRepo{pool: pool}
}

func (r *scaffoldRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new scaffold template.
func (r *scaffoldRepo) Create(ctx context.Context, t *scaffold.Template) error {
	if t == nil {
		return fmt.Errorf("template cannot be nil")
	}

	if t.ID == "" {
		t.ID = "tpl-" + uuid.NewString()[:8]
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole != "platform_admin" && tenantID != "" {
		t.TenantID = tenantID
	}
	if t.TenantID == "" {
		t.TenantID = "default-tenant"
	}

	if t.Category == "" {
		t.Category = scaffold.CategoryWeb
	}
	if t.Framework == "" {
		t.Framework = scaffold.FrameworkGoChi
	}
	if t.Variables == nil {
		t.Variables = []scaffold.TemplateVariable{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}

	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	varsJSON, err := json.Marshal(t.Variables)
	if err != nil {
		return fmt.Errorf("marshaling variables: %w", err)
	}

	tagsJSON, err := json.Marshal(t.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	query := `
		INSERT INTO scaffold_templates (
			id, name, description, category, framework,
			manifest_yaml, helm_values, docker_compose,
			variables, tags, built_in, tenant_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.getDB(ctx).Exec(ctx, query,
		t.ID,
		t.Name,
		t.Description,
		t.Category,
		t.Framework,
		t.ManifestYAML,
		t.HelmValues,
		t.DockerCompose,
		varsJSON,
		tagsJSON,
		t.BuiltIn,
		t.TenantID,
		t.CreatedAt,
		t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting scaffold template: %w", err)
	}

	return nil
}

// GetByID retrieves a template by ID, falling back to built-in templates.
func (r *scaffoldRepo) GetByID(ctx context.Context, id string) (*scaffold.Template, error) {
	// First check built-ins
	for _, builtin := range scaffold.GetBuiltinTemplates() {
		if builtin.ID == id {
			cp := builtin
			return &cp, nil
		}
	}

	query := `
		SELECT id, name, description, category, framework,
		       manifest_yaml, helm_values, docker_compose,
		       variables, tags, built_in, tenant_id, created_at, updated_at
		FROM scaffold_templates
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)

	var t scaffold.Template
	var varsRaw []byte
	var tagsRaw []byte

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&t.ID,
		&t.Name,
		&t.Description,
		&t.Category,
		&t.Framework,
		&t.ManifestYAML,
		&t.HelmValues,
		&t.DockerCompose,
		&varsRaw,
		&tagsRaw,
		&t.BuiltIn,
		&t.TenantID,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting scaffold template by id: %w", err)
	}

	t.Variables = []scaffold.TemplateVariable{}
	if len(varsRaw) > 0 {
		if err := json.Unmarshal(varsRaw, &t.Variables); err != nil {
			return nil, fmt.Errorf("unmarshaling variables: %w", err)
		}
	}

	t.Tags = []string{}
	if len(tagsRaw) > 0 {
		if err := json.Unmarshal(tagsRaw, &t.Tags); err != nil {
			return nil, fmt.Errorf("unmarshaling tags: %w", err)
		}
	}

	return &t, nil
}

// List returns custom templates and built-in templates matching filters.
func (r *scaffoldRepo) List(ctx context.Context, tenantID string, filter scaffold.ListFilter) ([]scaffold.Template, error) {
	baseQuery := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	userRole := tenancy.UserRoleFromContext(ctx)
	if userRole == "platform_admin" && tenantID != "" {
		baseQuery += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	if filter.Category != "" {
		baseQuery += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, filter.Category)
		argIdx++
	}

	if filter.Framework != "" {
		baseQuery += fmt.Sprintf(" AND framework = $%d", argIdx)
		args = append(args, filter.Framework)
		argIdx++
	}

	if filter.Search != "" {
		searchPattern := fmt.Sprintf("%%%s%%", filter.Search)
		baseQuery += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx)
		args = append(args, searchPattern)
		argIdx++
	}

	selectQuery := fmt.Sprintf(
		`SELECT id, name, description, category, framework,
		        manifest_yaml, helm_values, docker_compose,
		        variables, tags, built_in, tenant_id, created_at, updated_at
		 FROM scaffold_templates %s ORDER BY created_at DESC`,
		baseQuery,
	)
	selectQuery, args = BuildTenantQuery(ctx, selectQuery, args...)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("listing scaffold templates: %w", err)
	}
	defer rows.Close()

	customTemplates := make([]scaffold.Template, 0)
	dbTemplateIDs := make(map[string]bool)

	for rows.Next() {
		var t scaffold.Template
		var varsRaw []byte
		var tagsRaw []byte

		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Description,
			&t.Category,
			&t.Framework,
			&t.ManifestYAML,
			&t.HelmValues,
			&t.DockerCompose,
			&varsRaw,
			&tagsRaw,
			&t.BuiltIn,
			&t.TenantID,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning scaffold template: %w", err)
		}

		t.Variables = []scaffold.TemplateVariable{}
		if len(varsRaw) > 0 {
			if err := json.Unmarshal(varsRaw, &t.Variables); err != nil {
				return nil, fmt.Errorf("unmarshaling variables: %w", err)
			}
		}

		t.Tags = []string{}
		if len(tagsRaw) > 0 {
			if err := json.Unmarshal(tagsRaw, &t.Tags); err != nil {
				return nil, fmt.Errorf("unmarshaling tags: %w", err)
			}
		}

		dbTemplateIDs[t.ID] = true
		customTemplates = append(customTemplates, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating scaffold templates: %w", err)
	}

	// Filter and include built-in templates
	result := make([]scaffold.Template, 0)
	for _, builtin := range scaffold.GetBuiltinTemplates() {
		if dbTemplateIDs[builtin.ID] {
			continue
		}
		if filter.Category != "" && builtin.Category != filter.Category {
			continue
		}
		if filter.Framework != "" && builtin.Framework != filter.Framework {
			continue
		}
		if filter.Search != "" {
			term := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(builtin.Name), term) && !strings.Contains(strings.ToLower(builtin.Description), term) {
				continue
			}
		}
		result = append(result, builtin)
	}

	result = append(result, customTemplates...)
	return result, nil
}

// Update updates a user-defined template.
func (r *scaffoldRepo) Update(ctx context.Context, t *scaffold.Template) error {
	if t == nil {
		return fmt.Errorf("template cannot be nil")
	}
	if t.ID == "" {
		return fmt.Errorf("template ID cannot be empty")
	}

	// Prevent updating built-in templates
	for _, builtin := range scaffold.GetBuiltinTemplates() {
		if builtin.ID == t.ID {
			return fmt.Errorf("cannot modify system built-in template %q", t.ID)
		}
	}

	if t.Variables == nil {
		t.Variables = []scaffold.TemplateVariable{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	t.UpdatedAt = time.Now().UTC()

	varsJSON, err := json.Marshal(t.Variables)
	if err != nil {
		return fmt.Errorf("marshaling variables: %w", err)
	}

	tagsJSON, err := json.Marshal(t.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	query := `
		UPDATE scaffold_templates
		SET name = $2, description = $3, category = $4, framework = $5,
		    manifest_yaml = $6, helm_values = $7, docker_compose = $8,
		    variables = $9, tags = $10, updated_at = $11
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query,
		t.ID,
		t.Name,
		t.Description,
		t.Category,
		t.Framework,
		t.ManifestYAML,
		t.HelmValues,
		t.DockerCompose,
		varsJSON,
		tagsJSON,
		t.UpdatedAt,
	)

	tag, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating scaffold template: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scaffold template %s not found or access denied", t.ID)
	}

	return nil
}

// Delete removes a user-defined template.
func (r *scaffoldRepo) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("template ID cannot be empty")
	}

	// Prevent deleting built-in templates
	for _, builtin := range scaffold.GetBuiltinTemplates() {
		if builtin.ID == id {
			return fmt.Errorf("cannot delete system built-in template %q", id)
		}
	}

	query := `DELETE FROM scaffold_templates WHERE id = $1`
	query, args := BuildTenantQuery(ctx, query, id)

	tag, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("deleting scaffold template: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("scaffold template %s not found or access denied", id)
	}

	return nil
}
