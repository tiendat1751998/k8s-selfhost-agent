package http_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

const sampleIndexHTML = `<!DOCTYPE html><html lang="en"><head><title>Console</title></head><body><div id="app">SPA_ROOT</div></body></html>`
const sampleCSS = `body { background: #000; }`
const sampleJS = `console.log("k8sselfhost");`

func setupTestStaticDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(sampleIndexHTML), 0644); err != nil {
		t.Fatalf("failed to write index.html: %v", err)
	}

	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(assetsDir, "main.css"), []byte(sampleCSS), 0644); err != nil {
		t.Fatalf("failed to write main.css: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "app.js"), []byte(sampleJS), 0644); err != nil {
		t.Fatalf("failed to write app.js: %v", err)
	}

	return dir
}

func TestSPAHandler_DirectFallback(t *testing.T) {
	staticDir := setupTestStaticDir(t)
	handler := adapthttp.NewSPAHandler(staticDir)

	deepRoutes := []string{
		"/",
		"/incidents",
		"/incidents/INC-1001",
		"/fleet",
		"/fleet/clusters/prod-cluster-1",
		"/monitoring/overview",
		"/settings/general",
		"/settings/security/mfa",
	}

	for _, route := range deepRoutes {
		t.Run("DeepRoute_"+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("expected HTTP 200 for %s, got %d", route, rr.Code)
			}
			body := rr.Body.String()
			if !strings.Contains(body, "SPA_ROOT") {
				t.Errorf("expected SPA index.html content for %s, got %q", route, body)
			}
			contentType := rr.Header().Get("Content-Type")
			if !strings.Contains(contentType, "text/html") {
				t.Errorf("expected text/html for %s, got %s", route, contentType)
			}
		})
	}
}

func TestSPAHandler_ServeStaticAsset(t *testing.T) {
	staticDir := setupTestStaticDir(t)
	handler := adapthttp.NewSPAHandler(staticDir)

	req := httptest.NewRequest(http.MethodGet, "/assets/main.css", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for asset, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "background: #000") {
		t.Errorf("unexpected body content: %s", rr.Body.String())
	}
	if cc := rr.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("expected immutable cache control for /assets/, got %s", cc)
	}
}

func TestSPAHandler_NonSPAPathsReturnNotFound(t *testing.T) {
	staticDir := setupTestStaticDir(t)
	handler := adapthttp.NewSPAHandler(staticDir)

	nonSPARoutes := []string{
		"/api",
		"/api/v1/workloads",
		"/api/v1/nonexistent",
		"/healthz",
		"/healthz/extra",
		"/readyz",
		"/livez",
		"/metrics",
		"/ws",
	}

	for _, route := range nonSPARoutes {
		t.Run("NonSPA_"+route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusNotFound {
				t.Errorf("expected HTTP 404 for non-SPA route %s, got %d", route, rr.Code)
			}
			if strings.Contains(rr.Body.String(), "SPA_ROOT") {
				t.Errorf("non-SPA route %s must never return index.html fallback", route)
			}
		})
	}
}

func TestSPAHandler_MethodNotAllowed(t *testing.T) {
	staticDir := setupTestStaticDir(t)
	handler := adapthttp.NewSPAHandler(staticDir)

	disallowedMethods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range disallowedMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/incidents/INC-101", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected HTTP 405 for %s, got %d", method, rr.Code)
			}
		})
	}
}

func TestSPAHandler_DirectoryTraversalSecurity(t *testing.T) {
	staticDir := setupTestStaticDir(t)
	handler := adapthttp.NewSPAHandler(staticDir)

	req := httptest.NewRequest(http.MethodGet, "/../../../../etc/passwd", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Clean path prevents traversal outside rootDir; response must either be 403 or fallback to index.html within root
	if strings.Contains(rr.Body.String(), "root:x:") {
		t.Fatalf("directory traversal exploit succeeded!")
	}
}

func TestRouter_SPAHistoryFallback_Integration(t *testing.T) {
	staticDir := setupTestStaticDir(t)
	t.Setenv("FRONTEND_DIR", staticDir)

	healthHandler := health.NewHandler(5 * time.Second)
	router := adapthttp.NewRouter(healthHandler)

	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	// 1. Client-side deep route must return index.html with HTTP 200
	resp, err := client.Get(ts.URL + "/incidents/INC-12345")
	if err != nil {
		t.Fatalf("GET /incidents/INC-12345 error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 for deep route, got %d", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(bodyBytes), "SPA_ROOT") {
		t.Errorf("expected SPA_ROOT in index.html, got: %s", string(bodyBytes))
	}

	// 2. Fleet route must return index.html with HTTP 200
	resp2, err := client.Get(ts.URL + "/fleet/clusters")
	if err != nil {
		t.Fatalf("GET /fleet/clusters error: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 for /fleet/clusters, got %d", resp2.StatusCode)
	}

	// 3. Static asset must return exact asset
	respAsset, err := client.Get(ts.URL + "/assets/app.js")
	if err != nil {
		t.Fatalf("GET /assets/app.js error: %v", err)
	}
	defer respAsset.Body.Close()
	if respAsset.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 for static asset, got %d", respAsset.StatusCode)
	}
	assetBytes, _ := io.ReadAll(respAsset.Body)
	if !strings.Contains(string(assetBytes), "k8sselfhost") {
		t.Errorf("expected asset body, got: %s", string(assetBytes))
	}

	// 4. System health probe must return healthz JSON, NEVER index.html
	respHealth, err := client.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error: %v", err)
	}
	defer respHealth.Body.Close()
	if respHealth.StatusCode != http.StatusOK {
		t.Fatalf("expected HTTP 200 for /healthz, got %d", respHealth.StatusCode)
	}
	healthBytes, _ := io.ReadAll(respHealth.Body)
	if strings.Contains(string(healthBytes), "SPA_ROOT") {
		t.Errorf("/healthz must return health JSON, not SPA index.html")
	}
}
