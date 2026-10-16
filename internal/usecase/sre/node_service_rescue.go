package sre

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/agent"
)

// Classification constants for node health probing.
const (
	ClassificationSoftDeathServiceFault   = "SOFT_DEATH_SERVICE_FAULT"
	ClassificationHardDeathFencingRequired = "HARD_DEATH_FENCING_REQUIRED"
	ClassificationHealthy                 = "HEALTHY"
)

// ActionTaken constants for service rescue operations.
const (
	ActionRescueRestoredWorkloadsPreserved = "SERVICES_RESTARTED_RESCUED"
	ActionRescueFailedProceedToFencing    = "RESCUE_FAILED_PROCEED_TO_FENCING"
)

// HostProber probes host OS and container runtime layer connectivity.
type HostProber interface {
	CheckHostConnectivity(ctx context.Context, node *corev1.Node) (pingOk bool, agentOk bool, err error)
	CheckKubeletHealth(ctx context.Context, node *corev1.Node) (kubeletOk bool, containerdOk bool, err error)
}

// ServiceExecutor executes service restart actions on remote cluster nodes.
type ServiceExecutor interface {
	RestartServices(ctx context.Context, node *corev1.Node, services []string) error
}

// NodeProbeReport records the results of probing host connectivity vs Kubelet/containerd health.
type NodeProbeReport struct {
	NodeName              string   `json:"node_name"`
	Classification        string   `json:"classification"`
	HostAlive             bool     `json:"host_alive"`
	PingReachable         bool     `json:"ping_reachable"`
	AgentReachable        bool     `json:"agent_reachable"`
	KubeletResponsive     bool     `json:"kubelet_responsive"`
	ContainerdResponsive   bool     `json:"containerd_responsive"`
	Details               []string `json:"details"`
}

// ServiceRescueResult encapsulates the outcome of a soft-death service rescue attempt.
type ServiceRescueResult struct {
	NodeName      string   `json:"node_name"`
	Success       bool     `json:"success"`
	CancelFencing bool     `json:"cancel_fencing"`
	ActionTaken   string   `json:"action_taken"`
	Services      []string `json:"services"`
	Message       string   `json:"message"`
	DurationMs    int64    `json:"duration_ms"`
}

// NodeServiceRescueManager manages host probing and soft-death service rescue operations.
type NodeServiceRescueManager struct {
	prober       HostProber
	executor     ServiceExecutor
	pollInterval time.Duration
	pollTimeout  time.Duration
}

// NewNodeServiceRescueManager creates a new NodeServiceRescueManager.
func NewNodeServiceRescueManager(prober HostProber, executor ServiceExecutor) *NodeServiceRescueManager {
	if prober == nil {
		prober = &defaultHostProber{}
	}
	if executor == nil {
		executor = &defaultServiceExecutor{}
	}
	return &NodeServiceRescueManager{
		prober:       prober,
		executor:     executor,
		pollInterval: 100 * time.Millisecond,
		pollTimeout:  3 * time.Second,
	}
}

// Prober returns the underlying HostProber.
func (m *NodeServiceRescueManager) Prober() HostProber { return m.prober }

// Executor returns the underlying ServiceExecutor.
func (m *NodeServiceRescueManager) Executor() ServiceExecutor { return m.executor }

// SetComputeHostRepo configures the compute host repository on prober and executor if supported.
func (m *NodeServiceRescueManager) SetComputeHostRepo(repo docker.ComputeHostRepository) {
	if p, ok := m.prober.(*defaultHostProber); ok {
		p.hostRepo = repo
	}
	if e, ok := m.executor.(*defaultServiceExecutor); ok {
		e.hostRepo = repo
	}
}

