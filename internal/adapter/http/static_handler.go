// Package http provides HTTP routing, handler setup, and static asset serving.
package http

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SPAHandler serves static assets from rootDir and falls back to index.html for client routes.
type SPAHandler struct {
	rootDir   string
	indexPath string
}

// NewSPAHandler constructs an http.Handler that serves static files from rootDir
// and transparently falls back to index.html for SPA client-side deep routes
// (e.g., /incidents/*, /fleet/*, /monitoring/*, /settings/*).
// API and health endpoints (/api/*, /healthz, /readyz, /livez, /metrics, /ws) never fall back.
func NewSPAHandler(rootDir string) http.Handler {
	cleanRoot := filepath.Clean(rootDir)
	return &SPAHandler{
		rootDir:   cleanRoot,
		indexPath: filepath.Join(cleanRoot, "index.html"),
	}
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only GET and HEAD methods can read static files or SPA views
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Never return HTML fallback for API or system probe endpoints
	if isNonSPAPath(r.URL.Path) {
		http.NotFound(w, r)
		return
	}

	// Prevent directory traversal attacks outside rootDir
	cleanURLPath := path.Clean(r.URL.Path)
	relPath := strings.TrimPrefix(cleanURLPath, "/")
	targetPath := filepath.Join(h.rootDir, filepath.FromSlash(relPath))

	rel, err := filepath.Rel(h.rootDir, targetPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 1. If target file exists and is regular file, serve it directly
	info, err := os.Stat(targetPath)
	if err == nil && !info.IsDir() {
		if strings.HasPrefix(cleanURLPath, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFile(w, r, targetPath)
		return
	}

	// 2. If target is directory, check for direct index.html inside it
	if err == nil && info.IsDir() {
		subIndex := filepath.Join(targetPath, "index.html")
		if subInfo, subErr := os.Stat(subIndex); subErr == nil && !subInfo.IsDir() {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, subIndex)
			return
		}
	}

	// 3. Fallback to root index.html with HTTP 200 OK for SPA history client routes
	indexInfo, indexErr := os.Stat(h.indexPath)
	if indexErr != nil || indexInfo.IsDir() {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	http.ServeFile(w, r, h.indexPath)
}

// isNonSPAPath identifies API and telemetry endpoints that must return 404 when not found.
func isNonSPAPath(p string) bool {
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		return true
	}
	if p == "/healthz" || strings.HasPrefix(p, "/healthz/") {
		return true
	}
	if p == "/readyz" || strings.HasPrefix(p, "/readyz/") {
		return true
	}
	if p == "/livez" || strings.HasPrefix(p, "/livez/") {
		return true
	}
	if p == "/metrics" || strings.HasPrefix(p, "/metrics/") {
		return true
	}
	if p == "/ws" || strings.HasPrefix(p, "/ws/") {
		return true
	}
	return false
}
