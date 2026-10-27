package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/compliance"
	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

type mockComplianceRepo struct {
	frameworks []compliance.Framework
	violations []compliance.Violation
}

func (m *mockComplianceRepo) ListFrameworks(ctx context.Context) ([]compliance.Framework, error) {
	return m.frameworks, nil
}
func (m *mockComplianceRepo) GetFramework(ctx context.Context, id string) (*compliance.Framework, error) {
	return nil, nil
}
func (m *mockComplianceRepo) UpsertFramework(ctx context.Context, f *compliance.Framework) error {
	return nil
}
func (m *mockComplianceRepo) ListViolations(ctx context.Context, severity *compliance.ViolationSeverity, limit, offset int) ([]compliance.Violation, int, error) {
	return m.violations, len(m.violations), nil
}
func (m *mockComplianceRepo) GetViolation(ctx context.Context, id string) (*compliance.Violation, error) {
	for _, v := range m.violations {
		if v.ID == id {
			return &v, nil
		}
	}
	return nil, nil
}
func (m *mockComplianceRepo) CreateViolation(ctx context.Context, v *compliance.Violation) error {
	return nil
}
func (m *mockComplianceRepo) ResolveViolation(ctx context.Context, id string) error {
	return nil
}

type mockComplianceScannerStore struct {
	scanned bool
}

func (m *mockComplianceScannerStore) ListHosts(ctx context.Context) ([]usecaseCompliance.ComputeHostRow, error) {
	m.scanned = true
	return []usecaseCompliance.ComputeHostRow{}, nil
}
func (m *mockComplianceScannerStore) CountActiveAutomationRules(ctx context.Context) (int, error) {
	return 1, nil
}
func (m *mockComplianceScannerStore) CountAuditLogs(ctx context.Context) (int, error) {
	return 1, nil
}
func (m *mockComplianceScannerStore) UpsertFramework(ctx context.Context, fw *usecaseCompliance.FrameworkRecord) error {
	return nil
}
func (m *mockComplianceScannerStore) ReplaceViolations(ctx context.Context, violations []usecaseCompliance.ViolationRecord) error {
	return nil
}
func (m *mockComplianceScannerStore) ReplaceAuditFindings(ctx context.Context, findings []usecaseCompliance.AuditFindingRecord) error {
	return nil
}
func (m *mockComplianceScannerStore) InsertAuditLog(ctx context.Context, log *usecaseCompliance.AuditLogRecord) error {
	return nil
}

func TestComplianceHandler_ScanNow(t *testing.T) {
	repo := &mockComplianceRepo{}
	mockStore := &mockComplianceScannerStore{}
	scanner := usecaseCompliance.NewScanner(usecaseCompliance.WithDBStore(mockStore))

	handler := adapthttp.NewComplianceHandler(repo, adapthttp.WithComplianceScanner(scanner))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/compliance/scan", nil)
	rec := httptest.NewRecorder()

	handler.ScanNow(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	if !mockStore.scanned {
		t.Errorf("expected scanner to be executed")
	}
}

func TestComplianceHandler_BackgroundScanOnBoot(t *testing.T) {
	repo := &mockComplianceRepo{}
	mockStore := &mockComplianceScannerStore{}
	scanner := usecaseCompliance.NewScanner(usecaseCompliance.WithDBStore(mockStore))

	// Creating handler should start background scan
	_ = adapthttp.NewComplianceHandler(repo, adapthttp.WithComplianceScanner(scanner), adapthttp.WithBootScan(true))

	// Allow brief time for goroutine
	time.Sleep(50 * time.Millisecond)

	if !mockStore.scanned {
		t.Errorf("expected background scan on boot to run")
	}
}
