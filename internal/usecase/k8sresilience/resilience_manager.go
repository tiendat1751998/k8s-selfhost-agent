// Package k8sresilience implements application usecase orchestration for Kubernetes
// control plane resilience: admission webhook deadlock breaking, terminating namespace
// orphaned finalizer reaping, and Kubelet PLEG watchdogging.
package k8sresilience

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/k8sresilience"
)

// WebhookConfig defines the static configuration of an admission webhook.
type WebhookConfig struct {
	Name          string `json:"name"`
	Type          string `json:"type"`           // "mutating" or "validating"
	FailurePolicy string `json:"failure_policy"` // "Fail" or "Ignore"
	EndpointURL   string `json:"endpoint_url"`
}

// NamespaceInfo describes the state and finalizer metadata of a namespace.
type NamespaceInfo struct {
	Name                string     `json:"name"`
	Phase               string     `json:"phase"` // "Active", "Terminating"
	DeletionTimestamp   *time.Time `json:"deletion_timestamp,omitempty"`
	TerminatingDuration time.Duration `json:"terminating_duration"`
	Finalizers          []string   `json:"finalizers"`
}

// AuditEntry records a mutation action taken by the resilience engine.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"` // "WEBHOOK_BYPASS", "WEBHOOK_RESTORE", "FINALIZER_REAPED", "CONTAINERD_RESTART"
	Target    string    `json:"target"`
	Reason    string    `json:"reason"`
	Initiator string    `json:"initiator"`
}

// WebhookClient abstracts Kubernetes admission webhook queries and mutations.
type WebhookClient interface {
	GetWebhookConfigurations(ctx context.Context) ([]WebhookConfig, error)
	PatchWebhookFailurePolicy(ctx context.Context, webhookName, policy string) error
	GetWebhookPodStatus(ctx context.Context, webhookName string) (bool, error)
	ProbeWebhookEndpoint(ctx context.Context, endpointURL string) (time.Duration, error)
}

// NamespaceClient abstracts Kubernetes namespace lifecycle inspection and finalizer patching.
type NamespaceClient interface {
	GetNamespaceInfo(ctx context.Context, namespace string) (*NamespaceInfo, error)
	ListOrphanCRDFinalizers(ctx context.Context, namespace string) ([]string, error)
	PatchRemoveFinalizers(ctx context.Context, namespace string, finalizersToRemove []string) error
}

// NodeCRIClient abstracts Kubelet PLEG observation and node container runtime management.
type NodeCRIClient interface {
	GetNodePLEGMetrics(ctx context.Context, nodeName string) (time.Duration, error)
	PingContainerdSocket(ctx context.Context, nodeName string) (bool, error)
	RestartContainerd(ctx context.Context, nodeName string) error
}

// ClusterResilienceClient satisfies all subsystem clients.
type ClusterResilienceClient interface {
	WebhookClient
	NamespaceClient
	NodeCRIClient
}

// ResilienceManager orchestrates control plane self-healing and watchdog tasks.
type ResilienceManager struct {
	client ClusterResilienceClient
	logger *zap.Logger

	// Cache-line padded high-scale telemetry metrics to eliminate false sharing.
	webhookMetrics domain.PaddedWebhookMetrics
	plegMetrics    domain.PaddedPLEGMetrics

	// Concurrency-safe in-memory state tracking
	mu                  sync.RWMutex
	consecutiveFailures map[string]uint64
	bypassedWebhooks    map[string]bool
	auditTrail          []AuditEntry
}

// NewResilienceManager constructs a new ResilienceManager with safe defaults.
func NewResilienceManager(client ClusterResilienceClient, log *zap.Logger) *ResilienceManager {
	if log == nil {
		log = zap.NewNop()
	}
	return &ResilienceManager{
		client:              client,
		logger:              log,
		consecutiveFailures: make(map[string]uint64),
		bypassedWebhooks:    make(map[string]bool),
		auditTrail:          make([]AuditEntry, 0),
	}
}

