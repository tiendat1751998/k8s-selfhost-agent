package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/compliance"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

// RemediateViolationResponse represents the wire payload for compliance violation remediation.
type RemediateViolationResponse struct {
	ViolationID string    `json:"violation_id"`
	Status      string    `json:"status"` // "remediated" | "manual_action_required"
	Automated   bool      `json:"automated"`
	Policy      string    `json:"policy"`
	Resource    string    `json:"resource"`
	Summary     string    `json:"summary"`
	Commands    []string  `json:"commands"`
	ExecutedAt  time.Time `json:"executed_at"`
	Success     bool      `json:"success"`
}

// RemediateViolation handles POST /api/v1/compliance/violations/{id}/remediate
func (h *ComplianceHandler) RemediateViolation(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "violation id is required", err)
		return
	}

	v, err := h.repo.GetViolation(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "violation not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get violation", err)
		return
	}
	if v == nil {
		writeError(w, http.StatusNotFound, "violation not found", nil)
		return
	}

	hosts := h.staticHosts
	if len(hosts) == 0 && h.scanner != nil && h.scanner.Store() != nil {
		chHosts, _ := h.scanner.Store().ListHosts(r.Context())
		for _, ch := range chHosts {
			hosts = append(hosts, HostTarget{
				Name:     ch.Name,
				Endpoint: ch.Endpoint,
				Address:  ch.Address,
				Status:   ch.Status,
				Labels:   ch.Labels,
			})
		}
	}
	if len(hosts) == 0 {
		hosts = []HostTarget{
			{Name: "k8smaster60", Endpoint: "http://10.10.10.60:9100", Address: "10.10.10.60"},
			{Name: "k8smaster133", Endpoint: "http://10.10.10.133:9100", Address: "10.10.10.133"},
			{Name: "k8sworker80", Endpoint: "http://10.10.10.80:9100", Address: "10.10.10.80"},
		}
	}

	status, automated, summary, commands := h.evaluateRemediation(r.Context(), v, hosts)

	if status == "remediated" {
		if err := h.repo.ResolveViolation(r.Context(), v.ID); err != nil {
			logger.Get().Warn("failed to update resolved flag on compliance violation", zap.Error(err), zap.String("id", v.ID))
		}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "platform_admin"
	}
	if h.scanner != nil && h.scanner.Store() != nil {
		_ = h.scanner.Store().InsertAuditLog(r.Context(), &usecaseCompliance.AuditLogRecord{
			Actor:      userID,
			Action:     "remediate",
			TargetType: "compliance_violation",
			TargetName: v.Policy,
			Result:     status,
			Details: map[string]any{
				"violation_id": v.ID,
				"policy":       v.Policy,
				"resource":     v.Resource,
				"automated":    automated,
				"summary":      summary,
			},
			IPAddress: r.RemoteAddr,
			UserAgent: r.Header.Get("User-Agent"),
			TenantID:  "default-tenant",
		})
	}

	writeJSON(w, http.StatusOK, RemediateViolationResponse{
		ViolationID: v.ID,
		Status:      status,
		Automated:   automated,
		Policy:      v.Policy,
		Resource:    v.Resource,
		Summary:     summary,
		Commands:    commands,
		ExecutedAt:  time.Now().UTC(),
		Success:     true,
	})
}