// ProbeNodeHealth probes host ping/agent connectivity vs Kubelet/containerd socket
// to distinguish SOFT_DEATH_SERVICE_FAULT vs HARD_DEATH_FENCING_REQUIRED.
func (m *NodeServiceRescueManager) ProbeNodeHealth(ctx context.Context, client kubernetes.Interface, nodeName string) (*NodeProbeReport, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("nodeName is required")
	}
	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node %s for probing: %w", nodeName, err)
	}

	pingOk, agentOk, _ := m.prober.CheckHostConnectivity(ctx, node)
	kubeletOk, containerdOk, _ := m.prober.CheckKubeletHealth(ctx, node)

	hostAlive := pingOk || agentOk
	details := make([]string, 0)

	var classification string
	if !hostAlive {
		classification = ClassificationHardDeathFencingRequired
		details = append(details, "Host ping and agent unreachable: hardware power loss or kernel freeze")
	} else if !kubeletOk || !containerdOk {
		classification = ClassificationSoftDeathServiceFault
		details = append(details, "Host OS is responsive but Kubelet or containerd socket is deadlocked")
	} else {
		classification = ClassificationHealthy
		details = append(details, "Host and container services are operating normally")
	}

	return &NodeProbeReport{
		NodeName:              nodeName,
		Classification:        classification,
		HostAlive:             hostAlive,
		PingReachable:         pingOk,
		AgentReachable:        agentOk,
		KubeletResponsive:     kubeletOk,
		ContainerdResponsive:   containerdOk,
		Details:               details,
	}, nil
}

// ExecuteServiceRescue attempts graceful service restart before destructive storage fencing.
// If service restart restores Kubelet lease to Ready, cancels fencing and preserves all workloads.
func (m *NodeServiceRescueManager) ExecuteServiceRescue(ctx context.Context, client kubernetes.Interface, nodeName string, services []string) (*ServiceRescueResult, error) {
	start := time.Now()
	if len(services) == 0 {
		services = []string{"containerd", "kubelet"}
	}

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node %s for service rescue: %w", nodeName, err)
	}

	restartErr := m.executor.RestartServices(ctx, node, services)
	if restartErr != nil {
		return &ServiceRescueResult{
			NodeName:      nodeName,
			Success:       false,
			CancelFencing: false,
			ActionTaken:   ActionRescueFailedProceedToFencing,
			Services:      services,
			Message:       fmt.Sprintf("Failed to restart services: %v", restartErr),
			DurationMs:    time.Since(start).Milliseconds(),
		}, nil
	}

	// Poll lease and node condition to verify if NodeReady returns
	deadline := time.Now().Add(m.pollTimeout)
	isRestored := false

	pollTimer := time.NewTimer(m.pollInterval)
	defer pollTimer.Stop()

	for time.Now().Before(deadline) {
		if !pollTimer.Stop() {
			select {
			case <-pollTimer.C:
			default:
			}
		}
		pollTimer.Reset(m.pollInterval)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-pollTimer.C:
		}

		updatedNode, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil {
			for _, cond := range updatedNode.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					isRestored = true
					break
				}
			}
		}

		if lease, err := client.CoordinationV1().Leases("kube-node-lease").Get(ctx, nodeName, metav1.GetOptions{}); err == nil && lease != nil {
			if lease.Spec.RenewTime != nil && time.Since(lease.Spec.RenewTime.Time) < 1*time.Minute {
				isRestored = true
			}
		}

		if isRestored {
			break
		}
	}

	if isRestored {
		return &ServiceRescueResult{
			NodeName:      nodeName,
			Success:       true,
			CancelFencing: true,
			ActionTaken:   ActionRescueRestoredWorkloadsPreserved,
			Services:      services,
			Message:       "Service restart successfully revived Kubelet lease; workloads preserved with 0 downtime",
			DurationMs:    time.Since(start).Milliseconds(),
		}, nil
	}

	return &ServiceRescueResult{
		NodeName:      nodeName,
		Success:       false,
		CancelFencing: false,
		ActionTaken:   ActionRescueFailedProceedToFencing,
		Services:      services,
		Message:       "Service restart did not restore NodeReady lease within timeout; proceeding to storage fencing",
		DurationMs:    time.Since(start).Milliseconds(),
	}, nil
}

// defaultHostProber checks TCP ports on node InternalIP or queries k8s-agent /k8s/health.
type defaultHostProber struct {
	hostRepo docker.ComputeHostRepository
	client   *agent.AgentRemediationClient
}

func (p *defaultHostProber) getClient() *agent.AgentRemediationClient {
	if p.client != nil {
		return p.client
	}
	return agent.NewAgentRemediationClient()
}

func getNodeIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
}

func (p *defaultHostProber) CheckHostConnectivity(ctx context.Context, node *corev1.Node) (bool, bool, error) {
	ip := getNodeIP(node)
	if ip == "" {
		return false, false, nil
	}
	// Try SSH or node_exporter port 9100 with 1s timeout
	dialer := net.Dialer{Timeout: 1 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "22"))
	if err == nil {
		_ = conn.Close()
		return true, true, nil
	}
	agentConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "9100"))
	if err == nil {
		_ = agentConn.Close()
		return true, true, nil
	}
	return false, false, nil
}

