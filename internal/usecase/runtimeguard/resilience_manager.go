// Package runtimeguard implements usecase orchestration for container runtime storage
// health monitoring, safe dangling image reclamation, registry mirror rewrite, and P2P fallback.
package runtimeguard

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/runtimeguard"
)

// Inode and image validation regex patterns.
var (
	digestPattern = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)
	imagePattern  = regexp.MustCompile(`^[a-zA-Z0-9_.\-/:@]+$`)
)

// Default mirror settings.
const (
	DefaultMirrorURL       = "harbor.corp.local/proxy-cache"
	DefaultPrimaryRegistry = "docker.io"
)

// NodeStorageProbe gathers node storage and inode utilization stats.
type NodeStorageProbe interface {
	GetNodeStorageHealth(ctx context.Context) ([]domain.NodeStorageHealth, error)
}

// ContainerRuntimeOperator abstracts container image inspection and pruning (crictl).
type ContainerRuntimeOperator interface {
	ListActiveImages(ctx context.Context, node string) ([]string, error)
	ListDaemonSetImages(ctx context.Context, node string) ([]string, error)
	PruneDanglingImages(ctx context.Context, node string, protectedImages []string) (int, error)
}

// NodeTaintOperator applies Kubernetes node taints.
type NodeTaintOperator interface {
	ApplyOutOfInodesTaint(ctx context.Context, node string) error
}

// AlertDispatcher emits SRE and P1 incident alerts.
type AlertDispatcher interface {
	EmitAlert(ctx context.Context, severity, code, message, node string) error
}

// P2PImagePeerResolver resolves image chunks from Spegel or Dragonfly DHT networks.
type P2PImagePeerResolver interface {
	ResolveP2PPeer(ctx context.Context, imageURI string) (string, error)
}

// Default in-memory stubs when dependencies are not provided.
type nopStorageProbe struct{}

func (nopStorageProbe) GetNodeStorageHealth(ctx context.Context) ([]domain.NodeStorageHealth, error) {
	return []domain.NodeStorageHealth{}, nil
}

type nopRuntimeOperator struct{}

func (nopRuntimeOperator) ListActiveImages(ctx context.Context, node string) ([]string, error) {
	return nil, nil
}
func (nopRuntimeOperator) ListDaemonSetImages(ctx context.Context, node string) ([]string, error) {
	return nil, nil
}
func (nopRuntimeOperator) PruneDanglingImages(ctx context.Context, node string, protectedImages []string) (int, error) {
	return 0, nil
}

type nopNodeTainter struct{}

func (nopNodeTainter) ApplyOutOfInodesTaint(ctx context.Context, node string) error {
	return nil
}

type nopAlertDispatcher struct{}

func (nopAlertDispatcher) EmitAlert(ctx context.Context, severity, code, message, node string) error {
	return nil
}

type nopP2PResolver struct{}

func (nopP2PResolver) ResolveP2PPeer(ctx context.Context, imageURI string) (string, error) {
	return "", fmt.Errorf("p2p resolution disabled")
}

// ResilienceManager coordinates runtime storage resilience, auto-pruning, and mirror mutations.
type ResilienceManager struct {
	probe           NodeStorageProbe
	runtimeOp       ContainerRuntimeOperator
	tainter         NodeTaintOperator
	alertDispatcher AlertDispatcher
	p2pResolver     P2PImagePeerResolver
	logger          *zap.Logger

	mu           sync.RWMutex
	mirrorConfig domain.RegistryMirrorConfig
	p2pTimeout   time.Duration

	// Cache-aligned padded telemetry counters
	inodeAuditCounter domain.PaddedInodeUsageCounter
	rateLimitTracker  domain.PaddedRateLimitTracker
	metricTracker     domain.PaddedStorageMetricTracker
}

// Option configures ResilienceManager parameters.
type Option func(*ResilienceManager)

// WithStorageProbe sets the NodeStorageProbe.
func WithStorageProbe(probe NodeStorageProbe) Option {
	return func(r *ResilienceManager) {
		if probe != nil {
			r.probe = probe
		}
	}
}

// WithContainerRuntime sets the ContainerRuntimeOperator.
func WithContainerRuntime(op ContainerRuntimeOperator) Option {
	return func(r *ResilienceManager) {
		if op != nil {
			r.runtimeOp = op
		}
	}
}

// WithNodeTainter sets the NodeTaintOperator.
func WithNodeTainter(tainter NodeTaintOperator) Option {
	return func(r *ResilienceManager) {
		if tainter != nil {
			r.tainter = tainter
		}
	}
}

// WithAlertDispatcher sets the AlertDispatcher.
func WithAlertDispatcher(dispatcher AlertDispatcher) Option {
	return func(r *ResilienceManager) {
		if dispatcher != nil {
			r.alertDispatcher = dispatcher
		}
	}
}

// WithP2PResolver sets the P2PImagePeerResolver.
func WithP2PResolver(resolver P2PImagePeerResolver) Option {
	return func(r *ResilienceManager) {
		if resolver != nil {
			r.p2pResolver = resolver
		}
	}
}

