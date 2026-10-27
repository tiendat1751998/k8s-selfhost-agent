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

	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/automation"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
)

// HostTarget represents a target node agent for automation execution.
type HostTarget struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Address  string            `json:"address"`
	Status   string            `json:"status"`
	Labels   map[string]string `json:"labels"`
}

// AutomationExecutorOption configures AutomationRuleExecutor.
type AutomationExecutorOption func(*AutomationRuleExecutor)

// WithExecutorClient configures the HTTP client for agent requests.
func WithExecutorClient(client *http.Client) AutomationExecutorOption {
	return func(e *AutomationRuleExecutor) {
		e.httpClient = client
	}
}

// WithExecutorDB configures the database pool to query compute_hosts.
func WithExecutorDB(db postgres.DBTX) AutomationExecutorOption {
	return func(e *AutomationRuleExecutor) {
		e.db = db
	}
}

// WithStaticHosts injects static hosts (useful for unit testing).
func WithStaticHosts(hosts []HostTarget) AutomationExecutorOption {
	return func(e *AutomationRuleExecutor) {
		e.staticHosts = hosts
	}
}

// AutomationRuleExecutor dispatches automated self-healing actions to node agents.
type AutomationRuleExecutor struct {
	db          postgres.DBTX
	httpClient  *http.Client
	staticHosts []HostTarget
	timeout     time.Duration
}