// AuditWebhooks audits all mutating and validating webhook configurations in the cluster.
// Detects cold-start deadlocks and automatically restores healthy webhooks.
func (m *ResilienceManager) AuditWebhooks(ctx context.Context) ([]domain.WebhookHealthStatus, error) {
	if m.client == nil {
		return nil, domain.ErrWebhookNotFound
	}

	configs, err := m.client.GetWebhookConfigurations(ctx)
	if err != nil {
		return nil, fmt.Errorf("retrieving webhook configurations: %w", err)
	}

	atomic.AddUint64(&m.webhookMetrics.TotalAudits, 1)
	results := make([]domain.WebhookHealthStatus, 0, len(configs))

	for _, cfg := range configs {
		podsReady, _ := m.client.GetWebhookPodStatus(ctx, cfg.Name)
		latency, probeErr := m.client.ProbeWebhookEndpoint(ctx, cfg.EndpointURL)

		m.mu.Lock()
		failures := m.consecutiveFailures[cfg.Name]
		if probeErr != nil || !podsReady {
			failures++
			m.consecutiveFailures[cfg.Name] = failures
			atomic.AddUint64(&m.webhookMetrics.ConsecutiveFailures, 1)
		} else {
			m.consecutiveFailures[cfg.Name] = 0
			failures = 0
		}
		wasBypassed := m.bypassedWebhooks[cfg.Name]
		m.mu.Unlock()

		atomic.StoreUint64(&m.webhookMetrics.LastLatencyMicros, uint64(latency.Microseconds()))

		// Automatic restoration guardrail: if pods are now ready, probe succeeded, and policy was Ignore (due to bypass)
		currentPolicy := cfg.FailurePolicy
		if podsReady && probeErr == nil && wasBypassed && strings.EqualFold(cfg.FailurePolicy, "Ignore") {
			m.logger.Info("Restoring webhook failurePolicy to Fail after pod recovery", zap.String("webhook", cfg.Name))
			if err := m.client.PatchWebhookFailurePolicy(ctx, cfg.Name, "Fail"); err == nil {
				m.mu.Lock()
				m.bypassedWebhooks[cfg.Name] = false
				m.auditTrail = append(m.auditTrail, AuditEntry{
					Timestamp: time.Now().UTC(),
					Action:    "WEBHOOK_RESTORE",
					Target:    cfg.Name,
					Reason:    "Backing pods recovered and passed health probes; restoring strict admission enforcement",
					Initiator: "k8s-resilience-watchdog",
				})
				m.mu.Unlock()
				atomic.AddUint64(&m.webhookMetrics.RestoreCount, 1)
				currentPolicy = "Fail"
			}
		}

		status := domain.NewWebhookHealthStatus(
			cfg.Name,
			cfg.Type,
			currentPolicy,
			cfg.EndpointURL,
			failures,
			latency,
			podsReady,
		)
		results = append(results, status)
	}

	return results, nil
}

// EmergencyBypassDeadlockedWebhook patches a deadlocked webhook's failurePolicy to Ignore.
// Guardrail: Rejects bypass if webhook pods are healthy.
func (m *ResilienceManager) EmergencyBypassDeadlockedWebhook(ctx context.Context, webhookName string) error {
	if m.client == nil {
		return domain.ErrWebhookNotFound
	}

	configs, err := m.client.GetWebhookConfigurations(ctx)
	if err != nil {
		return fmt.Errorf("retrieving webhook configurations: %w", err)
	}

	var targetConfig *WebhookConfig
	for _, c := range configs {
		if c.Name == webhookName {
			targetConfig = &c
			break
		}
	}
	if targetConfig == nil {
		return domain.ErrWebhookNotFound
	}

	// Guardrail: Check whether pods are healthy
	podsReady, _ := m.client.GetWebhookPodStatus(ctx, webhookName)
	_, probeErr := m.client.ProbeWebhookEndpoint(ctx, targetConfig.EndpointURL)

	m.mu.RLock()
	failures := m.consecutiveFailures[webhookName]
	m.mu.RUnlock()

	if podsReady && probeErr == nil && failures == 0 {
		return fmt.Errorf("bypass rejected: webhook %s pods and endpoints are healthy", webhookName)
	}

	if err := m.client.PatchWebhookFailurePolicy(ctx, webhookName, "Ignore"); err != nil {
		return fmt.Errorf("patching webhook failurePolicy: %w", err)
	}

	m.mu.Lock()
	m.bypassedWebhooks[webhookName] = true
	m.auditTrail = append(m.auditTrail, AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    "WEBHOOK_BYPASS",
		Target:    webhookName,
		Reason:    "Admission deadlock broken: failurePolicy changed from Fail to Ignore to unblock pod scheduling",
		Initiator: "emergency-bypass-sre",
	})
	m.mu.Unlock()

	atomic.AddUint64(&m.webhookMetrics.BypassCount, 1)
	m.logger.Warn("Emergency webhook bypass executed", zap.String("webhook", webhookName))
	return nil
}

