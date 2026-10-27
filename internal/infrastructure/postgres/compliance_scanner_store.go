package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

// complianceScannerStore implements usecaseCompliance.Store using PostgreSQL.
type complianceScannerStore struct {
	db DBTX
}

// NewComplianceScannerStore creates a new PostgreSQL store for the compliance scanner.
func NewComplianceScannerStore(db DBTX) usecaseCompliance.Store {
	return &complianceScannerStore{db: db}
}

func (s *complianceScannerStore) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, s.db)
}

func (s *complianceScannerStore) ListHosts(ctx context.Context) ([]usecaseCompliance.ComputeHostRow, error) {
	query := `SELECT id, name, endpoint, status, labels FROM compute_hosts ORDER BY name ASC`
	query, args := BuildTenantQuery(ctx, query)
	rows, err := s.getDB(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing compute hosts: %w", err)
	}
	defer rows.Close()

	var hosts []usecaseCompliance.ComputeHostRow
	for rows.Next() {
		var h usecaseCompliance.ComputeHostRow
		var labelBytes []byte
		if err := rows.Scan(&h.ID, &h.Name, &h.Endpoint, &h.Status, &labelBytes); err != nil {
			return nil, fmt.Errorf("scanning compute host: %w", err)
		}
		if len(labelBytes) > 0 {
			var m map[string]string
			if err := json.Unmarshal(labelBytes, &m); err == nil {
				h.Labels = m
			}
		}
		if h.Labels == nil {
			h.Labels = make(map[string]string)
		}

		// Resolve Address
		if addr, ok := h.Labels["address"]; ok && addr != "" {
			h.Address = addr
		} else if h.Endpoint != "" {
			u, err := url.Parse(h.Endpoint)
			if err == nil && u.Hostname() != "" {
				h.Address = u.Hostname()
			} else {
				parts := strings.Split(strings.TrimPrefix(h.Endpoint, "http://"), ":")
				if len(parts) > 0 && parts[0] != "" {
					h.Address = parts[0]
				}
			}
		}
		if h.Address == "" {
			h.Address = "127.0.0.1"
		}

		hosts = append(hosts, h)
	}

	return hosts, nil
}

func (s *complianceScannerStore) CountActiveAutomationRules(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM automation_rules WHERE enabled = true`
	query, args := BuildTenantQuery(ctx, query)
	var count int
	if err := s.getDB(ctx).QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting active automation rules: %w", err)
	}
	return count, nil
}

func (s *complianceScannerStore) CountAuditLogs(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM audit_logs`
	query, args := BuildTenantQuery(ctx, query)
	var count int
	if err := s.getDB(ctx).QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting audit logs: %w", err)
	}
	return count, nil
}

func (s *complianceScannerStore) UpsertFramework(ctx context.Context, fw *usecaseCompliance.FrameworkRecord) error {
	tenantID := TenantIDOrDefault(ctx)
	query := `
		INSERT INTO compliance_frameworks (id, name, icon, total_checks, passed_checks, failed_checks, score, last_scan_at, tenant_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (name) DO UPDATE SET
			icon = EXCLUDED.icon,
			total_checks = EXCLUDED.total_checks,
			passed_checks = EXCLUDED.passed_checks,
			failed_checks = EXCLUDED.failed_checks,
			score = EXCLUDED.score,
			last_scan_at = EXCLUDED.last_scan_at,
			tenant_id = EXCLUDED.tenant_id,
			updated_at = NOW()
	`
	_, err := s.getDB(ctx).Exec(ctx, query,
		fw.ID, fw.Name, fw.Icon, fw.TotalChecks, fw.PassedChecks, fw.FailedChecks, fw.Score, fw.LastScanAt, tenantID,
	)
	if err != nil {
		return fmt.Errorf("upserting compliance framework %s: %w", fw.Name, err)
	}
	return nil
}

