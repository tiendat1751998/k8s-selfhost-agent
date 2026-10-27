package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

var knownHostDaemons = map[string]bool{
	"k8s-agent":      true,
	"containerd":     true,
	"dockerd":        true,
	"docker":         true,
	"kubelet":        true,
	"kube-scheduler": true,
	"postgres":       true,
	"postgresql":     true,
	"redis":          true,
	"redis-server":   true,
	"proxysql":       true,
}

// isHostDaemonTarget checks whether the request targets a systemd host daemon.
func isHostDaemonTarget(targetType, name string) bool {
	tLower := strings.ToLower(strings.TrimSpace(targetType))
	if tLower == "daemon" || tLower == "host-daemon" {
		return true
	}
	if tLower == "" {
		nLower := strings.ToLower(strings.TrimSpace(name))
		return knownHostDaemons[nLower]
	}
	return false
}

// scaleHostDaemon rejects scaling requests for host daemons with HTTP 422.
func (r *deploymentRepo) scaleHostDaemon(_ context.Context, _, _, _ string, _ int) error {
	return domainerrors.NewUnprocessable("systemd host daemons cannot be scaled or deleted from Kubernetes/Docker deployment API; manage unit via host systemd")
}

// deleteHostDaemon rejects deletion requests for host daemons with HTTP 422.
func (r *deploymentRepo) deleteHostDaemon(_ context.Context, _, _, _ string) error {
	return domainerrors.NewUnprocessable("systemd host daemons cannot be scaled or deleted from Kubernetes/Docker deployment API; manage unit via host systemd")
}

// restartHostDaemon dispatches service restart to the node agent via HTTP POST :9100/remediate.
// Protects critical control plane services (k8s-agent self-termination, postgres cluster DB).
func (r *deploymentRepo) restartHostDaemon(ctx context.Context, targetCluster, namespace, name string) error {
	canonical := strings.ToLower(strings.TrimSpace(name))

	if canonical == "k8s-agent" {
		return domainerrors.NewUnprocessable("k8s-agent cannot be restarted via HTTP RPC to avoid self-termination; manage unit via systemctl --user restart k8s-agent")
	}
	if canonical == "postgres" || canonical == "postgresql" {
		return domainerrors.NewUnprocessable("postgresql is a protected control-plane database service; direct restart via deployment API is prevented to protect cluster state")
	}

	targetService := canonical
	switch canonical {
	case "dockerd":
		targetService = "docker"
	case "redis":
		targetService = "redis-server"
	}

	host, err := r.resolveHostForDaemon(ctx, targetCluster, namespace)
	if err != nil {
		return domainerrors.NewUnprocessable(err.Error())
	}

	baseURL := formatHostBaseURL(host.Endpoint)
	token := resolveComputeHostToken(host)

	payload := map[string]string{
		"action": "restart_service",
		"target": targetService,
		"reason": "Restart triggered via deployment API",
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return domainerrors.NewInternal("failed to marshal remediation payload", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/remediate", bytes.NewReader(bodyBytes))
	if err != nil {
		return domainerrors.NewInternal("failed to build remediation request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return domainerrors.NewUnprocessable(fmt.Sprintf("failed to reach node agent at %s: %v", baseURL, err))
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return domainerrors.NewUnprocessable(fmt.Sprintf("agent remediation error (HTTP %d): %s", resp.StatusCode, msg))
	}

	var result struct {
		Success bool   `json:"success"`
		Details string `json:"details"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil {
		if !result.Success {
			return domainerrors.NewUnprocessable(fmt.Sprintf("daemon restart failed: %s", result.Details))
		}
	}

	return nil
}

func (r *deploymentRepo) resolveHostForDaemon(ctx context.Context, targetCluster, namespace string) (domainDocker.ComputeHost, error) {
	if r.computeHostRepo == nil {
		return domainDocker.ComputeHost{}, fmt.Errorf("compute host repository unconfigured")
	}

	hosts, err := r.computeHostRepo.ListAll(ctx)
	if err != nil {
		return domainDocker.ComputeHost{}, fmt.Errorf("listing compute hosts: %w", err)
	}
	if len(hosts) == 0 {
		return domainDocker.ComputeHost{}, fmt.Errorf("no compute hosts registered")
	}

	ns := strings.TrimSpace(namespace)
	cluster := strings.TrimSpace(targetCluster)

	for _, h := range hosts {
		if ns != "" && (strings.EqualFold(h.Name, ns) || strings.EqualFold(h.ID, ns) || strings.Contains(h.Endpoint, ns)) {
			return h, nil
		}
		if cluster != "" && (strings.EqualFold(h.Name, cluster) || strings.EqualFold(h.ID, cluster) || strings.Contains(h.Endpoint, cluster)) {
			return h, nil
		}
	}

	if len(hosts) == 1 {
		return hosts[0], nil
	}

	return domainDocker.ComputeHost{}, fmt.Errorf("no compute host found matching namespace %q or cluster %q", ns, cluster)
}
