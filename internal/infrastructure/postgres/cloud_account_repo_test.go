package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/cloud"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func init() {
	if os.Getenv("ENCRYPTION_KEY") == "" {
		os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	}
}

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("K8S_POSTGRES_HOST")
	if host == "" {
		t.Skip("Skipping test: K8S_POSTGRES_HOST is not set")
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
		t.Fatalf("failed to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping database: %v", err)
	}

	return pool
}

func tenantContext(tenantID string) context.Context {
	return context.WithValue(context.Background(), tenancy.TenantIDKey, tenantID)
}

func adminContext() context.Context {
	return context.WithValue(context.Background(), tenancy.UserRoleKey, "platform_admin")
}

func TestCloudAccountRepo_CreateAndGetRoundtrip(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("test-tenant-rt-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCloudAccountRepo(pool)

	rawCreds := `{"aws_access_key_id":"AKIAIOSFODNN7EXAMPLE","aws_secret_access_key":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}`
	account := cloud.NewCloudAccount("test-aws-roundtrip", cloud.ProviderAWS, rawCreds, "us-east-1", tenantID)

	// Create
	if err := repo.Create(ctx, account); err != nil {
		t.Fatalf("failed to create cloud account: %v", err)
	}
	if account.ID == "" {
		t.Fatalf("expected account.ID to be populated")
	}
	defer func() {
		_ = repo.Delete(ctx, account.ID)
	}()

	// Verify credentials are encrypted at rest in the DB
	var storedCreds string
	err := pool.QueryRow(context.Background(), "SELECT encrypted_creds FROM cloud_accounts WHERE id = $1", account.ID).Scan(&storedCreds)
	if err != nil {
		t.Fatalf("failed to query raw encrypted_creds: %v", err)
	}
	if storedCreds == rawCreds {
		t.Fatalf("expected stored credentials to be encrypted, got plaintext: %s", storedCreds)
	}

	// GetByID - should decrypt credentials
	fetched, err := repo.GetByID(ctx, account.ID)
	if err != nil {
		t.Fatalf("failed to get cloud account by ID: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected fetched account to be non-nil")
	}
	if fetched.ID != account.ID {
		t.Errorf("ID mismatch: got %s, want %s", fetched.ID, account.ID)
	}
	if fetched.Name != account.Name {
		t.Errorf("Name mismatch: got %s, want %s", fetched.Name, account.Name)
	}
	if fetched.Provider != cloud.ProviderAWS {
		t.Errorf("Provider mismatch: got %s, want %s", fetched.Provider, cloud.ProviderAWS)
	}
	if fetched.Region != "us-east-1" {
		t.Errorf("Region mismatch: got %s, want us-east-1", fetched.Region)
	}
	if fetched.EncryptedCreds != rawCreds {
		t.Errorf("EncryptedCreds decrypted mismatch: got %s, want %s", fetched.EncryptedCreds, rawCreds)
	}
}

func TestCloudAccountRepo_ListExcludesCreds(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("test-tenant-list-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCloudAccountRepo(pool)

	acc1 := cloud.NewCloudAccount("list-aws-account", cloud.ProviderAWS, `{"key":"aws-secret"}`, "us-west-2", tenantID)
	acc2 := cloud.NewCloudAccount("list-gcp-account", cloud.ProviderGCP, `{"key":"gcp-secret"}`, "us-central1", tenantID)

	if err := repo.Create(ctx, acc1); err != nil {
		t.Fatalf("failed to create acc1: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, acc1.ID) }()

	if err := repo.Create(ctx, acc2); err != nil {
		t.Fatalf("failed to create acc2: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, acc2.ID) }()

	list, err := repo.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("failed to list cloud accounts: %v", err)
	}
	if len(list) < 2 {
		t.Fatalf("expected at least 2 accounts, got %d", len(list))
	}

	for _, item := range list {
		if item.EncryptedCreds != "" {
			t.Errorf("expected list item creds to be empty, got: %s", item.EncryptedCreds)
		}
	}
}

func TestCloudAccountRepo_Delete(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("test-tenant-del-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCloudAccountRepo(pool)

	acc := cloud.NewCloudAccount("delete-test-account", cloud.ProviderAzure, `{"client_secret":"azure123"}`, "eastus", tenantID)
	if err := repo.Create(ctx, acc); err != nil {
		t.Fatalf("failed to create cloud account: %v", err)
	}

	if err := repo.Delete(ctx, acc.ID); err != nil {
		t.Fatalf("failed to delete cloud account: %v", err)
	}

	fetched, err := repo.GetByID(ctx, acc.ID)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if fetched != nil {
		t.Fatalf("expected account to be nil after deletion, got: %+v", fetched)
	}

	// Deleting non-existent should return an error
	if err := repo.Delete(ctx, acc.ID); err == nil {
		t.Fatalf("expected error deleting already deleted account, got nil")
	}
}

func TestCloudAccountRepo_DuplicateNameHandling(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("test-tenant-dup-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCloudAccountRepo(pool)

	accName := "duplicate-account-name"

	acc1 := cloud.NewCloudAccount(accName, cloud.ProviderAWS, `{"key":"secret1"}`, "us-east-1", tenantID)
	if err := repo.Create(ctx, acc1); err != nil {
		t.Fatalf("failed to create first cloud account: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, acc1.ID) }()

	acc2 := cloud.NewCloudAccount(accName, cloud.ProviderGCP, `{"key":"secret2"}`, "us-central1", tenantID)
	if err := repo.Create(ctx, acc2); err == nil {
		_ = repo.Delete(ctx, acc2.ID)
		t.Fatalf("expected error creating duplicate account name in same tenant, got nil")
	}
}

func TestCloudAccountRepo_Update(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantID := fmt.Sprintf("test-tenant-upd-%d", time.Now().UnixNano())
	ctx := tenantContext(tenantID)
	repo := postgres.NewCloudAccountRepo(pool)

	acc := cloud.NewCloudAccount("update-test-acc", cloud.ProviderAWS, `{"initial":"creds"}`, "us-east-1", tenantID)
	if err := repo.Create(ctx, acc); err != nil {
		t.Fatalf("failed to create account: %v", err)
	}
	defer func() { _ = repo.Delete(ctx, acc.ID) }()

	acc.Region = "us-west-1"
	acc.EncryptedCreds = `{"updated":"creds"}`
	if err := repo.Update(ctx, acc); err != nil {
		t.Fatalf("failed to update account: %v", err)
	}

	fetched, err := repo.GetByID(ctx, acc.ID)
	if err != nil {
		t.Fatalf("failed to get updated account: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected fetched account to be non-nil")
	}
	if fetched.Region != "us-west-1" {
		t.Errorf("Region mismatch: got %s, want us-west-1", fetched.Region)
	}
	if fetched.EncryptedCreds != `{"updated":"creds"}` {
		t.Errorf("EncryptedCreds mismatch: got %s, want updated creds", fetched.EncryptedCreds)
	}
}

func TestCloudAccountRepo_TenantIsolation(t *testing.T) {
	pool := getTestPool(t)
	defer pool.Close()

	tenantA := fmt.Sprintf("tenant-a-%d", time.Now().UnixNano())
	tenantB := fmt.Sprintf("tenant-b-%d", time.Now().UnixNano())

	ctxA := tenantContext(tenantA)
	ctxB := tenantContext(tenantB)

	repo := postgres.NewCloudAccountRepo(pool)

	accA := cloud.NewCloudAccount("acc-a", cloud.ProviderAWS, "creds-a", "us-east-1", tenantA)
	if err := repo.Create(ctxA, accA); err != nil {
		t.Fatalf("failed to create accA: %v", err)
	}
	defer func() {
		adminCtx := adminContext()
		_ = repo.Delete(adminCtx, accA.ID)
	}()

	// Tenant B cannot Get Tenant A's account
	fetchedByB, err := repo.GetByID(ctxB, accA.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fetchedByB != nil {
		t.Fatalf("Tenant B was able to access Tenant A's account: %+v", fetchedByB)
	}

	// Platform Admin can access Tenant A's account
	adminCtx := adminContext()
	fetchedByAdmin, err := repo.GetByID(adminCtx, accA.ID)
	if err != nil {
		t.Fatalf("admin get failed: %v", err)
	}
	if fetchedByAdmin == nil || fetchedByAdmin.ID != accA.ID {
		t.Fatalf("admin could not get account: %+v", fetchedByAdmin)
	}
}
