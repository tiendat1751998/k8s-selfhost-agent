package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/cloud"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type mockCloudAccountRepo struct {
	accounts map[string]*cloud.CloudAccount
	failErr  error
}

func newMockCloudAccountRepo() *mockCloudAccountRepo {
	return &mockCloudAccountRepo{
		accounts: make(map[string]*cloud.CloudAccount),
	}
}

func (m *mockCloudAccountRepo) Create(ctx context.Context, account *cloud.CloudAccount) error {
	if m.failErr != nil {
		return m.failErr
	}
	if account.ID == "" {
		account.ID = "acc-" + uuid.NewString()[:8]
	}
	if account.Status == "" {
		account.Status = string(cloud.AccountStatusActive)
	}
	now := time.Now().UTC()
	if account.CreatedAt.IsZero() {
		account.CreatedAt = now
	}
	account.UpdatedAt = now
	m.accounts[account.ID] = account
	return nil
}

func (m *mockCloudAccountRepo) GetByID(ctx context.Context, id string) (*cloud.CloudAccount, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	acc, ok := m.accounts[id]
	if !ok {
		return nil, nil
	}
	copied := *acc
	return &copied, nil
}

func (m *mockCloudAccountRepo) List(ctx context.Context, tenantID string) ([]cloud.CloudAccount, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	list := make([]cloud.CloudAccount, 0)
	for _, acc := range m.accounts {
		if tenantID == "" || acc.TenantID == tenantID || tenantID == "default-tenant" {
			copied := *acc
			copied.EncryptedCreds = ""
			list = append(list, copied)
		}
	}
	return list, nil
}

func (m *mockCloudAccountRepo) Update(ctx context.Context, account *cloud.CloudAccount) error {
	if m.failErr != nil {
		return m.failErr
	}
	if _, ok := m.accounts[account.ID]; !ok {
		return errors.New("cloud account not found: " + account.ID)
	}
	account.UpdatedAt = time.Now().UTC()
	m.accounts[account.ID] = account
	return nil
}

func (m *mockCloudAccountRepo) Delete(ctx context.Context, id string) error {
	if m.failErr != nil {
		return m.failErr
	}
	if _, ok := m.accounts[id]; !ok {
		return errors.New("cloud account not found: " + id)
	}
	delete(m.accounts, id)
	return nil
}

func setupCloudTestRouter(repo cloud.AccountRepository, factory CloudProviderFactory) *chi.Mux {
	r := chi.NewRouter()
	handler := NewCloudHandler(repo, factory, zap.NewNop())
	r.Route("/cloud", handler.RegisterRoutes)
	return r
}

func testCreateAccountSuccess(t *testing.T) {
	repo := newMockCloudAccountRepo()
	router := setupCloudTestRouter(repo, nil)

	body := map[string]interface{}{
		"name":        "prod-aws-account",
		"provider":    "aws",
		"credentials": `{"access_key_id":"AKIA1234567890","secret_access_key":"secret123"}`,
		"region":      "us-west-2",
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/cloud/accounts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), tenancy.TenantIDKey, "tenant-test-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &rawMap); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, exists := rawMap["credentials"]; exists {
		t.Errorf("response must NEVER contain credentials, but found 'credentials' field")
	}
	if _, exists := rawMap["encrypted_creds"]; exists {
		t.Errorf("response must NEVER contain credentials, but found 'encrypted_creds' field")
	}
	if rawMap["name"] != "prod-aws-account" {
		t.Errorf("expected name 'prod-aws-account', got %v", rawMap["name"])
	}
	if rawMap["provider"] != "aws" {
		t.Errorf("expected provider 'aws', got %v", rawMap["provider"])
	}
	if rawMap["region"] != "us-west-2" {
		t.Errorf("expected region 'us-west-2', got %v", rawMap["region"])
	}
	if rawMap["id"] == "" || rawMap["id"] == nil {
		t.Errorf("expected generated account ID in response")
	}
}

func TestCloud_CreateAccount_Success(t *testing.T) { testCreateAccountSuccess(t) }
func TestCreateAccount_Success(t *testing.T)        { testCreateAccountSuccess(t) }

