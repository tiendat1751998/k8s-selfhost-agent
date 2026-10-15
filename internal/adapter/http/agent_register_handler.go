package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// RegisterAgentRequest holds the auto-registration payload from k8s-agent.
type RegisterAgentRequest struct {
	Hostname         string            `json:"hostname"`
	IPAddress        string            `json:"ip_address"`
	Port             int               `json:"port"`
	OS               string            `json:"os"`
	Arch             string            `json:"arch"`
	KernelVersion    string            `json:"kernel_version"`
	Role             string            `json:"role"`
	CPUCores         int               `json:"cpu_cores"`
	MemoryTotalBytes int64             `json:"memory_total_bytes"`
	AuthToken        string            `json:"auth_token,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
}

// Validate ensures required fields are provided.
func (r *RegisterAgentRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Hostname) == "" {
		ve.Add("hostname", "hostname is required")
	}
	if strings.TrimSpace(r.IPAddress) == "" {
		ve.Add("ip_address", "ip_address is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// RegisterAgent handles POST /api/v1/agents/register auto-registration & heartbeats.
func (h *DockerHandler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	if h.hostRepo == nil {
		writeError(w, http.StatusServiceUnavailable, "compute host service unavailable", nil)
		return
	}

	req, ok := decodeJSON[RegisterAgentRequest](w, r)
	if !ok {
		return
	}

	port := req.Port
	if port <= 0 {
		port = 9100
	}
	endpoint := fmt.Sprintf("http://%s:%d", strings.TrimSpace(req.IPAddress), port)
	hostname := strings.TrimSpace(req.Hostname)

	tenantID := tenancy.TenantIDFromContext(r.Context())
	if tenantID == "" {
		tenantID = "default-tenant"
	}

	now := time.Now().UTC()
	allHosts, err := h.hostRepo.ListAll(r.Context())
	if err != nil {
		allHosts, _ = h.hostRepo.List(r.Context(), tenantID)
	}

	var existing *docker.ComputeHost
	for i := range allHosts {
		if strings.EqualFold(allHosts[i].Name, hostname) || strings.EqualFold(allHosts[i].Endpoint, endpoint) {
			existing = &allHosts[i]
			break
		}
	}

	labels := make(map[string]string)
	if existing != nil && existing.Labels != nil {
		for k, v := range existing.Labels {
			labels[k] = v
		}
	}
	if req.Labels != nil {
		for k, v := range req.Labels {
			labels[k] = v
		}
	}
	if req.OS != "" {
		labels["os"] = req.OS
	}
	if req.Arch != "" {
		labels["arch"] = req.Arch
	}
	if req.KernelVersion != "" {
		labels["kernel_version"] = req.KernelVersion
	}
	if req.Role != "" {
		labels["role"] = req.Role
	}
	if req.CPUCores > 0 {
		labels["cpu_cores"] = strconv.Itoa(req.CPUCores)
	}
	if req.MemoryTotalBytes > 0 {
		labels["memory_total_bytes"] = strconv.FormatInt(req.MemoryTotalBytes, 10)
	}
	if req.AuthToken != "" {
		labels["auth_token"] = req.AuthToken
	}

	if existing != nil {
		existing.Status = "connected"
		existing.LastHealthCheck = &now
		existing.Endpoint = endpoint
		existing.Labels = labels
		if existing.HostType == "" {
			existing.HostType = "agent"
		}
		if err := h.hostRepo.Update(r.Context(), existing); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update registered agent", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "registered",
			"host_id": existing.ID,
			"host":    redactComputeHost(existing),
		})
		return
	}

	newHost := &docker.ComputeHost{
		ID:              uuid.NewString(),
		Name:            hostname,
		HostType:        "agent",
		Endpoint:        endpoint,
		Status:          "connected",
		LastHealthCheck: &now,
		Labels:          labels,
		TenantID:        tenantID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.hostRepo.Create(r.Context(), newHost); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register agent host", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"status":  "registered",
		"host_id": newHost.ID,
		"host":    redactComputeHost(newHost),
	})
}
