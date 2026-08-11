package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/user"
)

type mockUserRepo struct {
	users map[string]*user.User
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func TestAuthUsecase_Authenticate(t *testing.T) {
	pass := "secret123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)

	repo := &mockUserRepo{
		users: map[string]*user.User{
			"admin@example.com": {
				ID:           "u-1",
				Email:        "admin@example.com",
				PasswordHash: string(hash),
				PlatformRole: "platform_admin",
				TenantID:     "tenant-1",
				TenantRole:   "admin",
			},
		},
	}

	uc := NewUsecase(repo)

	// Valid login
	res, err := uc.Authenticate(context.Background(), "admin@example.com", pass)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.UserID != "u-1" || res.Role != "platform_admin" || res.TenantID != "tenant-1" {
		t.Errorf("unexpected AuthResult: %+v", res)
	}

	// Invalid password
	_, err = uc.Authenticate(context.Background(), "admin@example.com", "wrongpass")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Non-existent user
	_, err = uc.Authenticate(context.Background(), "missing@example.com", pass)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
