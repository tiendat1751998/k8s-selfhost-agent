package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
)

// auditRepo implements audit.Repository using PostgreSQL.
type auditRepo struct {
	db *pgxpool.Pool
}

// NewAuditRepo creates a new Postgres-backed Audit repository.
func NewAuditRepo(db *pgxpool.Pool) audit.Repository {
	return &auditRepo{db: db}
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

	rows, err := r.db.Query(ctx, query, args...)
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
	var f audit.AuditFinding
	err := r.db.QueryRow(ctx, query, id).Scan(
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
	cmd, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("updating audit finding: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("audit finding not found")
	}
	return nil
}

func (r *auditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	query := `
		INSERT INTO audit_runs (status, start_time, end_time, findings_count)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	err := r.db.QueryRow(ctx, query, run.Status, run.StartTime, run.EndTime, run.FindingsCount).Scan(&run.ID)
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
	var run audit.AuditRun
	err := r.db.QueryRow(ctx, query).Scan(
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
		targetUUID = &targetID
	}
	
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (actor, action, target_type, target_id, target_name, result, details, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.db.Exec(ctx, query, actor, action, targetType, targetUUID, targetName, result, detailsJSON, ipAddress, userAgent)
	if err != nil {
		return fmt.Errorf("recording audit log action: %w", err)
	}
	return nil
}
