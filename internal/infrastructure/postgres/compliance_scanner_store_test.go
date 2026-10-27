package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

type mockScannerDB struct {
	lastExecSQL  string
	lastExecArgs []any
	hosts        []usecaseCompliance.ComputeHostRow
}

type mockScannerRows struct {
	hosts []usecaseCompliance.ComputeHostRow
	idx   int
}

func (m *mockScannerRows) Close() {}
func (m *mockScannerRows) Err() error { return nil }
func (m *mockScannerRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (m *mockScannerRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockScannerRows) Next() bool {
	return m.idx < len(m.hosts)
}
func (m *mockScannerRows) Scan(dest ...any) error {
	h := m.hosts[m.idx]
	m.idx++
	*dest[0].(*string) = h.ID
	*dest[1].(*string) = h.Name
	*dest[2].(*string) = h.Endpoint
	*dest[3].(*string) = h.Status
	labelBytes, _ := json.Marshal(h.Labels)
	*dest[4].(*[]byte) = labelBytes
	return nil
}
func (m *mockScannerRows) Values() ([]any, error) { return nil, nil }
func (m *mockScannerRows) RawValues() [][]byte { return nil }
func (m *mockScannerRows) Conn() *pgx.Conn { return nil }

func (m *mockScannerDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return &mockScannerRows{hosts: m.hosts}, nil
}

func (m *mockScannerDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &mockSingleRow{val: 5}
}

type mockSingleRow struct {
	val int
}

func (m *mockSingleRow) Scan(dest ...any) error {
	*dest[0].(*int) = m.val
	return nil
}

func (m *mockScannerDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.lastExecSQL = sql
	m.lastExecArgs = arguments
	return pgconn.NewCommandTag("INSERT 1"), nil
}

func TestComplianceScannerStore_ListHosts(t *testing.T) {
	mock := &mockScannerDB{
		hosts: []usecaseCompliance.ComputeHostRow{
			{
				ID:       "h-1",
				Name:     "k8smasterdeb",
				Endpoint: "http://10.10.10.60:9100",
				Status:   "connected",
				Labels: map[string]string{
					"auth_token": "tok1",
				},
			},
			{
				ID:       "h-2",
				Name:     "k8smater",
				Endpoint: "http://10.10.10.133:9100",
				Status:   "connected",
				Labels:   map[string]string{},
			},
		},
	}

	store := postgres.NewComplianceScannerStore(mock)

	hosts, err := store.ListHosts(context.Background())
	if err != nil {
		t.Fatalf("ListHosts failed: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].Address != "10.10.10.60" {
		t.Errorf("expected address 10.10.10.60, got %s", hosts[0].Address)
	}
	if hosts[0].Labels["auth_token"] != "tok1" {
		t.Errorf("expected label auth_token=tok1, got %v", hosts[0].Labels)
	}
}

func TestComplianceScannerStore_UpsertAndReplace(t *testing.T) {
	mock := &mockScannerDB{}
	store := postgres.NewComplianceScannerStore(mock)

	now := time.Now().UTC()
	fw := &usecaseCompliance.FrameworkRecord{
		ID:           "c1000001-0000-0000-0000-000000000001",
		Name:         "CIS Kubernetes Benchmark v1.8",
		Icon:         "🛡️",
		TotalChecks:  10,
		PassedChecks: 8,
		FailedChecks: 2,
		Score:        80.0,
		LastScanAt:   now,
	}

	if err := store.UpsertFramework(context.Background(), fw); err != nil {
		t.Fatalf("UpsertFramework failed: %v", err)
	}

	violations := []usecaseCompliance.ViolationRecord{
		{
			ID:          "v-1",
			FrameworkID: "c1000001-0000-0000-0000-000000000001",
			Severity:    "medium",
			Policy:      "Test Policy",
			Resource:    "test-res",
			Namespace:   "default",
			Cluster:     "k8snode",
			Message:     "Test message",
			Resolved:    false,
			DetectedAt:  now,
		},
	}

	if err := store.ReplaceViolations(context.Background(), violations); err != nil {
		t.Fatalf("ReplaceViolations failed: %v", err)
	}

	logRec := &usecaseCompliance.AuditLogRecord{
		Actor:      "compliance-scanner",
		Action:     "scan",
		TargetType: "compliance",
		TargetName: "fleet-compliance-scan",
		Result:     "success",
		Details:    map[string]any{"ok": true},
		IPAddress:  "127.0.0.1",
		UserAgent:  "test-agent",
		TenantID:   "default-tenant",
	}

	if err := store.InsertAuditLog(context.Background(), logRec); err != nil {
		t.Fatalf("InsertAuditLog failed: %v", err)
	}

	rulesCount, err := store.CountActiveAutomationRules(context.Background())
	if err != nil || rulesCount != 5 {
		t.Errorf("expected 5 rules, got %d (err: %v)", rulesCount, err)
	}

	logsCount, err := store.CountAuditLogs(context.Background())
	if err != nil || logsCount != 5 {
		t.Errorf("expected 5 logs, got %d (err: %v)", logsCount, err)
	}
}