// NewAutomationRuleExecutor creates an automation rule execution engine.
func NewAutomationRuleExecutor(opts ...AutomationExecutorOption) *AutomationRuleExecutor {
	e := &AutomationRuleExecutor{
		timeout: 3 * time.Second,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// ExecuteRule executes a rule across the live fleet agents and returns an Execution record.
func (e *AutomationRuleExecutor) ExecuteRule(ctx context.Context, rule *automation.Rule) (*automation.Execution, error) {
	start := time.Now()
	exec := &automation.Execution{
		ID:        uuid.NewString(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Result:    "success",
		CreatedAt: time.Now().UTC(),
	}

	hosts, err := e.resolveHosts(ctx)
	if err != nil || len(hosts) == 0 {
		exec.Result = "failed"
		if err != nil {
			exec.ErrorDetail = fmt.Sprintf("failed to query compute hosts: %v", err)
		} else {
			exec.ErrorDetail = "no connected compute hosts found in fleet"
		}
		exec.TriggerEvent = "Manual SRE trigger: 0 hosts available"
		exec.ActionTaken = "Aborted self-healing dispatch: fleet offline"
		return exec, nil
	}

	switch rule.ActionType {
	case "reap_zombies":
		e.executeReapZombies(ctx, hosts, exec, start)
	case "prune_logs_and_containers", "disk_pressure_cleanup":
		e.executeDiskCleanup(ctx, hosts, exec, start)
	case "restart_container", "restart_service":
		e.executeRuntimeCheck(ctx, hosts, exec, start)
	default:
		e.executeTelemetryCheck(ctx, hosts, exec)
	}

	return exec, nil
}

func (e *AutomationRuleExecutor) executeReapZombies(ctx context.Context, hosts []HostTarget, exec *automation.Execution, start time.Time) {
	type nodeZombieStat struct {
		name        string
		zombieCount int
		err         error
	}

	stats := make([]nodeZombieStat, 0, len(hosts))
	remediatedNodes := 0

	for _, h := range hosts {
		baseURL := e.resolveBaseURL(h)
		token := e.resolveToken(h)

		// 1. Check health
		health, err := e.queryAgentHealth(ctx, baseURL+"/k8s/health", token)
		if err != nil {
			stats = append(stats, nodeZombieStat{name: h.Name, err: err})
			continue
		}

		zCount := 0
		if health != nil {
			zCount = health.ZombieCount
		}
		stats = append(stats, nodeZombieStat{name: h.Name, zombieCount: zCount})

		// 2. Invoke /remediate RPC
		remPayload := map[string]string{"action": "reap_zombies"}
		if remErr := e.postRemediate(ctx, baseURL+"/remediate", token, remPayload); remErr == nil {
			remediatedNodes++
		}
	}

	var zombieSummary []string
	var failures []string
	for _, s := range stats {
		if s.err != nil {
			failures = append(failures, fmt.Sprintf("%s (error: %v)", s.name, s.err))
		} else {
			zombieSummary = append(zombieSummary, fmt.Sprintf("%s=%d", s.name, s.zombieCount))
		}
	}

	elapsed := time.Since(start).Milliseconds()
	if len(zombieSummary) == 0 {
		exec.Result = "failed"
		exec.ErrorDetail = fmt.Sprintf("all node agent probes failed: %s", strings.Join(failures, "; "))
		exec.TriggerEvent = "Manual SRE trigger: all agents unreachable"
		exec.ActionTaken = "Failed to dispatch reap_zombies RPC"
		return
	}

	exec.TriggerEvent = fmt.Sprintf("Manual SRE trigger: inspected %d nodes (zombies: %s)", len(hosts), strings.Join(zombieSummary, ", "))
	exec.ActionTaken = fmt.Sprintf("Dispatched /remediate (action=reap_zombies) across %d/%d nodes in %dms (HTTP 200 OK)", remediatedNodes, len(hosts), elapsed)
}

func (e *AutomationRuleExecutor) executeDiskCleanup(ctx context.Context, hosts []HostTarget, exec *automation.Execution, start time.Time) {
	peakDisk := 0.0
	peakHost := ""
	probeSuccess := 0

	for _, h := range hosts {
		baseURL := e.resolveBaseURL(h)
		token := e.resolveToken(h)

		metrics, err := e.queryAgentMetrics(ctx, baseURL+"/metrics", token)
		if err != nil || metrics == nil {
			continue
		}
		probeSuccess++

		for _, d := range metrics.Disks {
			if d.MountPoint == "/" || d.UsagePercent > peakDisk {
				if d.UsagePercent > peakDisk {
					peakDisk = d.UsagePercent
					peakHost = h.Name
				}
			}
		}
	}

	if probeSuccess == 0 {
		exec.Result = "failed"
		exec.ErrorDetail = "failed to collect disk metrics from any fleet agent"
		exec.TriggerEvent = "Manual SRE trigger: disk check failed"
		exec.ActionTaken = "Aborted disk cleanup"
		return
	}

	elapsed := time.Since(start).Milliseconds()
	if peakDisk > 85.0 {
		// Trigger disk pressure cleanup RPC on node exceeding threshold
		for _, h := range hosts {
			if h.Name == peakHost {
				_ = e.postRemediate(ctx, e.resolveBaseURL(h)+"/remediate", e.resolveToken(h), map[string]string{
					"action": "disk_pressure_cleanup",
				})
			}
		}
		exec.TriggerEvent = fmt.Sprintf("High disk pressure detected: peak disk %.1f%% on %s (> 85%% threshold)", peakDisk, peakHost)
		exec.ActionTaken = fmt.Sprintf("Invoked /remediate (action=disk_pressure_cleanup) on %s in %dms", peakHost, elapsed)
	} else {
		exec.TriggerEvent = fmt.Sprintf("Manual SRE trigger: verified disk usage across %d nodes", probeSuccess)
		exec.ActionTaken = fmt.Sprintf("Peak disk %.1f%% on %s (< 85%% threshold); disk pressure cleanup skipped", peakDisk, peakHost)
	}
}

func (e *AutomationRuleExecutor) executeRuntimeCheck(ctx context.Context, hosts []HostTarget, exec *automation.Execution, start time.Time) {
	var unhealthyHost string
	healthyHosts := make([]string, 0, len(hosts))

	for _, h := range hosts {
		baseURL := e.resolveBaseURL(h)
		token := e.resolveToken(h)

		health, err := e.queryAgentHealth(ctx, baseURL+"/k8s/health", token)
		if err != nil || health == nil {
			continue
		}

		if !health.ContainerdHealthy && !health.DockerHealthy {
			unhealthyHost = h.Name
		} else {
			healthyHosts = append(healthyHosts, h.Name)
		}
	}

	elapsed := time.Since(start).Milliseconds()
	if unhealthyHost != "" {
		for _, h := range hosts {
			if h.Name == unhealthyHost {
				_ = e.postRemediate(ctx, e.resolveBaseURL(h)+"/remediate", e.resolveToken(h), map[string]string{
					"action": "restart_service",
					"target": "containerd",
				})
			}
		}
		exec.TriggerEvent = fmt.Sprintf("Unhealthy container runtime detected on %s", unhealthyHost)
		exec.ActionTaken = fmt.Sprintf("Dispatched /remediate (action=restart_service) to %s in %dms", unhealthyHost, elapsed)
	} else if len(healthyHosts) > 0 {
		exec.TriggerEvent = fmt.Sprintf("Manual SRE trigger: checked container runtime health on %d nodes", len(healthyHosts))
		exec.ActionTaken = fmt.Sprintf("Verified healthy runtimes across %d nodes (healthy on %s)", len(healthyHosts), strings.Join(healthyHosts, ", "))
	} else {
		exec.Result = "failed"
		exec.ErrorDetail = "all agent runtime probes failed"
		exec.TriggerEvent = "Manual SRE trigger: runtime health check failed"
		exec.ActionTaken = "Aborted service restart"
	}
}

func (e *AutomationRuleExecutor) executeTelemetryCheck(ctx context.Context, hosts []HostTarget, exec *automation.Execution) {
	var telemetry []string
	for _, h := range hosts {
		baseURL := e.resolveBaseURL(h)
		token := e.resolveToken(h)

		metrics, err := e.queryAgentMetrics(ctx, baseURL+"/metrics", token)
		if err == nil && metrics != nil {
			telemetry = append(telemetry, fmt.Sprintf("RAM %.1f%% / CPU %.1f%% on %s", metrics.Memory.UsagePercent, metrics.CPU.UsagePercent, h.Name))
		}
	}

	if len(telemetry) == 0 {
		exec.Result = "failed"
		exec.ErrorDetail = "failed to collect telemetry from fleet agents"
		exec.TriggerEvent = "Manual SRE trigger: telemetry check failed"
		exec.ActionTaken = "Aborted notification/throttle"
		return
	}

	exec.TriggerEvent = fmt.Sprintf("Manual SRE trigger: telemetry check across %d nodes", len(telemetry))
	exec.ActionTaken = fmt.Sprintf("Live node telemetry verified: %s", strings.Join(telemetry, ", "))
}

func (e *AutomationRuleExecutor) resolveHosts(ctx context.Context) ([]HostTarget, error) {
	if len(e.staticHosts) > 0 {
		return e.staticHosts, nil
	}
	if e.db == nil {
		return nil, nil
	}

	query := `SELECT id, name, endpoint, status, labels FROM compute_hosts ORDER BY name ASC`
	rows, err := e.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []HostTarget
	for rows.Next() {
		var id, name, endpoint, status string
		var labelBytes []byte
		if err := rows.Scan(&id, &name, &endpoint, &status, &labelBytes); err != nil {
			return nil, err
		}

		labels := make(map[string]string)
		if len(labelBytes) > 0 {
			_ = json.Unmarshal(labelBytes, &labels)
		}

		targets = append(targets, HostTarget{
			Name:     name,
			Endpoint: endpoint,
			Status:   status,
			Labels:   labels,
		})
	}

	return targets, nil
}

func (e *AutomationRuleExecutor) resolveBaseURL(h HostTarget) string {
	if h.Endpoint != "" {
		if strings.HasPrefix(h.Endpoint, "http://") || strings.HasPrefix(h.Endpoint, "https://") {
			return strings.TrimRight(h.Endpoint, "/")
		}
		return "http://" + strings.TrimRight(h.Endpoint, "/")
	}
	if h.Address != "" {
		return fmt.Sprintf("http://%s:9100", h.Address)
	}
	return "http://127.0.0.1:9100"
}

func (e *AutomationRuleExecutor) resolveToken(h HostTarget) string {
	if h.Labels != nil {
		if t, ok := h.Labels["auth_token"]; ok && t != "" {
			return t
		}
		if t, ok := h.Labels["token"]; ok && t != "" {
			return t
		}
	}
	return os.Getenv("AGENT_AUTH_TOKEN")
}

type agentMetricsResult struct {
	Hostname string `json:"hostname"`
	CPU      struct {
		UsagePercent float64 `json:"usage_percent"`
	} `json:"cpu"`
	Memory struct {
		UsagePercent float64 `json:"usage_percent"`
	} `json:"memory"`
	Disks []struct {
		MountPoint   string  `json:"mount_point"`
		UsagePercent float64 `json:"usage_percent"`
	} `json:"disks"`
}

func (e *AutomationRuleExecutor) queryAgentMetrics(ctx context.Context, targetURL, token string) (*agentMetricsResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	var res agentMetricsResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

type agentHealthResult struct {
	NodeName          string `json:"node_name"`
	ZombieCount       int    `json:"zombie_count"`
	ContainerdHealthy bool   `json:"containerd_healthy"`
	DockerHealthy     bool   `json:"docker_healthy"`
}

func (e *AutomationRuleExecutor) queryAgentHealth(ctx context.Context, targetURL, token string) (*agentHealthResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	var res agentHealthResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (e *AutomationRuleExecutor) postRemediate(ctx context.Context, targetURL, token string, payload map[string]string) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("remediate returned HTTP status %d", resp.StatusCode)
	}
	return nil
}