func testCreateAccountBadRequest(t *testing.T) {
	repo := newMockCloudAccountRepo()
	router := setupCloudTestRouter(repo, nil)

	req := httptest.NewRequest(http.MethodPost, "/cloud/accounts", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCloud_CreateAccount_BadRequest(t *testing.T) { testCreateAccountBadRequest(t) }
func TestCreateAccount_BadRequest(t *testing.T)        { testCreateAccountBadRequest(t) }

func testCreateAccountMissingFields(t *testing.T) {
	repo := newMockCloudAccountRepo()
	router := setupCloudTestRouter(repo, nil)

	body := map[string]interface{}{
		"provider":    "aws",
		"credentials": `{"access_key_id":"AKIA123"}`,
		"region":      "us-west-2",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/cloud/accounts", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request for missing name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCloud_CreateAccount_MissingFields(t *testing.T) { testCreateAccountMissingFields(t) }
func TestCreateAccount_MissingFields(t *testing.T)        { testCreateAccountMissingFields(t) }

func testListAccountsEmpty(t *testing.T) {
	repo := newMockCloudAccountRepo()
	router := setupCloudTestRouter(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/cloud/accounts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var accounts []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &accounts); err != nil {
		t.Fatalf("failed to unmarshal array response: %v, body: %s", err, rec.Body.String())
	}

	if len(accounts) != 0 {
		t.Errorf("expected empty array, got %d items", len(accounts))
	}
}

func TestCloud_ListAccounts_Empty(t *testing.T) { testListAccountsEmpty(t) }
func TestListAccounts_Empty(t *testing.T)        { testListAccountsEmpty(t) }

func testGetAccountSuccess(t *testing.T) {
	repo := newMockCloudAccountRepo()
	acc := cloud.NewCloudAccount("my-gcp-acc", cloud.ProviderGCP, "secret-gcp-creds", "us-central1", "tenant-1")
	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatalf("failed to seed account: %v", err)
	}

	router := setupCloudTestRouter(repo, nil)
	req := httptest.NewRequest(http.MethodGet, "/cloud/accounts/"+acc.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &rawMap); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, exists := rawMap["credentials"]; exists {
		t.Errorf("response must NEVER contain credentials, but found 'credentials' field")
	}
	if _, exists := rawMap["encrypted_creds"]; exists {
		t.Errorf("response must NEVER contain credentials, but found 'encrypted_creds' field")
	}
	if rawMap["id"] != acc.ID {
		t.Errorf("expected id %s, got %v", acc.ID, rawMap["id"])
	}
	if rawMap["name"] != "my-gcp-acc" {
		t.Errorf("expected name 'my-gcp-acc', got %v", rawMap["name"])
	}
}

func TestCloud_GetAccount_Success(t *testing.T) { testGetAccountSuccess(t) }
func TestGetAccount_Success(t *testing.T)        { testGetAccountSuccess(t) }

func testGetAccountNotFound(t *testing.T) {
	repo := newMockCloudAccountRepo()
	router := setupCloudTestRouter(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/cloud/accounts/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCloud_GetAccount_NotFound(t *testing.T) { testGetAccountNotFound(t) }
func TestGetAccount_NotFound(t *testing.T)        { testGetAccountNotFound(t) }

func testDeleteAccountNotFound(t *testing.T) {
	repo := newMockCloudAccountRepo()
	router := setupCloudTestRouter(repo, nil)

	req := httptest.NewRequest(http.MethodDelete, "/cloud/accounts/nonexistent-id", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCloud_DeleteAccount_NotFound(t *testing.T) { testDeleteAccountNotFound(t) }
func TestDeleteAccount_NotFound(t *testing.T)        { testDeleteAccountNotFound(t) }

func testDeleteAccountSuccess(t *testing.T) {
	repo := newMockCloudAccountRepo()
	acc := cloud.NewCloudAccount("to-delete", cloud.ProviderAzure, "secret", "eastus", "tenant-1")
	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatalf("failed to seed account: %v", err)
	}

	router := setupCloudTestRouter(repo, nil)
	req := httptest.NewRequest(http.MethodDelete, "/cloud/accounts/"+acc.ID, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204 No Content, got %d: %s", rec.Code, rec.Body.String())
	}

	fetched, _ := repo.GetByID(context.Background(), acc.ID)
	if fetched != nil {
		t.Errorf("expected account to be deleted from repo")
	}
}

func TestCloud_DeleteAccount_Success(t *testing.T) { testDeleteAccountSuccess(t) }
func TestDeleteAccount_Success(t *testing.T)        { testDeleteAccountSuccess(t) }

func testValidateAccount(t *testing.T) {
	repo := newMockCloudAccountRepo()
	acc := cloud.NewCloudAccount("valid-account", cloud.ProviderAWS, "creds", "us-east-1", "tenant-1")
	if err := repo.Create(context.Background(), acc); err != nil {
		t.Fatalf("failed to seed account: %v", err)
	}

	router := setupCloudTestRouter(repo, nil)
	req := httptest.NewRequest(http.MethodPost, "/cloud/accounts/"+acc.ID+"/validate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["status"] != "not_configured" {
		t.Errorf("expected status 'not_configured', got '%s'", resp["status"])
	}
	if resp["message"] != "Cloud provider SDK not yet configured" {
		t.Errorf("expected message 'Cloud provider SDK not yet configured', got '%s'", resp["message"])
	}
}

func TestCloud_ValidateAccount(t *testing.T) { testValidateAccount(t) }
func TestValidateAccount(t *testing.T)        { testValidateAccount(t) }
