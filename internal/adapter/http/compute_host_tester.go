package http

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

func measureLatency(start time.Time) int64 {
	latencyMs := time.Since(start).Milliseconds()
	if latencyMs <= 0 {
		return 1
	}
	return latencyMs
}

// TestHost handles POST /api/v1/docker/hosts/{id}/test
func (h *DockerHandler) TestHost(w http.ResponseWriter, r *http.Request) {
	h.TestHostConnectivity(w, r)
}

// TestHostConnectivity tests connectivity to a compute host based on its host type (agent, docker, k8s).
func (h *DockerHandler) TestHostConnectivity(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing compute host id", nil)
		return
	}

	host, err := h.hostRepo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get compute host", err)
		return
	}
	if host == nil {
		writeError(w, http.StatusNotFound, "compute host not found", nil)
		return
	}

	now := time.Now().UTC()
	var (
		latencyMs int64
		testErr   error
		msg       string
		agentInfo map[string]interface{}
	)

	switch host.HostType {
	case "agent":
		latencyMs, agentInfo, testErr = testAgentHostConnection(r.Context(), host)
		msg = "successfully connected to agent host"
	case "docker":
		latencyMs, testErr = testDockerHostConnection(r.Context(), host)
		msg = "successfully connected to docker host"
	case "k8s":
		latencyMs, testErr = testK8sHostConnection(r.Context(), host)
		msg = "successfully connected to kubernetes api"
	case "prometheus":
		latencyMs, testErr = testPrometheusHostConnection(r.Context(), host)
		msg = "successfully connected to prometheus endpoint"
	case "git":
		latencyMs, testErr = testGitHostConnection(r.Context(), host)
		msg = "successfully connected to git repository endpoint"
	case "database":
		latencyMs, testErr = testDatabaseHostConnection(r.Context(), host)
		msg = "successfully connected to database endpoint"
	default:
		latencyMs, testErr = testCustomHostConnection(r.Context(), host)
		msg = "successfully connected to endpoint"
	}

	if testErr != nil {
		if err := h.hostRepo.UpdateStatus(r.Context(), host.ID, "error", now); err != nil {
			log.Printf("failed to update host status to error: %v", err)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "error", "message": testErr.Error(), "latency_ms": 0})
		return
	}

	if err := h.hostRepo.UpdateStatus(r.Context(), host.ID, "connected", now); err != nil {
		log.Printf("failed to update host status to connected: %v", err)
	}
	resp := map[string]interface{}{"status": "connected", "message": msg, "latency_ms": latencyMs}
	if len(agentInfo) > 0 {
		resp["agent_info"] = agentInfo
	}
	writeJSON(w, http.StatusOK, resp)
}

// resolveHostAgentToken resolves the Bearer token for agent connection testing.
// Precedence: host.Labels["auth_token"] > host.Labels["token"] > AGENT_AUTH_TOKEN env.
func resolveHostAgentToken(host *docker.ComputeHost) string {
	if host != nil && host.Labels != nil {
		if token, ok := host.Labels["auth_token"]; ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
		if token, ok := host.Labels["token"]; ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
	}
	return os.Getenv("AGENT_AUTH_TOKEN")
}
