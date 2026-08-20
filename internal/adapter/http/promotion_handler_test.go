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

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domainPromo "github.com/datdt/k8sselfhost/internal/domain/promotion"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	promotionUsecase "github.com/datdt/k8sselfhost/internal/usecase/promotion"
)

type mockPromotionRepoForHandler struct {
	items map[string]*domainPromo.Promotion
}

func newMockPromotionRepoForHandler() *mockPromotionRepoForHandler {
	return &mockPromotionRepoForHandler{
		items: make(map[string]*domainPromo.Promotion),
	}
}

func (m *mockPromotionRepoForHandler) List(ctx context.Context, status string, limit, offset int) ([]domainPromo.Promotion, int, error) {
	var result []domainPromo.Promotion
	for _, p := range m.items {
		if status == "" || status == "all" || p.Status == status {
			result = append(result, *p)
		}
	}
	return result, len(result), nil
}

func (m *mockPromotionRepoForHandler) GetByID(ctx context.Context, id string) (*domainPromo.Promotion, error) {
	p, ok := m.items[id]
	if !ok {
		return nil, domainerrors.NewNotFound("promotion", id)
	}
	cp := *p
	return &cp, nil
}

func (m *mockPromotionRepoForHandler) Create(ctx context.Context, p *domainPromo.Promotion) error {
	p.ID = "promo-test-123"
	m.items[p.ID] = p
	return nil
}

func (m *mockPromotionRepoForHandler) Approve(ctx context.Context, id, approver string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusApproved
	p.Approver = approver
	now := time.Now().UTC()
	p.ApprovedAt = &now
	return nil
}

func (m *mockPromotionRepoForHandler) Reject(ctx context.Context, id, rejecter string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusRejected
	p.Approver = rejecter
	now := time.Now().UTC()
	p.ApprovedAt = &now
	return nil
}

func (m *mockPromotionRepoForHandler) Complete(ctx context.Context, id string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusCompleted
	now := time.Now().UTC()
	p.CompletedAt = &now
	return nil
}

func setupPromotionTestServer(repo *mockPromotionRepoForHandler) (http.Handler, *mockPromotionRepoForHandler) {
	uc := promotionUsecase.NewUsecase(repo)
	h := NewPromotionHandler(uc)

	r := chi.NewRouter()
	r.Route("/promotions", h.RegisterRoutes)
	return r, repo
}

func TestPromotionHandler_CreateAndList(t *testing.T) {
	repo := newMockPromotionRepoForHandler()
	router, _ := setupPromotionTestServer(repo)

	// 1. Create Promotion
	body, _ := json.Marshal(map[string]interface{}{
		"service":   "order-api",
		"version":   "v1.0.0",
		"from_env":  "dev",
		"to_env":    "qa",
		"requester": "developer",
	})
	req := httptest.NewRequest(http.MethodPost, "/promotions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d, body: %s", w.Code, w.Body.String())
	}

	var created domainPromo.Promotion
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if created.ID != "promo-test-123" || created.Status != "pending" {
		t.Errorf("unexpected created promotion: %+v", created)
	}

	// 2. List Promotions
	req = httptest.NewRequest(http.MethodGet, "/promotions", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}
	var listResp struct {
		Data  []domainPromo.Promotion `json:"data"`
		Total int                     `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp.Total != 1 || len(listResp.Data) != 1 {
		t.Errorf("expected 1 promotion in list, got total=%d, len=%d", listResp.Total, len(listResp.Data))
	}
}

func TestPromotionHandler_StateTransitions(t *testing.T) {
	repo := newMockPromotionRepoForHandler()
	repo.items["p1"] = &domainPromo.Promotion{
		ID:        "p1",
		Service:   "payment-svc",
		Version:   "v1.0.0",
		FromEnv:   "dev",
		ToEnv:     "qa",
		Status:    "pending",
		Requester: "alice",
	}

	router, _ := setupPromotionTestServer(repo)

	// 1. Approve
	req := httptest.NewRequest(http.MethodPut, "/promotions/p1/approve", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK approving, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Approve again should fail (400 Bad Request because of validation)
	req = httptest.NewRequest(http.MethodPut, "/promotions/p1/approve", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on re-approving, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Complete
	req = httptest.NewRequest(http.MethodPut, "/promotions/p1/complete", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK completing, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Reject completed should fail (400 Bad Request)
	req = httptest.NewRequest(http.MethodPut, "/promotions/p1/reject", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on rejecting completed promo, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPromotionHandler_ValidationFailures(t *testing.T) {
	repo := newMockPromotionRepoForHandler()
	router, _ := setupPromotionTestServer(repo)

	// Missing fields
	body, _ := json.Marshal(map[string]interface{}{
		"service": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/promotions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
	}

	// Same source and target env
	body, _ = json.Marshal(map[string]interface{}{
		"service":  "svc",
		"version":  "v1",
		"from_env": "qa",
		"to_env":   "qa",
	})
	req = httptest.NewRequest(http.MethodPost, "/promotions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for same environments, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPromotionHandler_CompleteWithUserContext(t *testing.T) {
	repo := newMockPromotionRepoForHandler()
	repo.items["p-complete-1"] = &domainPromo.Promotion{
		ID:        "p-complete-1",
		Service:   "catalog-svc",
		Version:   "v2.1.0",
		FromEnv:   "staging",
		ToEnv:     "production",
		Status:    "approved",
		Requester: "dev",
		Approver:  "lead",
	}

	router, _ := setupPromotionTestServer(repo)

	req := httptest.NewRequest(http.MethodPut, "/promotions/p-complete-1/complete", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "ops-admin")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	stored := repo.items["p-complete-1"]
	if stored.Status != domainPromo.StatusCompleted {
		t.Errorf("expected status completed, got %s", stored.Status)
	}
}