func (s *complianceScannerStore) ReplaceViolations(ctx context.Context, violations []usecaseCompliance.ViolationRecord) error {
	tenantID := TenantIDOrDefault(ctx)

	// Clean out stale seed violations from migration 062 and older scan detections scoped by tenant
	delQuery := `
		DELETE FROM compliance_violations 
		WHERE (id::text LIKE 'd1000001%' 
		   OR message LIKE '%AlwaysPullImages%' 
		   OR message LIKE '%default-deny%' 
		   OR message LIKE '%NetworkPolicy%' 
		   OR message LIKE '%HostPID%'
		   OR detected_at < NOW() - INTERVAL '1 hour')
	`
	delQuery, delArgs := BuildTenantQuery(ctx, delQuery)
	if _, err := s.getDB(ctx).Exec(ctx, delQuery, delArgs...); err != nil {
		return fmt.Errorf("deleting stale violations: %w", err)
	}

	insertQuery := `
		INSERT INTO compliance_violations (id, framework_id, severity, policy, resource, namespace, cluster, message, resolved, detected_at, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	for _, v := range violations {
		violID := v.ID
		if violID == "" {
			violID = uuid.NewString()
		}
		_, err := s.getDB(ctx).Exec(ctx, insertQuery,
			violID, v.FrameworkID, v.Severity, v.Policy, v.Resource, v.Namespace, v.Cluster, v.Message, v.Resolved, v.DetectedAt, tenantID,
		)
		if err != nil {
			return fmt.Errorf("inserting compliance violation: %w", err)
		}
	}

	return nil
}

func (s *complianceScannerStore) InsertAuditLog(ctx context.Context, log *usecaseCompliance.AuditLogRecord) error {
	detailsJSON, err := json.Marshal(log.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	tenantID := log.TenantID
	if tenantID == "" {
		tenantID = TenantIDOrDefault(ctx)
	}

	logID := uuid.NewString()

	query := `
		INSERT INTO audit_logs (id, actor, action, target_type, target_name, result, details, ip_address, user_agent, tenant_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`
	_, err = s.getDB(ctx).Exec(ctx, query,
		logID, log.Actor, log.Action, log.TargetType, log.TargetName, log.Result, detailsJSON, log.IPAddress, log.UserAgent, tenantID,
	)
	if err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}
	return nil
}

func (s *complianceScannerStore) ReplaceAuditFindings(ctx context.Context, findings []usecaseCompliance.AuditFindingRecord) error {
	tenantID := TenantIDOrDefault(ctx)

	delQuery := `
		DELETE FROM audit_findings 
		WHERE status = 'open' 
		  AND category IN ('runtime_security', 'database_network', 'database_transport', 'kubelet_api', 'control_plane', 'container_runtime', 'process_table')
	`
	delQuery, delArgs := BuildTenantQuery(ctx, delQuery)
	if _, err := s.getDB(ctx).Exec(ctx, delQuery, delArgs...); err != nil {
		return fmt.Errorf("deleting stale runtime audit findings: %w", err)
	}

	insertQuery := `
		INSERT INTO audit_findings (id, category, severity, description, remediation, status, detected_at, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			category = EXCLUDED.category,
			severity = EXCLUDED.severity,
			description = EXCLUDED.description,
			remediation = EXCLUDED.remediation,
			status = EXCLUDED.status,
			detected_at = EXCLUDED.detected_at,
			tenant_id = EXCLUDED.tenant_id
	`
	for _, f := range findings {
		fID := f.ID
		if fID == "" {
			fID = uuid.NewString()
		}
		fTenant := f.TenantID
		if fTenant == "" {
			fTenant = tenantID
		}
		fDetected := f.DetectedAt
		if fDetected.IsZero() {
			fDetected = time.Now().UTC()
		}
		fStatus := f.Status
		if fStatus == "" {
			fStatus = "open"
		}
		_, err := s.getDB(ctx).Exec(ctx, insertQuery,
			fID, f.Category, f.Severity, f.Description, f.Remediation, fStatus, fDetected, fTenant,
		)
		if err != nil {
			return fmt.Errorf("inserting audit finding %s: %w", fID, err)
		}
	}

	return nil
}
