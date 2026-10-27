
package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"k8s.io/client-go/tools/clientcmd"
)

// Extending FleetHandler
func (h *FleetHandler) RegisterImportRoutes(r chi.Router) {
	r.Post("/import", h.ImportCluster)
	r.Post("/validate-kubeconfig", h.ValidateKubeconfig)
	r.Get("/{id}/health", h.GetClusterHealth)
	r.Post("/{id}/discover", h.DiscoverClusterResources)
}

func (h *FleetHandler) ImportCluster(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form", err)
		return
	}

	id := r.FormValue("id")
	name := r.FormValue("name")
	group := r.FormValue("group")
	region := r.FormValue("region")
	provider := r.FormValue("provider")

	file, _, err := r.FormFile("kubeconfig")
	if err != nil {
		writeError(w, http.StatusBadRequest, "kubeconfig file is required", err)
		return
	}
	defer file.Close()

	kubeconfig, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read kubeconfig", err)
		return
	}

	c, err := h.importUsecase.ImportCluster(r.Context(), id, name, group, region, provider, kubeconfig)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to import cluster", err)
		return
	}

	if c != nil {
		c.EncryptedToken = ""
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *FleetHandler) GetClusterHealth(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cluster id is required", err)
		return
	}
	c, err := h.repo.GetCluster(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster", err)
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "cluster not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"health_status": c.HealthStatus,
		"last_health_check": c.LastHealthCheck,
	})
}

func (h *FleetHandler) DiscoverClusterResources(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "cluster id is required", err)
		return
	}
	c, err := h.repo.GetCluster(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get cluster", err)
		return
	}
	if c == nil {
		writeError(w, http.StatusNotFound, "cluster not found", nil)
		return
	}

	// Would call DiscoveryPort here, but doing simple for now
	writeJSON(w, http.StatusOK, c.DiscoveredResources)
}

type validateKubeconfigRequest struct {
	Kubeconfig string `json:"kubeconfig"`
	Server     string `json:"server"`
}

// ValidateKubeconfig handles POST /api/v1/fleet/validate-kubeconfig
func (h *FleetHandler) ValidateKubeconfig(w http.ResponseWriter, r *http.Request) {
	var rawKubeconfig []byte
	var serverURL string

	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var req validateKubeconfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			rawKubeconfig = []byte(req.Kubeconfig)
			serverURL = strings.TrimSpace(req.Server)
		}
	} else if strings.Contains(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err == nil {
			if file, _, err := r.FormFile("kubeconfig"); err == nil {
				defer file.Close()
				rawKubeconfig, _ = io.ReadAll(file)
			} else {
				rawKubeconfig = []byte(r.FormValue("kubeconfig"))
			}
			serverURL = strings.TrimSpace(r.FormValue("server"))
		}
	} else {
		rawKubeconfig, _ = io.ReadAll(r.Body)
	}

	if len(rawKubeconfig) == 0 && serverURL == "" {
		writeError(w, http.StatusBadRequest, "kubeconfig or server is required", nil)
		return
	}

	if serverURL == "" && len(rawKubeconfig) > 0 {
		apiConfig, err := clientcmd.Load(rawKubeconfig)
		if err != nil || apiConfig == nil || len(apiConfig.Clusters) == 0 {
			writeError(w, http.StatusBadRequest, "invalid kubeconfig: unable to parse cluster configuration", err)
			return
		}
		for _, cluster := range apiConfig.Clusters {
			if strings.TrimSpace(cluster.Server) != "" {
				serverURL = strings.TrimSpace(cluster.Server)
				break
			}
		}
	}

	if serverURL == "" {
		writeError(w, http.StatusBadRequest, "invalid kubeconfig: missing cluster server URL", nil)
		return
	}

	u, err := url.Parse(serverURL)
	if err != nil || u.Host == "" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid cluster server URL '%s'", serverURL), err)
		return
	}

	host := u.Hostname()
	if isBlockedKubeconfigHost(host) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("server host %s is not permitted (SSRF blocked)", host), nil)
		return
	}

	targetAddr := u.Host
	if !strings.Contains(targetAddr, ":") {
		if strings.EqualFold(u.Scheme, "http") {
			targetAddr += ":80"
		} else {
			targetAddr += ":443"
		}
	}

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	start := time.Now()
	conn, dialErr := dialer.DialContext(r.Context(), "tcp", targetAddr)
	latencyMs := time.Since(start).Milliseconds()

	reachable := dialErr == nil
	var errMsg string
	if conn != nil {
		_ = conn.Close()
	} else {
		errMsg = dialErr.Error()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":      true,
		"reachable":  reachable,
		"server":     serverURL,
		"latency_ms": latencyMs,
		"error":      errMsg,
	})
}

// isBlockedKubeconfigHost checks for prohibited cloud metadata addresses while allowing LAN and loopback test endpoints.
func isBlockedKubeconfigHost(host string) bool {
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.String() == "169.254.169.254" || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return true
		}
	}
	h := strings.ToLower(host)
	if strings.Contains(h, "169.254") || strings.Contains(h, "metadata.google.internal") {
		return true
	}
	return false
}

