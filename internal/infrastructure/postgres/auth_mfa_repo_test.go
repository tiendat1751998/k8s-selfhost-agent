package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datdt/k8sselfhost/internal/domain/user"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

func getAuthTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
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
	pass := os.Getenv("K8S_POSTGRES_PASSWORD")
	if pass == "" {
		pass = "mysecretpassword"
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
		t.Skipf("Skipping test: cannot connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("Skipping test: database ping failed: %v", err)
	}

	return pool
}

func createTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	userID := uuid.New().String()
	email := fmt.Sprintf("test-user-%s@example.com", userID[:8])
	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name)
		VALUES ($1, $2, '$2a$10$xseVQz9NZxRiL6XHVHfIxec2sPO3vH2gfRjgRCXXJfrc64/hCDzzq', 'Test', 'User')
		ON CONFLICT (id) DO NOTHING
	`
	_, err := pool.Exec(context.Background(), query, userID, email)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	return userID
}

func deleteTestUser(pool *pgxpool.Pool, userID string) {
	_, _ = pool.Exec(context.Background(), "DELETE FROM refresh_tokens WHERE user_id = $1", userID)
	_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", userID)
}

func TestRefreshTokenRepo_CRUD(t *testing.T) {
	pool := getAuthTestPool(t)
	defer pool.Close()

	userID := createTestUser(t, pool)
	defer deleteTestUser(pool, userID)

	repo := postgres.NewRefreshTokenRepo(pool)
	ctx := context.Background()

	tokenHash := fmt.Sprintf("hash-%s", uuid.New().String())
	rt := &user.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		UserAgent: "Mozilla/5.0 Test",
		IPAddress: "127.0.0.1",
	}

	// 1. Create
	if err := repo.Create(ctx, rt); err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}
	if rt.ID == "" {
		t.Fatal("expected non-empty ID after creation")
	}

	// 2. GetByHash
	fetched, err := repo.GetByHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("failed to get refresh token by hash: %v", err)
	}
	if fetched.ID != rt.ID || fetched.UserID != userID || fetched.TokenHash != tokenHash {
		t.Fatalf("unexpected fetched token: %+v", fetched)
	}
	if fetched.UserAgent != "Mozilla/5.0 Test" || fetched.IPAddress != "127.0.0.1" {
		t.Fatalf("unexpected metadata in fetched token: %+v", fetched)
	}

	// 3. Revoke single token
	if err := repo.Revoke(ctx, rt.ID); err != nil {
		t.Fatalf("failed to revoke token: %v", err)
	}
	revoked, err := repo.GetByHash(ctx, tokenHash)
	if err != nil {
		t.Fatalf("failed to get revoked token: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("expected RevokedAt to be set")
	}

	// 4. RevokeAllForUser
	tokenHash2 := fmt.Sprintf("hash2-%s", uuid.New().String())
	rt2 := &user.RefreshToken{
		UserID:    userID,
		TokenHash: tokenHash2,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := repo.Create(ctx, rt2); err != nil {
		t.Fatalf("failed to create token2: %v", err)
	}
	if err := repo.RevokeAllForUser(ctx, userID); err != nil {
		t.Fatalf("failed to revoke all tokens for user: %v", err)
	}
	revoked2, err := repo.GetByHash(ctx, tokenHash2)
	if err != nil {
		t.Fatalf("failed to get revoked token2: %v", err)
	}
	if revoked2.RevokedAt == nil {
		t.Fatal("expected token2 to be revoked")
	}

	// 5. DeleteExpired
	expiredHash := fmt.Sprintf("expired-%s", uuid.New().String())
	rtExpired := &user.RefreshToken{
		UserID:    userID,
		TokenHash: expiredHash,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	if err := repo.Create(ctx, rtExpired); err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}
	deletedCount, err := repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("failed to delete expired tokens: %v", err)
	}
	if deletedCount < 1 {
		t.Fatalf("expected at least 1 deleted token, got %d", deletedCount)
	}
}

func TestUserRepo_MFA_Fields(t *testing.T) {
	pool := getAuthTestPool(t)
	defer pool.Close()

	userID := createTestUser(t, pool)
	defer deleteTestUser(pool, userID)

	repo := postgres.NewUserRepo(pool)
	ctx := context.Background()

	// 1. Initial lookup
	usr, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if usr.MFAEnabled {
		t.Fatal("expected MFA to be disabled initially")
	}

	// 2. Set MFA Secret (unverified)
	encSecret := "encrypted-mfa-secret-12345"
	if err := repo.UpdateMFA(ctx, userID, encSecret, false); err != nil {
		t.Fatalf("UpdateMFA(unverified) failed: %v", err)
	}

	secret, err := repo.GetMFASecret(ctx, userID)
	if err != nil {
		t.Fatalf("GetMFASecret failed: %v", err)
	}
	if secret != encSecret {
		t.Fatalf("expected secret %s, got %s", encSecret, secret)
	}

	// 3. Set Recovery Codes
	encCodes := "encrypted-recovery-codes-json"
	if err := repo.SetRecoveryCodes(ctx, userID, encCodes); err != nil {
		t.Fatalf("SetRecoveryCodes failed: %v", err)
	}

	// 4. Enable MFA
	if err := repo.UpdateMFA(ctx, userID, "", true); err != nil {
		t.Fatalf("UpdateMFA(enable) failed: %v", err)
	}

	// 5. Verify fields via GetByID and GetByEmail
	usrAfter, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID after enable failed: %v", err)
	}
	if !usrAfter.MFAEnabled {
		t.Fatal("expected MFAEnabled to be true")
	}
	if usrAfter.MFASecret != encSecret {
		t.Fatalf("expected secret %s, got %s", encSecret, usrAfter.MFASecret)
	}
	if usrAfter.MFARecoveryCodes != encCodes {
		t.Fatalf("expected recovery codes %s, got %s", encCodes, usrAfter.MFARecoveryCodes)
	}
	if usrAfter.MFAVerifiedAt == nil {
		t.Fatal("expected MFAVerifiedAt to be set")
	}

	usrByEmail, err := repo.GetByEmail(ctx, usrAfter.Email)
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if !usrByEmail.MFAEnabled || usrByEmail.MFASecret != encSecret {
		t.Fatalf("unexpected user by email: %+v", usrByEmail)
	}

	// 6. Disable MFA
	if err := repo.UpdateMFA(ctx, userID, "", false); err != nil {
		t.Fatalf("UpdateMFA(disable) failed: %v", err)
	}

	usrDisabled, err := repo.GetByID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByID after disable failed: %v", err)
	}
	if usrDisabled.MFAEnabled {
		t.Fatal("expected MFAEnabled to be false after disabling")
	}
	if usrDisabled.MFASecret != "" {
		t.Fatal("expected MFASecret to be cleared after disabling")
	}
	if usrDisabled.MFARecoveryCodes != "" {
		t.Fatal("expected MFARecoveryCodes to be cleared after disabling")
	}
}