func (p *defaultHostProber) CheckKubeletHealth(ctx context.Context, node *corev1.Node) (bool, bool, error) {
	cli := p.getClient()
	target := node.Name
	if target == "" {
		target = getNodeIP(node)
	}
	if target != "" {
		if host, token, err := cli.ResolveHostForNode(ctx, p.hostRepo, target); err == nil && host != nil {
			resp, hErr := cli.GetNodeK8sHealth(ctx, host.Endpoint, token)
			if hErr == nil && resp != nil {
				leaseHealthy := true
				if resp.LastLeaseRenewUnix > 0 {
					leaseHealthy = time.Since(time.Unix(resp.LastLeaseRenewUnix, 0)) < 2*time.Minute
				}
				kubeletOk := resp.KubeletHealthy && leaseHealthy
				if kubeletOk && resp.ContainerdHealthy {
					return true, true, nil
				}
				return kubeletOk, resp.ContainerdHealthy, nil
			}
		}
	}

	ip := getNodeIP(node)
	if ip == "" {
		return false, false, nil
	}
	dialer := net.Dialer{Timeout: 1 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip, "10250"))
	if err == nil {
		_ = conn.Close()
		return true, true, nil
	}
	return false, false, nil
}

// defaultServiceExecutor handles remote service execution via k8s-agent RPC.
type defaultServiceExecutor struct {
	hostRepo docker.ComputeHostRepository
	client   *agent.AgentRemediationClient
}

func (e *defaultServiceExecutor) getClient() *agent.AgentRemediationClient {
	if e.client != nil {
		return e.client
	}
	return agent.NewAgentRemediationClient()
}

func (e *defaultServiceExecutor) RestartServices(ctx context.Context, node *corev1.Node, services []string) error {
	cli := e.getClient()
	target := node.Name
	if target == "" {
		target = getNodeIP(node)
	}
	if target == "" {
		return fmt.Errorf("direct node agent communication unavailable for services: %s", strings.Join(services, ", "))
	}

	host, token, err := cli.ResolveHostForNode(ctx, e.hostRepo, target)
	if err != nil || host == nil {
		if ip := getNodeIP(node); ip != "" && ip != target {
			host, token, err = cli.ResolveHostForNode(ctx, e.hostRepo, ip)
		}
	}
	if err != nil || host == nil {
		return fmt.Errorf("direct node agent communication unavailable for services: %s", strings.Join(services, ", "))
	}

	for _, svc := range services {
		res, err := cli.ExecuteRemediation(ctx, host.Endpoint, token, agent.RemediationRequest{
			Action: "restart_service",
			Target: svc,
			Reason: "Control Plane SRE Soft-Death Service Rescue",
		})
		if err != nil {
			return fmt.Errorf("restarting service %s on node %s (%s): %w", svc, node.Name, host.Endpoint, err)
		}
		if !res.Success {
			return fmt.Errorf("restarting service %s failed on node %s: %s", svc, node.Name, res.Details)
		}
	}
	return nil
}

// SetComputeHostRepo configures compute host repository for node remediation and service rescue.
func (c *RemediationController) SetComputeHostRepo(repo docker.ComputeHostRepository) {
	if c.rescueManager == nil {
		c.rescueManager = NewNodeServiceRescueManager(nil, nil)
	}
	c.rescueManager.SetComputeHostRepo(repo)
}

// ExecuteRescue attempts graceful service restart for a node via RemediationController.
func (c *RemediationController) ExecuteRescue(ctx context.Context, clusterID, nodeName string, services []string) (*ServiceRescueResult, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("nodeName is required")
	}
	client, err := c.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving k8s client for cluster %s: %w", clusterID, err)
	}
	if client == nil || isNilK8sClient(client) {
		return nil, fmt.Errorf("kubernetes client unavailable for cluster '%s'", clusterID)
	}
	if c.rescueManager == nil {
		c.rescueManager = NewNodeServiceRescueManager(nil, nil)
	}
	return c.rescueManager.ExecuteServiceRescue(ctx, client, nodeName, services)
}
