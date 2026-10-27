package compliance_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

// mockDBTX implements usecaseCompliance.DBTX for unit testing.
type mockDBTX struct {
	hosts            []usecaseCompliance.ComputeHostRow
	automationCount int
	auditLogCount    int
	upsertedFw       map[string]*usecaseCompliance.FrameworkRecord
	deletedViolations bool
	insertedViolations []usecaseCompliance.ViolationRecord
	insertedFindings   []usecaseCompliance.AuditFindingRecord
	insertedAuditLogs  []usecaseCompliance.AuditLogRecord
}

func (m *mockDBTX) ListHosts(ctx context.Context) ([]usecaseCompliance.ComputeHostRow, error) {
	return m.hosts, nil
}

func (m *mockDBTX) CountActiveAutomationRules(ctx context.Context) (int, error) {
	return m.automationCount, nil
}

func (m *mockDBTX) CountAuditLogs(ctx context.Context) (int, error) {
	return m.auditLogCount, nil
}

func (m *mockDBTX) UpsertFramework(ctx context.Context, fw *usecaseCompliance.FrameworkRecord) error {
	if m.upsertedFw == nil {
		m.upsertedFw = make(map[string]*usecaseCompliance.FrameworkRecord)
	}
	m.upsertedFw[fw.Name] = fw
	return nil
}

func (m *mockDBTX) ReplaceViolations(ctx context.Context, violations []usecaseCompliance.ViolationRecord) error {
	m.deletedViolations = true
	m.insertedViolations = violations
	return nil
}

func (m *mockDBTX) ReplaceAuditFindings(ctx context.Context, findings []usecaseCompliance.AuditFindingRecord) error {
	m.insertedFindings = findings
	return nil
}

func (m *mockDBTX) InsertAuditLog(ctx context.Context, log *usecaseCompliance.AuditLogRecord) error {
	m.insertedAuditLogs = append(m.insertedAuditLogs, *log)
	return nil
}

func TestFleetComplianceScanner_FullScan(t *testing.T) {
	// Setup a mock agent server mimicking :9100
	agentMux := http.NewServeMux()
	agentMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-agent-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hostname": "k8smasterdeb",
			"memory": map[string]interface{}{
				"usage_percent": 42.5,
			},
			"disks": []map[string]interface{}{
				{
					"mount_point":   "/",
					"usage_percent": 46.3,
				},
			},
		})
	})

	agentMux.HandleFunc("/k8s/health", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-agent-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"node_name":          "k8smasterdeb",
			"zombie_count":       0,
			"containerd_healthy": true,
			"docker_healthy":     false,
		})
	})

	agentServer := httptest.NewServer(agentMux)
	defer agentServer.Close()

	agentURL := agentServer.URL
	_, agentPort, err := net.SplitHostPort(strings.TrimPrefix(agentURL, "http://"))
	if err != nil {
		t.Fatalf("failed to split host port: %v", err)
	}

	mockDB := &mockDBTX{
		hosts: []usecaseCompliance.ComputeHostRow{
			{
				ID:       "host-1",
				Name:     "k8smasterdeb",
				Endpoint: agentURL,
				Address:  "127.0.0.1",
				Status:   "connected",
				Labels: map[string]string{
					"auth_token": "test-agent-token",
				},
			},
		},
		automationCount: 4,
		auditLogCount:   10,
	}

	scanner := usecaseCompliance.NewScanner(
		usecaseCompliance.WithDBStore(mockDB),
		usecaseCompliance.WithHTTPClient(agentServer.Client()),
		usecaseCompliance.WithKubeletProbePort(agentPort), // point kubelet probe to agent server so it is open
	)

	_ = os.Setenv("K8S_POSTGRES_SSLMODE", "disable")

	res, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("scanner.Scan failed: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil ScanResult")
	}

	// Verify CIS Benchmark was scanned and scored
	cis, ok := mockDB.upsertedFw["CIS Kubernetes Benchmark v1.8"]
	if !ok {
		t.Fatalf("CIS benchmark was not upserted into DB")
	}
	if cis.TotalChecks == 0 {
		t.Errorf("expected > 0 CIS total checks, got %d", cis.TotalChecks)
	}
	if cis.Score <= 0 || cis.Score > 100 {
		t.Errorf("invalid CIS score: %f", cis.Score)
	}

	// Verify NSA guidance was scanned
	nsa, ok := mockDB.upsertedFw["NSA-CISA Kubernetes Hardening Guidance"]
	if !ok {
		t.Fatalf("NSA guidance was not upserted into DB")
	}
	if nsa.TotalChecks == 0 {
		t.Errorf("expected > 0 NSA total checks, got %d", nsa.TotalChecks)
	}

	// Verify SOC 2 controls were scanned
	soc2, ok := mockDB.upsertedFw["SOC 2 Type II Security Controls"]
	if !ok {
		t.Fatalf("SOC 2 controls were not upserted into DB")
	}
	if soc2.TotalChecks == 0 {
		t.Errorf("expected > 0 SOC 2 total checks, got %d", soc2.TotalChecks)
	}

	// Verify violations were replaced in DB
	if !mockDB.deletedViolations {
		t.Errorf("expected stale violations to be cleared")
	}
	if len(mockDB.insertedViolations) == 0 {
		t.Errorf("expected at least 1 violation (e.g. sslmode=disable or kubelet exposed)")
	}

	// Verify audit log record was inserted
	if len(mockDB.insertedAuditLogs) == 0 {
		t.Errorf("expected audit log record to be inserted")
	}
}

func TestFleetComplianceScanner_ZeroHosts(t *testing.T) {
	mockDB := &mockDBTX{
		hosts:           []usecaseCompliance.ComputeHostRow{},
		automationCount: 0,
		auditLogCount:   0,
	}

	scanner := usecaseCompliance.NewScanner(
		usecaseCompliance.WithDBStore(mockDB),
	)

	res, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan with zero hosts failed: %v", err)
	}
	if len(res.Frameworks) != 3 {
		t.Fatalf("expected 3 frameworks, got %d", len(res.Frameworks))
	}
}

func TestFleetComplianceScanner_ZeroTrustViolation(t *testing.T) {
	// Agent that incorrectly returns 200 without token
	badAgent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "insecure"})
	}))
	defer badAgent.Close()

	mockDB := &mockDBTX{
		hosts: []usecaseCompliance.ComputeHostRow{
			{
				ID:       "host-bad",
				Name:     "insecure-node",
				Endpoint: badAgent.URL,
				Address:  "127.0.0.1",
				Status:   "connected",
			},
		},
	}

	scanner := usecaseCompliance.NewScanner(
		usecaseCompliance.WithDBStore(mockDB),
		usecaseCompliance.WithHTTPClient(badAgent.Client()),
	)

	res, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	foundZeroTrustViolation := false
	for _, v := range res.Violations {
		if v.Policy == "Zero-Trust Agent Auth Enforcement" && v.Severity == "critical" {
			foundZeroTrustViolation = true
			break
		}
	}
	if !foundZeroTrustViolation {
		t.Errorf("expected critical Zero-Trust Agent Auth Enforcement violation")
	}
}

