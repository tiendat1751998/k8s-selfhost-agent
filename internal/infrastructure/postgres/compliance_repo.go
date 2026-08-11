package postgres

import (
	"context"
	"fmt"


	"github.com/datdt/k8sselfhost/internal/domain/compliance"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// ComplianceRepo implements compliance.Repository using PostgreSQL.
type ComplianceRepo struct {
	pool DBTX
}

// NewComplianceRepo creates a new PostgreSQL-backed compliance repository.
func NewComplianceRepo(pool DBTX) *ComplianceRepo {
	return &ComplianceRepo{pool: pool}
}

func (r *ComplianceRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

func (r *ComplianceRepo) ListFrameworks(ctx context.Context) ([]compliance.Framework, error) {
	query := `SELECT id, name, icon, total_checks, passed_checks, failed_checks, score, last_scan_at, created_at, updated_at FROM compliance_frameworks ORDER BY name`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "listing compliance frameworks")
	}
	defer rows.Close()

	var frameworks []compliance.Framework
	for rows.Next() {
		var f compliance.Framework
		if err := rows.Scan(&f.ID, &f.Name, &f.Icon, &f.TotalChecks, &f.PassedChecks, &f.FailedChecks, &f.Score, &f.LastScanAt, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, errors.Wrap(err, "scanning compliance framework")
		}
		frameworks = append(frameworks, f)
	}
	return frameworks, nil
}

func (r *ComplianceRepo) GetFramework(ctx context.Context, id string) (*compliance.Framework, error) {
	var f compliance.Framework
	query := `SELECT id, name, icon, total_checks, passed_checks, failed_checks, score, last_scan_at, created_at, updated_at FROM compliance_frameworks WHERE id = $1`
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(&f.ID, &f.Name, &f.Icon, &f.TotalChecks, &f.PassedChecks, &f.FailedChecks, &f.Score, &f.LastScanAt, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, errors.Wrap(err, "getting compliance framework")
	}
	return &f, nil
}

func (r *ComplianceRepo) UpsertFramework(ctx context.Context, f *compliance.Framework) error {
	query := `INSERT INTO compliance_frameworks (name, icon, total_checks, passed_checks, failed_checks, score, last_scan_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (name) DO UPDATE SET
			icon = EXCLUDED.icon, total_checks = EXCLUDED.total_checks, passed_checks = EXCLUDED.passed_checks,
			failed_checks = EXCLUDED.failed_checks, score = EXCLUDED.score, last_scan_at = EXCLUDED.last_scan_at, updated_at = NOW()
		RETURNING id`

	err := r.getDB(ctx).QueryRow(ctx, query,
		f.Name, f.Icon, f.TotalChecks, f.PassedChecks, f.FailedChecks, f.Score, f.LastScanAt,
	).Scan(&f.ID)
	if err != nil {
		return errors.Wrap(err, "upserting compliance framework")
	}
	return nil
}

func (r *ComplianceRepo) ListViolations(ctx context.Context, severity *compliance.ViolationSeverity, limit, offset int) ([]compliance.Violation, int, error) {
	query := `SELECT id, framework_id, severity, policy, resource, namespace, cluster, message, resolved, detected_at FROM compliance_violations WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM compliance_violations WHERE 1=1`
	args := []interface{}{}
	idx := 1

	if severity != nil {
		query += fmt.Sprintf(" AND severity = $%d", idx)
		countQuery += fmt.Sprintf(" AND severity = $%d", idx)
		args = append(args, string(*severity))
		idx++
	}

	var total int
	if err := r.getDB(ctx).QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "counting compliance violations")
	}

	selectQuery := query + fmt.Sprintf(" ORDER BY detected_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	selectArgs := append(args, limit, offset)

	rows, err := r.getDB(ctx).Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "listing compliance violations")
	}
	defer rows.Close()

	var violations []compliance.Violation
	for rows.Next() {
		var v compliance.Violation
		if err := rows.Scan(&v.ID, &v.FrameworkID, &v.Severity, &v.Policy, &v.Resource, &v.Namespace, &v.Cluster, &v.Message, &v.Resolved, &v.DetectedAt); err != nil {
			return nil, 0, errors.Wrap(err, "scanning compliance violation")
		}
		violations = append(violations, v)
	}
	return violations, total, nil
}

func (r *ComplianceRepo) CreateViolation(ctx context.Context, v *compliance.Violation) error {
	query := `INSERT INTO compliance_violations (framework_id, severity, policy, resource, namespace, cluster, message, resolved, detected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	err := r.getDB(ctx).QueryRow(ctx, query,
		v.FrameworkID, string(v.Severity), v.Policy, v.Resource, v.Namespace, v.Cluster, v.Message, v.Resolved, v.DetectedAt,
	).Scan(&v.ID)
	if err != nil {
		return errors.Wrap(err, "inserting compliance violation")
	}
	return nil
}

func (r *ComplianceRepo) ResolveViolation(ctx context.Context, id string) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE compliance_violations SET resolved = TRUE WHERE id = $1`, id)
	if err != nil {
		return errors.Wrap(err, "resolving compliance violation")
	}
	return nil
}

var _ compliance.Repository = (*ComplianceRepo)(nil)