// WithP2PTimeout overrides the default P2P fallback timeout.
func WithP2PTimeout(timeout time.Duration) Option {
	return func(r *ResilienceManager) {
		if timeout > 0 {
			r.p2pTimeout = timeout
		}
	}
}

// WithLogger sets the zap logger.
func WithLogger(logger *zap.Logger) Option {
	return func(r *ResilienceManager) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// NewResilienceManager constructs a new production ResilienceManager instance.
func NewResilienceManager(opts ...Option) *ResilienceManager {
	mgr := &ResilienceManager{
		probe:           nopStorageProbe{},
		runtimeOp:       nopRuntimeOperator{},
		tainter:         nopNodeTainter{},
		alertDispatcher: nopAlertDispatcher{},
		p2pResolver:     nopP2PResolver{},
		logger:          zap.NewNop(),
		p2pTimeout:      domain.DefaultP2PFallbackTimeout,
		mirrorConfig: domain.RegistryMirrorConfig{
			PrimaryRegistry: DefaultPrimaryRegistry,
			MirrorURL:       DefaultMirrorURL,
			FallbackTimeout: domain.DefaultP2PFallbackTimeout,
			RateLimitFloor:  0,
		},
	}

	for _, opt := range opts {
		opt(mgr)
	}

	return mgr
}

// AuditNodeInodes monitors containerd mountpoint inode utilization and executes protective auto-prunes and taints.
func (r *ResilienceManager) AuditNodeInodes(ctx context.Context) ([]domain.NodeStorageHealth, error) {
	r.metricTracker.Add(1)

	healths, err := r.probe.GetNodeStorageHealth(ctx)
	if err != nil {
		r.logger.Error("Failed to probe node storage health", zap.Error(err))
		return nil, fmt.Errorf("probing node storage health: %w", err)
	}

	now := time.Now().UTC()
	for i := range healths {
		r.inodeAuditCounter.Inc()
		node := &healths[i]

		// Warning threshold (75%): trigger dangling image prune
		if node.RequiresPrune() {
			r.logger.Warn("Node inode usage reached warning threshold; triggering dangling image prune",
				zap.String("node", node.NodeName),
				zap.Float64("used_inodes_pct", node.UsedInodesPct),
			)
			if pruneErr := r.TriggerDanglingPrune(ctx, node.NodeName); pruneErr != nil {
				r.logger.Error("Failed to prune dangling images on inode pressure",
					zap.String("node", node.NodeName),
					zap.Error(pruneErr),
				)
			} else {
				node.LastPrunedAt = now
			}
		}

		// Critical threshold (85%): taint node out-of-inodes:NoSchedule and emit P1 alert
		if node.RequiresTaint() {
			r.logger.Error("Node inode usage reached critical threshold; applying NoSchedule taint",
				zap.String("node", node.NodeName),
				zap.Float64("used_inodes_pct", node.UsedInodesPct),
			)
			if taintErr := r.tainter.ApplyOutOfInodesTaint(ctx, node.NodeName); taintErr != nil {
				r.logger.Error("Failed to apply out-of-inodes taint",
					zap.String("node", node.NodeName),
					zap.Error(taintErr),
				)
			}
			msg := fmt.Sprintf(
				"CRITICAL P1 ALERT: Node '%s' containerd mountpoint '%s' inode table saturated (%.1f%% >= 85%%). Tainted %s:%s.",
				node.NodeName, node.MountPoint, node.UsedInodesPct, domain.NodeTaintOutOfInodesKey, domain.NodeTaintNoScheduleEffect,
			)
			_ = r.alertDispatcher.EmitAlert(ctx, "CRITICAL", domain.CodeContainerdInodeSaturation, msg, node.NodeName)
		}
	}

	return healths, nil
}

// TriggerDanglingPrune safe-prunes dangling images without disturbing running pods or daemonset containers.
func (r *ResilienceManager) TriggerDanglingPrune(ctx context.Context, node string) error {
	r.logger.Info("Executing safe dangling image prune", zap.String("node", node))

	// Collect active images from running workloads
	activeImgs, err := r.runtimeOp.ListActiveImages(ctx, node)
	if err != nil {
		r.logger.Warn("Could not list active pod images; continuing with daemonset inspection", zap.Error(err))
	}

	// Collect daemonset images
	daemonImgs, err := r.runtimeOp.ListDaemonSetImages(ctx, node)
	if err != nil {
		r.logger.Warn("Could not list daemonset images", zap.Error(err))
	}

	// Safety guardrail: Deduplicate protected images set
	protectedMap := make(map[string]struct{}, len(activeImgs)+len(daemonImgs))
	for _, img := range activeImgs {
		if strings.TrimSpace(img) != "" {
			protectedMap[strings.TrimSpace(img)] = struct{}{}
		}
	}
	for _, img := range daemonImgs {
		if strings.TrimSpace(img) != "" {
			protectedMap[strings.TrimSpace(img)] = struct{}{}
		}
	}

	protected := make([]string, 0, len(protectedMap))
	for img := range protectedMap {
		protected = append(protected, img)
	}

	prunedCount, err := r.runtimeOp.PruneDanglingImages(ctx, node, protected)
	if err != nil {
		return fmt.Errorf("pruning dangling images on node '%s': %w", node, err)
	}

	r.logger.Info("Successfully pruned dangling images",
		zap.String("node", node),
		zap.Int("pruned_count", prunedCount),
		zap.Int("protected_count", len(protected)),
	)
	return nil
}

// ValidateImageManifest checks format and digest signatures of container image specifications.
func (r *ResilienceManager) ValidateImageManifest(imageURI string) (bool, error) {
	trimmed := strings.TrimSpace(imageURI)
	if trimmed == "" || strings.Contains(trimmed, " ") {
		return false, domain.ErrManifestValidationFailed
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return false, domain.ErrManifestValidationFailed
	}
	if !imagePattern.MatchString(trimmed) {
		return false, domain.ErrManifestValidationFailed
	}

	// Check digest component if present
	if idx := strings.Index(trimmed, "@"); idx != -1 {
		digest := trimmed[idx+1:]
		if !digestPattern.MatchString(digest) {
			return false, domain.ErrManifestValidationFailed
		}
	}

	return true, nil
}

// ConfigureRegistryMirror updates primary registry mirror routing and fallback timeout safely.
func (r *ResilienceManager) ConfigureRegistryMirror(ctx context.Context, config domain.RegistryMirrorConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validating registry mirror config: %w", err)
	}

	r.mu.Lock()
	r.mirrorConfig = config
	r.p2pTimeout = config.FallbackTimeout
	r.mu.Unlock()

	r.logger.Info("Updated registry mirror configuration",
		zap.String("primary_registry", config.PrimaryRegistry),
		zap.String("mirror_url", config.MirrorURL),
		zap.Duration("fallback_timeout", config.FallbackTimeout),
	)
	return nil
}

