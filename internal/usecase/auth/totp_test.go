package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/user"
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
)

type mockUserRepo struct {
	users map[string]*user.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[string]*user.User),
	}
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			copied := *u
			return &copied, nil
		}
	}
	return nil, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	copied := *u
	return &copied, nil
}

func (m *mockUserRepo) UpdateMFA(ctx context.Context, userID, encryptedSecret string, enabled bool) error {
	u, ok := m.users[userID]
	if !ok {
		return nil
	}
	if enabled {
		if encryptedSecret != "" {
			u.MFASecret = encryptedSecret
		}
		u.MFAEnabled = true
		now := time.Now()
		u.MFAVerifiedAt = &now
	} else {
		if encryptedSecret != "" {
			u.MFASecret = encryptedSecret
			u.MFAEnabled = false
		} else {
			u.MFASecret = ""
			u.MFAEnabled = false
			u.MFARecoveryCodes = ""
			u.MFAVerifiedAt = nil
		}
	}
	return nil
}

func (m *mockUserRepo) SetRecoveryCodes(ctx context.Context, userID, encryptedCodes string) error {
	u, ok := m.users[userID]
	if !ok {
		return nil
	}
	u.MFARecoveryCodes = encryptedCodes
	return nil
}

func (m *mockUserRepo) GetMFASecret(ctx context.Context, userID string) (string, error) {
	u, ok := m.users[userID]
	if !ok {
		return "", nil
	}
	return u.MFASecret, nil
}

func setupTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
}

func TestSetupTOTP_GeneratesValidSecret(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	res, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	if res.Secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if res.QRCodeURI == "" {
		t.Fatal("expected non-empty QRCodeURI")
	}

	// Stored secret should be encrypted in DB
	stored := repo.users["u1"]
	if stored.MFASecret == "" {
		t.Fatal("expected MFASecret to be stored")
	}
	if stored.MFASecret == res.Secret {
		t.Fatal("expected secret to be stored encrypted, not plaintext")
	}
	if stored.MFAEnabled {
		t.Fatal("expected MFA not to be enabled before verification")
	}
}

func TestVerifyAndEnableTOTP_ValidCode(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	code, err := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generating totp code failed: %v", err)
	}

	recoveryCodes, err := uc.VerifyAndEnableTOTP(context.Background(), "u1", code)
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	if len(recoveryCodes) != 10 {
		t.Fatalf("expected 10 recovery codes, got %d", len(recoveryCodes))
	}

	stored := repo.users["u1"]
	if !stored.MFAEnabled {
		t.Fatal("expected MFA to be enabled")
	}
	if stored.MFAVerifiedAt == nil {
		t.Fatal("expected MFAVerifiedAt to be set")
	}
	if stored.MFARecoveryCodes == "" {
		t.Fatal("expected MFARecoveryCodes to be stored")
	}
}

func TestVerifyAndEnableTOTP_InvalidCode(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	_, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	_, err = uc.VerifyAndEnableTOTP(context.Background(), "u1", "000000")
	if err == nil {
		t.Fatal("expected error with invalid TOTP code, got nil")
	}

	stored := repo.users["u1"]
	if stored.MFAEnabled {
		t.Fatal("expected MFA not to be enabled after invalid code")
	}
}

func TestVerifyTOTP_ValidCode(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	code, err := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generating totp code failed: %v", err)
	}

	_, err = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	// Verify during login
	currentCode, err := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generating totp code failed: %v", err)
	}
	if err := uc.VerifyTOTP(context.Background(), "u1", currentCode); err != nil {
		t.Fatalf("VerifyTOTP failed with valid code: %v", err)
	}
}

func TestVerifyTOTP_InvalidCode_RateLimit(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, err := uc.SetupTOTP(context.Background(), "u1")
	if err != nil {
		t.Fatalf("SetupTOTP failed: %v", err)
	}

	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	_, _ = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)

	// Attempt 5 invalid codes
	for i := 0; i < 5; i++ {
		err := uc.VerifyTOTP(context.Background(), "u1", "999999")
		if err == nil {
			t.Fatalf("attempt %d: expected error for invalid code, got nil", i+1)
		}
	}

	// 6th attempt should be blocked by rate limit
	err = uc.VerifyTOTP(context.Background(), "u1", code)
	if err == nil {
		t.Fatal("expected rate limit error on 6th attempt, got nil")
	}
	if err.Error() != "too many failed MFA attempts, please try again in 15 minutes" {
		t.Fatalf("unexpected rate limit message: %v", err)
	}
}

func TestVerifyRecoveryCode_ValidAndConsumed(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, _ := uc.SetupTOTP(context.Background(), "u1")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	recoveryCodes, err := uc.VerifyAndEnableTOTP(context.Background(), "u1", code)
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	firstCode := recoveryCodes[0]
	if err := uc.VerifyRecoveryCode(context.Background(), "u1", firstCode); err != nil {
		t.Fatalf("VerifyRecoveryCode failed: %v", err)
	}
}

func TestVerifyRecoveryCode_AlreadyUsed(t *testing.T) {
	setupTestEnv(t)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:    "u1",
		Email: "user1@example.com",
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, _ := uc.SetupTOTP(context.Background(), "u1")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	recoveryCodes, err := uc.VerifyAndEnableTOTP(context.Background(), "u1", code)
	if err != nil {
		t.Fatalf("VerifyAndEnableTOTP failed: %v", err)
	}

	firstCode := recoveryCodes[0]
	if err := uc.VerifyRecoveryCode(context.Background(), "u1", firstCode); err != nil {
		t.Fatalf("first use of recovery code failed: %v", err)
	}

	// Reuse should fail
	err = uc.VerifyRecoveryCode(context.Background(), "u1", firstCode)
	if err == nil {
		t.Fatal("expected error on reusing recovery code, got nil")
	}
}

func TestDisableTOTP_RequiresPasswordAndCode(t *testing.T) {
	setupTestEnv(t)
	pwdHash, _ := bcrypt.GenerateFromPassword([]byte("secretpassword123"), bcrypt.DefaultCost)
	repo := newMockUserRepo()
	repo.users["u1"] = &user.User{
		ID:           "u1",
		Email:        "user1@example.com",
		PasswordHash: string(pwdHash),
	}

	uc := usecaseAuth.NewUsecase(repo)
	setupRes, _ := uc.SetupTOTP(context.Background(), "u1")
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	_, _ = uc.VerifyAndEnableTOTP(context.Background(), "u1", code)

	// 1. Wrong password
	currentCode, _ := totp.GenerateCode(setupRes.Secret, time.Now().UTC())
	err := uc.DisableTOTP(context.Background(), "u1", "wrongpassword", currentCode)
	if err == nil {
		t.Fatal("expected error with wrong password, got nil")
	}

	// 2. Wrong code
	err = uc.DisableTOTP(context.Background(), "u1", "secretpassword123", "000000")
	if err == nil {
		t.Fatal("expected error with wrong code, got nil")
	}

	// 3. Valid password + valid code
	err = uc.DisableTOTP(context.Background(), "u1", "secretpassword123", currentCode)
	if err != nil {
		t.Fatalf("DisableTOTP failed: %v", err)
	}

	stored := repo.users["u1"]
	if stored.MFAEnabled {
		t.Fatal("expected MFA to be disabled")
	}
	if stored.MFASecret != "" {
		t.Fatal("expected MFASecret to be cleared")
	}
	if stored.MFARecoveryCodes != "" {
		t.Fatal("expected MFARecoveryCodes to be cleared")
	}
}
