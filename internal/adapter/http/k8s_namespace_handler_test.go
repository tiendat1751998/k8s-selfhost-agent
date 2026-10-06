package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestK8sNamespaceHandler_ListNamespaces_TenantFiltering(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-node-lease"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a-staging"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "tenant-b"}},
	)
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	handler := NewK8sResourceHandler(repo, nil)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/namespaces", handler.ListNamespaces)

	t.Run("tenant_admin only sees their allowed namespaces", func(t *testing.T) {
		tenantCtx := tenancy.WithUserRole(context.Background(), "tenant_admin")
		tenantCtx = tenancy.WithTenantID(tenantCtx, "tenant-a")

		req := httptest.NewRequest(http.MethodGet, "/k8s/local/namespaces", nil).WithContext(tenantCtx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []corev1.Namespace `json:"data"`
			Total int                `json:"total"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 2 {
			t.Fatalf("expected 2 namespaces, got %d", resp.Total)
		}

		names := make([]string, len(resp.Data))
		for i, ns := range resp.Data {
			names[i] = ns.Name
		}
		expected := map[string]bool{"tenant-a": true, "tenant-a-staging": true}
		for _, name := range names {
			if !expected[name] {
				t.Fatalf("unexpected namespace in list: %s", name)
			}
		}
	})

	t.Run("platform_admin sees all namespaces including system and cross-tenant", func(t *testing.T) {
		adminCtx := tenancy.WithUserRole(context.Background(), "platform_admin")
		adminCtx = tenancy.WithTenantID(adminCtx, "platform")

		req := httptest.NewRequest(http.MethodGet, "/k8s/local/namespaces", nil).WithContext(adminCtx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data  []corev1.Namespace `json:"data"`
			Total int                `json:"total"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Total != 6 {
			t.Fatalf("expected 6 namespaces for platform_admin, got %d", resp.Total)
		}
	})
}
