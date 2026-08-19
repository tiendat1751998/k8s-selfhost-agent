package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func init() {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	}
}

func getComputeHostTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("K8S_POSTGRES_HOST")
	if host == "" {
		host = "10.10.10.133"
	}

	user := os.Getenv("K8S_POSTGRES_USER")
	if user == "" {
		user = "myuser"
	}
	pass := os.Getenv("K8S_POSTGRES_PASSWORD")
	if pass == "" {
		pass = "mysecretpassword"
	}
	dbname := os.Getenv("K8S_POSTGRES_DBNAME")
	if dbname == "" {
		dbname = "mydatabase"
	}
	port := os.Getenv("K8S_POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbname)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping test: database ping failed: %v", err)
	}

	// Ensure table exists for tests
	migrationSQL := `
		CREATE TABLE IF NOT EXISTS compute_hosts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			host_type VARCHAR(50) NOT NULL DEFAULT 'docker',
			endpoint VARCHAR(512) NOT NULL,
			tls_enabled BOOLEAN NOT NULL DEFAULT false,
			tls_ca TEXT,
			tls_cert TEXT,
			tls_key TEXT,
			api_version VARCHAR(20) DEFAULT '',
			status VARCHAR(50) NOT NULL DEFAULT 'pending',
			last_health_check TIMESTAMPTZ,
			labels JSONB DEFAULT '{}',
			tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(name, tenant_id)
		);
		CREATE INDEX IF NOT EXISTS idx_compute_hosts_tenant ON compute_hosts(tenant_id);
	`
	_, _ = pool.Exec(ctx, migrationSQL)

	return pool
}

func computeHostTenantContext(tenantID string) context.Context {
	return context.WithValue(context.Background(), tenancy.TenantIDKey, tenantID)
}

func computeHostAdminContext() context.Context {
	return context.WithValue(context.Background(), tenancy.UserRoleKey, "platform_admin")
}

func TestComputeHostRepo_CreateAndGetRoundtrip(t *testing.T) {
	pool := getComputeHostTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("test-tenant-%d", time.Now().UnixNano())
	ctx := computeHostTenantContext(tenantID)
	repo := postgres.NewComputeHostRepo(pool)

	rawTLSKey := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0...\n-----END RSA PRIVATE KEY-----"
	host := &docker.ComputeHost{
		Name:       "docker-swarm-node-1",
		HostType:   "docker",
		Endpoint:   "tcp://10.10.10.133:2375",
		TLSEnabled: true,
		TLSCA:      "-----BEGIN CERTIFICATE-----\nCA...\n-----END CERTIFICATE-----",
		TLSCert:    "-----BEGIN CERTIFICATE-----\nCERT...\n-----END CERTIFICATE-----",
		TLSKey:     rawTLSKey,
		APIVersion: "1.44",
		Status:     "pending",
		Labels:     map[string]string{"env": "staging", "zone": "us-east-1"},
		TenantID:   tenantID,
	}

	// 1. Create
	if err := repo.Create(ctx, host); err != nil {
		t.Fatalf("failed to create compute host: %v", err)
	}
	if host.ID == "" {
		t.Fatalf("expected host.ID to be populated")
	}
	defer func() {
		_ = repo.Delete(ctx, host.ID)
	}()

	// 2. Verify encrypted at rest in DB
	var storedKey string
	err := pool.QueryRow(context.Background(), "SELECT tls_key FROM compute_hosts WHERE id = $1", host.ID).Scan(&storedKey)
	if err != nil {
		t.Fatalf("failed to query raw tls_key: %v", err)
	}
	if storedKey == rawTLSKey {
		t.Fatalf("SECURITY VIOLATION: TLS key stored in plaintext!")
	}

	// 3. GetByID and verify decrypted key
	fetched, err := repo.GetByID(ctx, host.ID)
	if err != nil {
		t.Fatalf("failed to get compute host by ID: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected compute host to be found")
	}
	if fetched.TLSKey != rawTLSKey {
		t.Errorf("expected decrypted TLS key %q, got %q", rawTLSKey, fetched.TLSKey)
	}
	if fetched.Labels["env"] != "staging" || fetched.Labels["zone"] != "us-east-1" {
		t.Errorf("labels mismatch: %+v", fetched.Labels)
	}
}