// ReapOrphanedFinalizers purges blocking orphaned CRD/APIService finalizers from a namespace
// stuck in Terminating for longer than 10 minutes (600s).
func (m *ResilienceManager) ReapOrphanedFinalizers(ctx context.Context, namespace string) error {
	if m.client == nil {
		return domain.ErrFinalizerReapFailed
	}

	nsInfo, err := m.client.GetNamespaceInfo(ctx, namespace)
	if err != nil {
		return fmt.Errorf("fetching namespace info: %w", err)
	}

	// Guardrail 1: Namespace must be in Terminating phase
	if !strings.EqualFold(nsInfo.Phase, "Terminating") {
		return fmt.Errorf("%w: namespace %s is in phase %s", domain.ErrNamespaceNotStuck, namespace, nsInfo.Phase)
	}

	// Guardrail 2: Namespace must be terminating for > 10m
	const minStuckThreshold = 10 * time.Minute
	if nsInfo.TerminatingDuration < minStuckThreshold {
		return fmt.Errorf("%w: terminating for %s (threshold is %s)", domain.ErrNamespaceNotStuck, nsInfo.TerminatingDuration, minStuckThreshold)
	}

	orphans, err := m.client.ListOrphanCRDFinalizers(ctx, namespace)
	if err != nil {
		return fmt.Errorf("listing orphaned finalizers: %w", err)
	}

	if len(orphans) == 0 {
		return nil
	}

	if err := m.client.PatchRemoveFinalizers(ctx, namespace, orphans); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrFinalizerReapFailed, err)
	}

	m.mu.Lock()
	m.auditTrail = append(m.auditTrail, AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    "FINALIZER_REAPED",
		Target:    namespace,
		Reason:    fmt.Sprintf("Removed orphaned finalizers: %s", strings.Join(orphans, ", ")),
		Initiator: "k8s-resilience-reaper",
	})
	m.mu.Unlock()

	m.logger.Info("Successfully reaped orphaned finalizers",
		zap.String("namespace", namespace),
		zap.Strings("reaped_finalizers", orphans),
	)
	return nil
}

// CheckPLEGHealth checks whether a node's Kubelet PLEG relist cycle is healthy and containerd is responsive.
func (m *ResilienceManager) CheckPLEGHealth(ctx context.Context, nodeName string) (*domain.PLEGStatus, error) {
	if m.client == nil {
		return nil, domain.ErrNodeNotFound
	}

	relistDuration, err := m.client.GetNodePLEGMetrics(ctx, nodeName)
	if err != nil {
		return nil, fmt.Errorf("getting node PLEG metrics: %w", err)
	}

	containerdResponsive, err := m.client.PingContainerdSocket(ctx, nodeName)
	if err != nil {
		containerdResponsive = false
	}

	atomic.StoreUint64(&m.plegMetrics.RelistDurationMicros, uint64(relistDuration.Microseconds()))

	status := domain.NewPLEGStatus(nodeName, relistDuration, containerdResponsive)
	if status.IsHealthy {
		atomic.AddUint64(&m.plegMetrics.HealthyChecks, 1)
	} else {
		atomic.AddUint64(&m.plegMetrics.TimeoutCount, 1)
	}

	return &status, nil
}

// RestartStalledContainerd executes a graceful containerd daemon recycle on the target node.
func (m *ResilienceManager) RestartStalledContainerd(ctx context.Context, nodeName string) error {
	if m.client == nil {
		return domain.ErrNodeNotFound
	}

	if err := m.client.RestartContainerd(ctx, nodeName); err != nil {
		return fmt.Errorf("restarting containerd on node %s: %w", nodeName, err)
	}

	m.mu.Lock()
	m.auditTrail = append(m.auditTrail, AuditEntry{
		Timestamp: time.Now().UTC(),
		Action:    "CONTAINERD_RESTART",
		Target:    nodeName,
		Reason:    "Kubelet PLEG relist exceeded 180s timeout; recycled containerd daemon to prevent node eviction storm",
		Initiator: "k8s-pleg-watchdog",
	})
	m.mu.Unlock()

	atomic.AddUint64(&m.plegMetrics.RestartCount, 1)
	m.logger.Warn("Containerd daemon recycled for stalled node", zap.String("node", nodeName))
	return nil
}

// GetAuditTrail returns an immutable copy of the resilience actions audit trail.
func (m *ResilienceManager) GetAuditTrail() []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trail := make([]AuditEntry, len(m.auditTrail))
	copy(trail, m.auditTrail)
	return trail
}

// GetMetrics returns a snapshot of the cache-line padded metrics.
func (m *ResilienceManager) GetMetrics() (domain.PaddedWebhookMetrics, domain.PaddedPLEGMetrics) {
	return domain.PaddedWebhookMetrics{
		ConsecutiveFailures: atomic.LoadUint64(&m.webhookMetrics.ConsecutiveFailures),
		TotalAudits:         atomic.LoadUint64(&m.webhookMetrics.TotalAudits),
		BypassCount:         atomic.LoadUint64(&m.webhookMetrics.BypassCount),
		RestoreCount:        atomic.LoadUint64(&m.webhookMetrics.RestoreCount),
		LastLatencyMicros:   atomic.LoadUint64(&m.webhookMetrics.LastLatencyMicros),
	}, domain.PaddedPLEGMetrics{
		RelistDurationMicros: atomic.LoadUint64(&m.plegMetrics.RelistDurationMicros),
		TimeoutCount:         atomic.LoadUint64(&m.plegMetrics.TimeoutCount),
		RestartCount:         atomic.LoadUint64(&m.plegMetrics.RestartCount),
		HealthyChecks:        atomic.LoadUint64(&m.plegMetrics.HealthyChecks),
	}
}
