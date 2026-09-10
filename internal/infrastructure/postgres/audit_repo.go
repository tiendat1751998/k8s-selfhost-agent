package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// auditRepo implements audit.Repository using PostgreSQL.
type auditRepo struct {
	db DBTX
}

// NewAuditRepo creates a new Postgres-backed Audit repository.
func NewAuditRepo(db DBTX) audit.Repository {
	return &auditRepo{db: db}
}

func (r *auditRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *auditRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	var query string
	var args []interface{}

	if status != "" && status != "all" {
		query = `
			SELECT id, category, severity, description, remediation, status, detected_at, resolved_at 
			FROM audit_findings 
			WHERE status = $1 
			ORDER BY detected_at DESC
		`
		args = append(args, status)
	} else {
		query = `
			SELECT id, category, severity, description, remediation, status, detected_at, resolved_at 
			FROM audit_findings 
			ORDER BY detected_at DESC
		`
	}
	query, args = BuildTenantQuery(ctx, query, args...)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying audit findings: %w", err)
	}
	defer rows.Close()

	var findings []audit.AuditFinding
	for rows.Next() {
		var f audit.AuditFinding
		if err := rows.Scan(
			&f.ID, &f.Category, &f.Severity, &f.Description, &f.Remediation,
			&f.Status, &f.DetectedAt, &f.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning audit finding: %w", err)
		}
		findings = append(findings, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit findings: %w", err)
	}

	return findings, nil
}

func (r *auditRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	query := `
		SELECT id, category, severity, description, remediation, status, detected_at, resolved_at 
		FROM audit_findings 
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)
	var f audit.AuditFinding
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&f.ID, &f.Category, &f.Severity, &f.Description, &f.Remediation,
		&f.Status, &f.DetectedAt, &f.ResolvedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("audit finding not found")
		}
		return nil, fmt.Errorf("querying audit finding: %w", err)
	}
	return &f, nil
}

func (r *auditRepo) ResolveFinding(ctx context.Context, id string) error {
	query := `
		UPDATE audit_findings 
		SET status = 'resolved', resolved_at = NOW() 
		WHERE id = $1
	`
	query, args := BuildTenantQuery(ctx, query, id)
	cmd, err := r.getDB(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("updating audit finding: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("audit finding not found")
	}
	return nil
}

func (r *auditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	query := `
		INSERT INTO audit_runs (status, start_time, end_time, findings_count, tenant_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	err := r.getDB(ctx).QueryRow(ctx, query, run.Status, run.StartTime, run.EndTime, run.FindingsCount, tenantID).Scan(&run.ID)
	if err != nil {
		return fmt.Errorf("inserting audit run: %w", err)
	}
	return nil
}

func (r *auditRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	query := `
		SELECT id, status, start_time, end_time, findings_count 
		FROM audit_runs 
		ORDER BY start_time DESC 
		LIMIT 1
	`
	query, args := BuildTenantQuery(ctx, query)
	var run audit.AuditRun
	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&run.ID, &run.Status, &run.StartTime, &run.EndTime, &run.FindingsCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			// If no run exists, return a dummy or nil without error
			return nil, nil
		}
		return nil, fmt.Errorf("querying last audit run: %w", err)
	}
	return &run, nil
}