// MutateImageForMirror rewrites docker.io and unqualified pull specs to the configured internal mirror.
func (r *ResilienceManager) MutateImageForMirror(imageURI string) (string, error) {
	r.mu.RLock()
	cfg := r.mirrorConfig
	r.mu.RUnlock()

	// Rate limit check
	currentPulls := r.rateLimitTracker.Load()
	if cfg.RateLimitFloor > 0 && currentPulls >= uint64(cfg.RateLimitFloor) {
		return imageURI, domain.ErrRegistryRateLimitExceeded
	}
	r.rateLimitTracker.Add(1)

	trimmed := strings.TrimSpace(imageURI)
	if trimmed == "" {
		return "", domain.ErrManifestValidationFailed
	}

	// 1. Qualified docker.io or index.docker.io
	for _, prefix := range []string{"docker.io/", "index.docker.io/"} {
		if strings.HasPrefix(trimmed, prefix) {
			remainder := strings.TrimPrefix(trimmed, prefix)
			return fmt.Sprintf("%s/%s", cfg.MirrorURL, remainder), nil
		}
	}

	// 2. Already pointing to other registries (e.g. quay.io, ghcr.io, harbor...)
	slashIdx := strings.Index(trimmed, "/")
	if slashIdx != -1 {
		domainPart := trimmed[:slashIdx]
		if strings.Contains(domainPart, ".") || strings.Contains(domainPart, ":") {
			return trimmed, nil
		}
		// Unqualified with namespace (e.g. library/alpine:3.18, myorg/app:v1)
		return fmt.Sprintf("%s/%s", cfg.MirrorURL, trimmed), nil
	}

	// 3. Unqualified single component (e.g. nginx:latest, redis:alpine)
	return fmt.Sprintf("%s/library/%s", cfg.MirrorURL, trimmed), nil
}

// ResolveImageWithP2PFallback attempts fast P2P chunk resolution with unyielding fallback timeout.
func (r *ResilienceManager) ResolveImageWithP2PFallback(ctx context.Context, imageURI string) (string, error) {
	r.mu.RLock()
	timeout := r.p2pTimeout
	r.mu.RUnlock()

	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	peerURL, err := r.p2pResolver.ResolveP2PPeer(ctxTimeout, imageURI)
	if err == nil && peerURL != "" {
		return peerURL, nil
	}

	// Fallback to origin or mirror on peer failure or timeout
	r.logger.Warn("Spegel/Dragonfly P2P peer lookup timed out or failed; falling back to registry",
		zap.String("image", imageURI),
		zap.Duration("timeout", timeout),
		zap.Error(err),
	)

	return r.MutateImageForMirror(imageURI)
}

// InodeAuditCount returns total audits processed.
func (r *ResilienceManager) InodeAuditCount() uint64 {
	return r.inodeAuditCounter.Load()
}

// RateLimitTrackerCount returns total rate limit checks.
func (r *ResilienceManager) RateLimitTrackerCount() uint64 {
	return r.rateLimitTracker.Load()
}