func TestComputeHostRepo_TenantIsolation(t *testing.T) {
	pool := getComputeHostTestPool(t)
	defer pool.Close()

	tenantA := fmt.Sprintf("tenant-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-b-%d", time.Now().UnixNano())

	ctxA := computeHostTenantContext(tenantA)
	ctxB := computeHostTenantContext(tenantB)
	ctxAdmin := computeHostAdminContext()

	repo := postgres.NewComputeHostRepo(pool)

	hostA := &docker.ComputeHost{
		Name:     "host-a",
		Endpoint: "tcp://10.10.10.1:2375",
		TenantID: tenantA,
	}
	if err := repo.Create(ctxA, hostA); err != nil {
		t.Fatalf("failed to create host A: %v", err)
	}
	defer func() { _ = repo.Delete(ctxAdmin, hostA.ID) }()

	// Tenant B cannot fetch Tenant A's host
	fetchedB, err := repo.GetByID(ctxB, hostA.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchedB != nil {
		t.Fatalf("SECURITY VIOLATION: Tenant B accessed Tenant A's compute host")
	}

	// Admin can fetch Tenant A's host
	fetchedAdmin, err := repo.GetByID(ctxAdmin, hostA.ID)
	if err != nil {
		t.Fatalf("admin get failed: %v", err)
	}
	if fetchedAdmin == nil || fetchedAdmin.ID != hostA.ID {
		t.Fatalf("expected admin to find host A")
	}
}

func TestComputeHostRepo_UpdateAndStatus(t *testing.T) {
	pool := getComputeHostTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("tenant-upd-%d", time.Now().UnixNano())
	ctx := computeHostTenantContext(tenantID)
	repo := postgres.NewComputeHostRepo(pool)

	host := &docker.ComputeHost{
		Name:     "initial-host",
		Endpoint: "tcp://10.10.10.10:2375",
		TenantID: tenantID,
	}
	if err := repo.Create(ctx, host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, host.ID) }()

	// Update host
	host.Name = "renamed-host"
	host.Endpoint = "tcp://10.10.10.11:2375"
	if err := repo.Update(ctx, host); err != nil {
		t.Fatalf("failed to update host: %v", err)
	}

	updated, err := repo.GetByID(ctx, host.ID)
	if err != nil {
		t.Fatalf("failed to get updated host: %v", err)
	}
	if updated.Name != "renamed-host" || updated.Endpoint != "tcp://10.10.10.11:2375" {
		t.Errorf("unexpected updated host: %+v", updated)
	}

	// Update Status
	now := time.Now().UTC().Truncate(time.Millisecond)
	if err := repo.UpdateStatus(ctx, host.ID, "connected", now); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	statusCheck, err := repo.GetByID(ctx, host.ID)
	if err != nil {
		t.Fatalf("failed to get host after status update: %v", err)
	}
	if statusCheck.Status != "connected" {
		t.Errorf("expected status 'connected', got %s", statusCheck.Status)
	}
}

func TestComputeHostRepo_ListAll(t *testing.T) {
	pool := getComputeHostTestPool(t)
	defer pool.Close()

	tenantA := fmt.Sprintf("tenant-listall-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-listall-b-%d", time.Now().UnixNano())

	ctxA := computeHostTenantContext(tenantA)
	ctxB := computeHostTenantContext(tenantB)
	ctxAdmin := computeHostAdminContext()
	ctxBg := context.Background()

	repo := postgres.NewComputeHostRepo(pool)

	hostA := &docker.ComputeHost{
		Name:     fmt.Sprintf("host-listall-a-%d", time.Now().UnixNano()),
		Endpoint: "tcp://10.10.10.1:2375",
		TenantID: tenantA,
	}
	if err := repo.Create(ctxA, hostA); err != nil {
		t.Fatalf("failed to create host A: %v", err)
	}
	defer func() { _ = repo.Delete(ctxAdmin, hostA.ID) }()

	hostB := &docker.ComputeHost{
		Name:     fmt.Sprintf("host-listall-b-%d", time.Now().UnixNano()),
		Endpoint: "tcp://10.10.10.2:2375",
		TenantID: tenantB,
	}
	if err := repo.Create(ctxB, hostB); err != nil {
		t.Fatalf("failed to create host B: %v", err)
	}
	defer func() { _ = repo.Delete(ctxAdmin, hostB.ID) }()

	// ListAll with empty background context (no tenant) should return both hosts
	allHosts, err := repo.ListAll(ctxBg)
	if err != nil {
		t.Fatalf("ListAll failed: %v", err)
	}

	foundA, foundB := false, false
	for _, h := range allHosts {
		if h.ID == hostA.ID {
			foundA = true
		}
		if h.ID == hostB.ID {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Errorf("expected ListAll to find both host A and host B, got foundA=%v, foundB=%v", foundA, foundB)
	}
}