func (r *auditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	var targetUUID *string
	if targetID != "" {
		if _, err := uuid.Parse(targetID); err == nil {
			targetUUID = &targetID
		} else {
			targetUUID = nil
			if targetName == "" {
				targetName = targetID
			}
		}
	}
	
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	query := `
		INSERT INTO audit_logs (actor, action, target_type, target_id, target_name, result, details, ip_address, user_agent, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.getDB(ctx).Exec(ctx, query, actor, action, targetType, targetUUID, targetName, result, detailsJSON, ipAddress, userAgent, tenantID)
	if err != nil {
		return fmt.Errorf("recording audit log action: %w", err)
	}
	return nil
}

func (r *auditRepo) ListLogs(ctx context.Context, filter audit.AuditLogFilter) ([]audit.AuditLog, int, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	baseQuery := `
		WITH enriched AS (
			SELECT 
				id::text AS id,
				actor,
				action,
				target_type,
				target_id::text AS target_id,
				target_name,
				result,
				details,
				ip_address,
				user_agent,
				created_at,
				tenant_id,
				COALESCE(
					NULLIF(details->>'action_type', ''),
					CASE 
						WHEN action IN ('grant', 'revoke', 'rbac_grant') OR target_type IN ('k8s_rolebinding', 'role_binding', 'rbac') THEN 'rbac_grant'
						WHEN action IN ('delete', 'deletion', 'evict', 'destroy', 'purge', 'remove', 'drop') THEN 'deletion'
						WHEN action IN ('login', 'logout', 'access', 'read', 'view', 'auth') THEN 'access'
						ELSE 'mutation'
					END
				) AS action_type,
				COALESCE(
					NULLIF(details->>'severity', ''),
					CASE 
						WHEN result IN ('denied', 'forbidden', 'rejected', 'blocked') OR action IN ('evict', 'drain') THEN 'critical'
						WHEN result IN ('failure', 'failed', 'error') OR action IN ('delete', 'cordon', 'grant') THEN 'high'
						WHEN action IN ('scale', 'restart', 'apply', 'upgrade', 'install', 'update') THEN 'medium'
						WHEN action IN ('login', 'logout', 'read', 'view') THEN 'info'
						ELSE 'low'
					END
				) AS severity,
				CASE 
					WHEN result IN ('success', 'succeeded', 'ok') THEN 'success'
					WHEN result IN ('denied', 'forbidden', 'rejected', 'blocked', 'unauthorized') THEN 'denied'
					WHEN result IN ('failure', 'failed', 'error') THEN 'error'
					WHEN result = 'pending' THEN 'pending'
					ELSE result
				END AS status
			FROM audit_logs
		)
	`

	var args []interface{}
	argIdx := 1
	tenantID := tenancy.TenantIDFromContext(ctx)
	userRole := tenancy.UserRoleFromContext(ctx)
	whereClause := " WHERE 1=1"
	if userRole != "platform_admin" && tenantID != "" {
		whereClause += fmt.Sprintf(" AND tenant_id = $%d", argIdx)
		args = append(args, tenantID)
		argIdx++
	}

	if filter.Actor != "" {
		whereClause += fmt.Sprintf(" AND LOWER(actor) = LOWER($%d)", argIdx)
		args = append(args, filter.Actor)
		argIdx++
	}

	if filter.ActionType != "" {
		whereClause += fmt.Sprintf(" AND LOWER(action_type) = LOWER($%d)", argIdx)
		args = append(args, filter.ActionType)
		argIdx++
	}

	if filter.Severity != "" {
		whereClause += fmt.Sprintf(" AND LOWER(severity) = LOWER($%d)", argIdx)
		args = append(args, filter.Severity)
		argIdx++
	}

	if filter.Status != "" {
		whereClause += fmt.Sprintf(" AND LOWER(status) = LOWER($%d)", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	if filter.Search != "" {
		searchPattern := fmt.Sprintf("%%%s%%", filter.Search)
		whereClause += fmt.Sprintf(" AND (actor ILIKE $%d OR action ILIKE $%d OR target_name ILIKE $%d OR ip_address ILIKE $%d)", argIdx, argIdx, argIdx, argIdx)
		args = append(args, searchPattern)
		argIdx++
	}

	countQuery := fmt.Sprintf("%s SELECT COUNT(*) FROM enriched %s", baseQuery, whereClause)
	var total int
	if err := r.getDB(ctx).QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting audit logs: %w", err)
	}

	selectQuery := fmt.Sprintf(
		"%s SELECT id, actor, action, target_type, target_id, target_name, result, details, ip_address, user_agent, created_at, tenant_id, action_type, severity, status FROM enriched %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d",
		baseQuery, whereClause, argIdx, argIdx+1,
	)
	selectArgs := append(args, limit, offset)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying audit logs: %w", err)
	}
	defer rows.Close()

	logs := make([]audit.AuditLog, 0)
	for rows.Next() {
		var l audit.AuditLog
		var targetID *string
		var rawResult string
		var detailsJSON []byte
		var tenantID *string

		if err := rows.Scan(
			&l.ID,
			&l.Actor,
			&l.Action,
			&l.TargetType,
			&targetID,
			&l.TargetResource,
			&rawResult,
			&detailsJSON,
			&l.IPAddress,
			&l.UserAgent,
			&l.Timestamp,
			&tenantID,
			&l.ActionType,
			&l.Severity,
			&l.Status,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning audit log: %w", err)
		}

		if targetID != nil && *targetID != "" {
			l.TargetID = targetID
		}
		if l.TargetResource == "" && targetID != nil {
			l.TargetResource = *targetID
		}
		if tenantID != nil && *tenantID != "" {
			l.TenantID = tenantID
		}

		if len(detailsJSON) > 0 {
			if err := json.Unmarshal(detailsJSON, &l.Details); err != nil {
				l.Details = make(map[string]interface{})
			}
		}
		if l.Details == nil {
			l.Details = make(map[string]interface{})
		}
		if p, ok := l.Details["payload"].(map[string]interface{}); ok && p != nil {
			l.Payload = p
		} else {
			l.Payload = make(map[string]interface{})
		}

		logs = append(logs, l)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating audit logs: %w", err)
	}

	return logs, total, nil
}