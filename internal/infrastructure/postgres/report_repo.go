// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// ReportRepo implements report.Repository using PostgreSQL.
type ReportRepo struct {
	pool DBTX
}

// NewReportRepo creates a new PostgreSQL-backed report repository.
func NewReportRepo(pool DBTX) *ReportRepo {
	return &ReportRepo{pool: pool}
}

func (r *ReportRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.pool)
}

// Create persists a new report.
func (r *ReportRepo) Create(ctx context.Context, rpt *report.Report) error {
	evidenceJSON, err := json.Marshal(rpt.Evidence)
	if err != nil {
		return errors.Wrap(err, "marshaling evidence")
	}

	tenantID := tenancy.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	query := `
		INSERT INTO rca_reports (incident_id, root_cause, evidence, confidence, risk_level, remediation, rollback_plan, llm_model, prompt_tokens, response_tokens, tenant_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id`

	err = r.getDB(ctx).QueryRow(ctx, query,
		rpt.IncidentID,
		rpt.RootCause,
		evidenceJSON,
		rpt.Confidence,
		string(rpt.RiskLevel),
		rpt.Remediation,
		rpt.RollbackPlan,
		rpt.LLMModel,
		rpt.PromptTokens,
		rpt.ResponseTokens,
		tenantID,
		rpt.CreatedAt,
	).Scan(&rpt.ID)

	if err != nil {
		return errors.Wrap(err, "inserting report")
	}

	return nil
}

// GetByID retrieves a report by ID.
func (r *ReportRepo) GetByID(ctx context.Context, id string) (*report.Report, error) {
	query := `
		SELECT id, incident_id, root_cause, evidence, confidence, risk_level, remediation, rollback_plan, llm_model, prompt_tokens, response_tokens, created_at
		FROM rca_reports
		WHERE id = $1`

	query, args := BuildTenantQuery(ctx, query, id)

	var rpt report.Report
	var evidenceJSON []byte
	var riskLevel string

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&rpt.ID,
		&rpt.IncidentID,
		&rpt.RootCause,
		&evidenceJSON,
		&rpt.Confidence,
		&riskLevel,
		&rpt.Remediation,
		&rpt.RollbackPlan,
		&rpt.LLMModel,
		&rpt.PromptTokens,
		&rpt.ResponseTokens,
		&rpt.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, errors.Wrap(err, "getting report by id")
	}

	if err := json.Unmarshal(evidenceJSON, &rpt.Evidence); err != nil {
		return nil, errors.Wrap(err, "unmarshaling evidence")
	}
	rpt.RiskLevel = report.RiskLevel(riskLevel)

	return &rpt, nil
}

// GetByIncidentID retrieves a report by incident ID.
func (r *ReportRepo) GetByIncidentID(ctx context.Context, incidentID string) (*report.Report, error) {
	query := `
		SELECT id, incident_id, root_cause, evidence, confidence, risk_level, remediation, rollback_plan, llm_model, prompt_tokens, response_tokens, created_at
		FROM rca_reports
		WHERE incident_id = $1`

	query, args := BuildTenantQuery(ctx, query, incidentID)

	var rpt report.Report
	var evidenceJSON []byte
	var riskLevel string

	err := r.getDB(ctx).QueryRow(ctx, query, args...).Scan(
		&rpt.ID,
		&rpt.IncidentID,
		&rpt.RootCause,
		&evidenceJSON,
		&rpt.Confidence,
		&riskLevel,
		&rpt.Remediation,
		&rpt.RollbackPlan,
		&rpt.LLMModel,
		&rpt.PromptTokens,
		&rpt.ResponseTokens,
		&rpt.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.ErrNotFound
		}
		return nil, errors.Wrap(err, "getting report by incident id")
	}

	if err := json.Unmarshal(evidenceJSON, &rpt.Evidence); err != nil {
		return nil, errors.Wrap(err, "unmarshaling evidence")
	}
	rpt.RiskLevel = report.RiskLevel(riskLevel)

	return &rpt, nil
}

// List retrieves reports with pagination.
func (r *ReportRepo) List(ctx context.Context, limit, offset int) ([]*report.Report, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM rca_reports`
	countQuery, countArgs := BuildTenantQuery(ctx, countQuery)
	if err := r.getDB(ctx).QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, "counting reports")
	}

	query := `
		SELECT id, incident_id, root_cause, evidence, confidence, risk_level, remediation, rollback_plan, llm_model, prompt_tokens, response_tokens, created_at
		FROM rca_reports
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`
	query, args := BuildTenantQuery(ctx, query, limit, offset)

	rows, err := r.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "listing reports")
	}
	defer rows.Close()

	var reports []*report.Report
	for rows.Next() {
		var rpt report.Report
		var evidenceJSON []byte
		var riskLevel string

		err := rows.Scan(
			&rpt.ID,
			&rpt.IncidentID,
			&rpt.RootCause,
			&evidenceJSON,
			&rpt.Confidence,
			&riskLevel,
			&rpt.Remediation,
			&rpt.RollbackPlan,
			&rpt.LLMModel,
			&rpt.PromptTokens,
			&rpt.ResponseTokens,
			&rpt.CreatedAt,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "scanning report")
		}

		if err := json.Unmarshal(evidenceJSON, &rpt.Evidence); err != nil {
			return nil, 0, errors.Wrap(err, "unmarshaling evidence")
		}
		rpt.RiskLevel = report.RiskLevel(riskLevel)

		reports = append(reports, &rpt)
	}

	return reports, total, nil
}
