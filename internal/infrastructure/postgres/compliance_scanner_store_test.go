package postgres_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/usecase/cluster"
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

func getLiveScannerTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("POSTGRES_PASSWORD") == "" && os.Getenv("K8S_POSTGRES_PASSWORD") == "" {
		if f, err := os.Open(`d:\project\k8sseflhost\.env`); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
					}
				}
			}
			f.Close()
		}
	}

	host := os.Getenv("K8S_POSTGRES_HOST")
	if host == "" {
		host = "10.10.10.133"
	}
	port := os.Getenv("K8S_POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("K8S_POSTGRES_USER")
	if user == "" {
		user = "myuser"
	}
	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		pass = os.Getenv("K8S_POSTGRES_PASSWORD")
	}
	if pass == "" {
		t.Skip("Skipping live database test: POSTGRES_PASSWORD not set")
		return nil
	}
	dbname := os.Getenv("K8S_POSTGRES_DBNAME")
	if dbname == "" {
		dbname = "mydatabase"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Skipping live database test: unable to connect: %v", err)
		return nil
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping live database test: unable to ping: %v", err)
		return nil
	}
	return pool
}

func TestComplianceScannerStore_LiveDB_SaveScanResult(t *testing.T) {
	pool := getLiveScannerTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := postgres.NewComplianceScannerStore(pool)
	now := time.Now().UTC()

	// 1. Upsert Framework
	fw := &usecaseCompliance.FrameworkRecord{
		ID:           "c1000001-0000-0000-0000-000000000001",
		Name:         "CIS Kubernetes Benchmark v1.8",
		Icon:         "🛡️",
		TotalChecks:  15,
		PassedChecks: 5,
		FailedChecks: 10,
		Score:        33.33,
		LastScanAt:   now,
	}
	err := store.UpsertFramework(ctx, fw)
	require.NoError(t, err, "UpsertFramework must succeed without not-null constraint errors")

	// 2. Insert 12 live compliance violations (proves no SQLSTATE 23502)
	var violations []usecaseCompliance.ViolationRecord
	for i := 1; i <= 12; i++ {
		violations = append(violations, usecaseCompliance.ViolationRecord{
			ID:          uuid.NewString(),
			FrameworkID: fw.ID,
			Severity:    "high",
			Policy:      fmt.Sprintf("CIS Policy %d", i),
			Resource:    fmt.Sprintf("k8s-pod-%d", i),
			Namespace:   "default",
			Cluster:     "k8snode",
			Message:     fmt.Sprintf("Security hardening test violation %d", i),
			Resolved:    false,
			DetectedAt:  now,
		})
	}

	err = store.ReplaceViolations(ctx, violations)
	require.NoError(t, err, "ReplaceViolations must succeed with non-null tenant_id")

	// 3. Verify in PostgreSQL that compliance_violations has persisted rows for "default-tenant"
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM compliance_violations WHERE cluster = 'k8snode' AND tenant_id = 'default-tenant'").Scan(&count)
	require.NoError(t, err, "Querying compliance_violations failed")
	assert.GreaterOrEqual(t, count, 10, "Expected at least 10 violations persisted for default-tenant")

	// 4. Verify no violations have NULL or empty tenant_id
	var invalidCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM compliance_violations WHERE tenant_id IS NULL OR tenant_id = ''").Scan(&invalidCount)
	require.NoError(t, err, "Querying invalid tenant_id failed")
	assert.Equal(t, 0, invalidCount, "No violations should have NULL or empty tenant_id")

	// 5. ReplaceAuditFindings test
	var findings []usecaseCompliance.AuditFindingRecord
	for i := 1; i <= 3; i++ {
		findings = append(findings, usecaseCompliance.AuditFindingRecord{
			ID:          uuid.NewString(),
			Category:    "runtime_security",
			Severity:    "high",
			Description: fmt.Sprintf("Live test audit finding %d", i),
			Remediation: "Enforce network policies",
			Status:      "open",
			DetectedAt:  now,
			TenantID:    "default-tenant",
		})
	}
	err = store.ReplaceAuditFindings(ctx, findings)
	require.NoError(t, err, "ReplaceAuditFindings must succeed")
}

type mockDiscoveryForLiveHealth struct{}

func (m *mockDiscoveryForLiveHealth) DiscoverResources(ctx context.Context, kubeconfig []byte) (map[string]interface{}, error) {
	return map[string]interface{}{"nodes_count": 3}, nil
}

func (m *mockDiscoveryForLiveHealth) ValidateConnectivity(ctx context.Context, kubeconfig []byte) error {
	return errors.New("k8s API server connectivity refused")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestHealthChecker_LiveDB_BackgroundContext_UpdatesK8sNode(t *testing.T) {
	pool := getLiveScannerTestPool(t)
	if pool == nil {
		return
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fleetRepo := postgres.NewFleetRepo(pool)
	computeHostRepo := postgres.NewComputeHostRepo(pool)
	discovery := &mockDiscoveryForLiveHealth{}

	// TCP dialer: candidate APIServer :6443 is unreachable (simulating real k8snode setup)
	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, errors.New("dial tcp: connection refused")
	}

	// HTTP client: node agents respond 200 OK on :9100/health
	mockClient := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if strings.HasSuffix(r.URL.Path, "/health") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`))),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	checker := cluster.NewHealthChecker(
		fleetRepo,
		discovery,
		zap.NewNop(),
		cluster.WithComputeHostRepo(computeHostRepo),
		cluster.WithDialContext(dialFn),
		cluster.WithHTTPClient(mockClient),
	)

	// Call CheckAll with bare context.Background() - this directly tests that background ctx
	// is properly enriched with platform_admin + default-tenant and updates k8snode!
	checker.CheckAll(context.Background())

	// Verify k8snode in PostgreSQL fleet_clusters
	var status, healthStatus, tenantID string
	var nodes int
	err := pool.QueryRow(ctx, "SELECT status, health_status, nodes, tenant_id FROM fleet_clusters WHERE id = 'k8snode' OR name = 'k8snode'").
		Scan(&status, &healthStatus, &nodes, &tenantID)
	require.NoError(t, err, "Failed to query k8snode in fleet_clusters")

	assert.Equal(t, "degraded", healthStatus, "k8snode health_status should be updated to degraded")
	assert.Equal(t, "degraded", status, "k8snode status should be updated to degraded")
	assert.GreaterOrEqual(t, nodes, 1, "k8snode nodes count should reflect live node agents")
	assert.Equal(t, "default-tenant", tenantID, "k8snode tenant_id should be default-tenant")
}