func (h *ComplianceHandler) evaluateRemediation(ctx context.Context, v *compliance.Violation, hosts []HostTarget) (string, bool, string, []string) {
	text := strings.ToLower(v.Policy + " " + v.Resource + " " + v.Message)

	// 1. Automated process table remediation
	if strings.Contains(text, "zombie") || strings.Contains(text, "process integrity") {
		successCount := h.dispatchRemediateAction(ctx, hosts, "reap_zombies")
		if successCount > 0 {
			summary := fmt.Sprintf("Automated remediation succeeded: dispatched reap_zombies RPC across %d node agents.", successCount)
			commands := []string{"curl -X POST http://<node>:9100/remediate -d '{\"action\":\"reap_zombies\"}'"}
			return "remediated", true, summary, commands
		}
	}

	// 2. Automated disk pressure cleanup
	if strings.Contains(text, "rootfs") || strings.Contains(text, "storage governance") {
		successCount := h.dispatchRemediateAction(ctx, hosts, "disk_pressure_cleanup")
		if successCount > 0 {
			summary := fmt.Sprintf("Automated remediation succeeded: dispatched disk_pressure_cleanup RPC across %d node agents.", successCount)
			commands := []string{"curl -X POST http://<node>:9100/remediate -d '{\"action\":\"disk_pressure_cleanup\"}'"}
			return "remediated", true, summary, commands
		}
	}

	// 3. Kubelet API LAN Exposure (:10250)
	if strings.Contains(text, "10250") || strings.Contains(text, "kubelet api lan") {
		summary := "Kubelet API port 10250 is exposed on external LAN interfaces. Inbound network traffic must be restricted using firewall iptables rules or binding Kubelet to localhost."
		commands := []string{
			"sudo iptables -A INPUT -p tcp --dport 10250 -s 10.10.10.0/24 -j DROP",
			"sudo iptables -A INPUT -p tcp --dport 10250 -s 127.0.0.1 -j ACCEPT",
			"sudo netfilter-persistent save",
		}
		return "manual_action_required", false, summary, commands
	}

	// 4. Redis unprotected LAN port (:6379)
	if strings.Contains(text, "6379") || strings.Contains(text, "redis") {
		summary := "Redis cache port 6379 is listening on LAN without authentication. Bind Redis strictly to 127.0.0.1 or configure requirepass in /etc/redis/redis.conf."
		commands := []string{
			"sudo sed -i 's/^bind .*/bind 127.0.0.1/' /etc/redis/redis.conf",
			"sudo systemctl restart redis-server",
		}
		return "manual_action_required", false, summary, commands
	}

	// 5. PostgreSQL TLS Enforcement (sslmode=disable / :5432)
	if strings.Contains(text, "unencrypted database") || strings.Contains(text, "sslmode=disable") || strings.Contains(text, "postgres") {
		summary := "PostgreSQL :5432 is operating without TLS encryption enforcement (sslmode=disable). Enable SSL in postgresql.conf and enforce client verification."
		commands := []string{
			"sudo sed -i 's/^ssl = off/ssl = on/' /etc/postgresql/*/main/postgresql.conf",
			"export K8S_POSTGRES_SSLMODE=verify-ca",
			"sudo systemctl restart postgresql",
		}
		return "manual_action_required", false, summary, commands
	}

	// 6. Control-plane API server :6443 unreachable
	if strings.Contains(text, "6443") || strings.Contains(text, "control plane api") {
		summary := "Kubernetes API server port 6443 is unreachable on master control plane node. Verify kubelet service and kube-apiserver static pod container logs."
		commands := []string{
			"sudo systemctl status kubelet",
			"sudo crictl ps -a | grep apiserver",
			"sudo journalctl -u kubelet -n 50 --no-pager",
		}
		return "manual_action_required", false, summary, commands
	}

	// 7. Zero-Trust Agent Auth
	if strings.Contains(text, "zero-trust agent auth") || strings.Contains(text, "unauthenticated") {
		summary := "Node agent accepts unauthenticated requests without authorization tokens. Set AGENT_AUTH_TOKEN in ~/.k8s-agent/agent.env and restart the service."
		commands := []string{
			"echo 'AGENT_AUTH_TOKEN=your-random-secret-token' >> ~/.k8s-agent/agent.env",
			"systemctl --user restart k8s-agent",
		}
		return "manual_action_required", false, summary, commands
	}

	// Default operator fallback
	summary := fmt.Sprintf("Operator manual review required for policy: %s (%s).", v.Policy, v.Message)
	commands := []string{
		"kubectl get events -A --sort-by='.lastTimestamp'",
		"sudo journalctl -xeu k8s-agent -n 50",
	}
	return "manual_action_required", false, summary, commands
}

func (h *ComplianceHandler) dispatchRemediateAction(ctx context.Context, hosts []HostTarget, action string) int {
	client := http.Client{Timeout: 2 * time.Second}
	success := 0
	for _, host := range hosts {
		baseURL := host.Endpoint
		if baseURL == "" && host.Address != "" {
			baseURL = fmt.Sprintf("http://%s:9100", host.Address)
		}
		if baseURL == "" {
			continue
		}
		token := ""
		if host.Labels != nil {
			token = host.Labels["auth_token"]
			if token == "" {
				token = host.Labels["token"]
			}
		}
		if token == "" {
			token = os.Getenv("AGENT_AUTH_TOKEN")
		}

		payload, _ := json.Marshal(map[string]string{"action": action})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/remediate", bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 400 {
				success++
			}
		}
	}
	return success
}
