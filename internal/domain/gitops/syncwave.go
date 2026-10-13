// Package gitops provides domain aggregates, resilience contracts, and DAG analysis for GitOps workflows.
//
// --------------------------------------------------------------------------------------------------
// PHYSICAL HARDWARE BOTTLENECK & PRODUCTION RELIABILITY ENGINEERING NOTE:
// 1. Git Packfile I/O Deserialization Memory Overhead:
//    In high-volume monorepos with deep commit histories and multi-gigabyte packfiles (.pack/.idx),
//    executing commit ancestor resolution (git merge-base) triggers Git delta chain decompression.
//    Under concurrent webhook bursts across multi-core nodes, unconstrained git worker processes
//    can consume gigabytes of RSS memory within milliseconds, causing the Linux OOM killer to terminate
//    critical platform controller pods.
//
// 2. Graph Traversal Recursion Depth Limits on Large Release Charts:
//    Enterprise umbrella Helm charts frequently contain hundreds of microservices, CRDs, and configurations.
//    Unbounded recursive DAG traversals during sync-wave cycle audits can exceed thread stack limits
//    and trigger runtime panics. Algorithms must enforce explicit depth ceilings or iterative traversal.
//
// 3. etcd 3-Way Merge Conflict CPU Overhead During Massive managedFields Reconciliation:
//    Kubernetes Server-Side Apply tracks granular field ownership in `metadata.managedFields`. Large
//    reconciliation loops parsing deeply nested managedFields JSON structures generate intense garbage
//    collection pressure and high CPU serialization overhead on the etcd raft leader.
//
// 4. Real-World Failure Modes Defended:
//    - Circular Dependency Deadlocks: Inverted sync-wave dependencies or mutating webhooks awaiting pods
//      in earlier waves cause ArgoCD and Flux reconciliation engines to lock indefinitely in `Progressing`.
//    - Out-of-Order Webhook Delivery: Asynchronous HTTP deliveries can arrive out of order, causing a delayed
//      older webhook to check out an obsolete commit and silently overwrite a critical production hotfix.
//    - Out-of-Band kubectl edit Corruption: Manual modifications generate conflicting field ownership in
//      managedFields, permanently blocking automated GitOps sync loops with HTTP 409 Conflict.
//
// 5. CPU Cache Coherency (False Sharing):
//    High-frequency webhook counters and latency metrics residing on the same 64-byte L1/L2 cache line
//    cause severe cache line bouncing between CPU cores under MESI/MOESI cache coherence protocols.
//    Padded counters with 64-byte alignment eliminate this performance degradation.
// --------------------------------------------------------------------------------------------------
package gitops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Standard RFC 7807 Error Codes for GitOps Resilience.
const (
	CodeGitOpsObsoleteWebhook = "ERR_GITOPS_OBSOLETE_WEBHOOK"
	CodeGitOpsCircularSyncWave = "ERR_GITOPS_CIRCULAR_SYNCWAVE"
	CodeGitOpsDriftConflict   = "ERR_GITOPS_DRIFT_CONFLICT"
)

// RFC7807ProblemDetail defines an RFC 7807 compliant problem details object for domain errors.
type RFC7807ProblemDetail struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Error satisfies the Go error interface.
func (p *RFC7807ProblemDetail) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status %d)", p.Code, p.Title, p.Detail, p.Status)
}

// Is enables error equivalence checking via errors.Is.
func (p *RFC7807ProblemDetail) Is(target error) bool {
	var t *RFC7807ProblemDetail
	if errors.As(target, &t) {
		return p.Code == t.Code
	}
	return false
}

// WithDetail returns a copy of the problem details with an enriched detail message.
func (p *RFC7807ProblemDetail) WithDetail(detail string) *RFC7807ProblemDetail {
	clone := *p
	clone.Detail = detail
	return &clone
}

// STANDARD DOMAIN ERRORS
var (
	// ErrGitOpsObsoleteWebhook is returned when an incoming commit is an ancestor of the running commit.
	ErrGitOpsObsoleteWebhook = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_GITOPS_OBSOLETE_WEBHOOK",
		Title:  "Obsolete GitOps Webhook Delivery",
		Status: http.StatusConflict, // 409
		Detail: "Incoming commit is an ancestor of current running commit; rejected to prevent out-of-order rollback over hotfixes",
		Code:   CodeGitOpsObsoleteWebhook,
	}

	// ErrGitOpsCircularSyncWave is returned when circular dependencies are detected in the sync-wave DAG.
	ErrGitOpsCircularSyncWave = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_GITOPS_CIRCULAR_SYNCWAVE",
		Title:  "Circular Sync-Wave Dependency Detected",
		Status: http.StatusUnprocessableEntity, // 422
		Detail: "Circular dependency detected in manifest sync-wave DAG; rejected to prevent ArgoCD/Flux Progressing lockup",
		Code:   CodeGitOpsCircularSyncWave,
	}

	// ErrGitOpsDriftConflict is returned when manual kubectl edit changes conflict with new chart manifests.
	ErrGitOpsDriftConflict = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/errors/ERR_GITOPS_DRIFT_CONFLICT",
		Title:  "Three-Way Merge Drift Conflict",
		Status: http.StatusConflict, // 409
		Detail: "ManagedFields conflict detected between out-of-band kubectl edit mutations and new chart manifest",
		Code:   CodeGitOpsDriftConflict,
	}
)

// PaddedCounter is a 64-bit integer padded to exactly 64 bytes (1 CPU cache line)
// to prevent false sharing across SMP cores under the MESI/MOESI cache coherence protocols.
type PaddedCounter struct {
	val int64
	_   [56]byte
}

// Inc atomically increments the counter by 1.
func (c *PaddedCounter) Inc() int64 {
	return atomic.AddInt64(&c.val, 1)
}

// Add atomically adds delta to the counter.
func (c *PaddedCounter) Add(delta int64) int64 {
	return atomic.AddInt64(&c.val, delta)
}

// Load atomically retrieves the current counter value.
func (c *PaddedCounter) Load() int64 {
	return atomic.LoadInt64(&c.val)
}

// Store atomically sets the counter value.
func (c *PaddedCounter) Store(val int64) {
	atomic.StoreInt64(&c.val, val)
}

// SyncWaveMetrics provides cache-line padded metrics for GitOps operations.
type SyncWaveMetrics struct {
	WebhookProcessedCount       *PaddedCounter
	WebhookDroppedObsoleteCount *PaddedCounter
	DAGTraversalNanosTotal      *PaddedCounter
	DriftEvaluationsCount       *PaddedCounter
	DriftConflictsCount         *PaddedCounter
	DriftAutoResolvedCount      *PaddedCounter
}

// NewSyncWaveMetrics allocates an initialized SyncWaveMetrics with isolated cache lines.
func NewSyncWaveMetrics() *SyncWaveMetrics {
	return &SyncWaveMetrics{
		WebhookProcessedCount:       &PaddedCounter{},
		WebhookDroppedObsoleteCount: &PaddedCounter{},
		DAGTraversalNanosTotal:      &PaddedCounter{},
		DriftEvaluationsCount:       &PaddedCounter{},
		DriftConflictsCount:         &PaddedCounter{},
		DriftAutoResolvedCount:      &PaddedCounter{},
	}
}

// WebhookEventPayload encapsulates an incoming webhook event from a Git repository.
type WebhookEventPayload struct {
	DeliveryID      string    `json:"delivery_id"`
	RepositoryURL   string    `json:"repository_url"`
	Branch          string    `json:"branch"`
	CommitSHA       string    `json:"commit_sha"`
	CommitTimestamp time.Time `json:"commit_timestamp"`
	Author          string    `json:"author"`
}

// Validate verifies mandatory fields in WebhookEventPayload.
func (p *WebhookEventPayload) Validate() error {
	if p.DeliveryID == "" {
		return errors.New("delivery_id cannot be empty")
	}
	if p.CommitSHA == "" {
		return errors.New("commit_sha cannot be empty")
	}
	if p.Branch == "" {
		return errors.New("branch cannot be empty")
	}
	return nil
}

// SyncWaveGraphNode represents a Kubernetes resource vertex within the GitOps sync-wave DAG.
type SyncWaveGraphNode struct {
	ResourceName string   `json:"resource_name"`
	Kind         string   `json:"kind"`
	Wave         int      `json:"wave"`
	DependsOn    []string `json:"depends_on"`
}

// DriftReconciliationResult encapsulates the evaluation of 3-way merge managedFields drift.
type DriftReconciliationResult struct {
	HasDrift         bool     `json:"has_drift"`
	DriftedFields    []string `json:"drifted_fields"`
	ConflictDetected bool     `json:"conflict_detected"`
	AutoResolved     bool     `json:"auto_resolved"`
}

// GitOpsResilienceEngine defines the domain port for GitOps resilience operations.
type GitOpsResilienceEngine interface {
	// VerifyCommitMonotonicOrdering ensures an incoming commit is not an ancestor of current running commit.
	VerifyCommitMonotonicOrdering(ctx context.Context, repoPath string, incomingSHA, currentSHA string) (isNewer bool, err error)

	// AuditSyncWaveCycle constructs a DAG of manifests and verifies no cyclic dependencies exist.
	AuditSyncWaveCycle(ctx context.Context, manifests [][]byte) (hasCycle bool, cycleDetails string, err error)

	// DetectAndReconcileDrift audits managedFields for out-of-band drift against desired chart manifests.
	DetectAndReconcileDrift(ctx context.Context, releaseName, namespace string, newChartManifest []byte) (*DriftReconciliationResult, error)
}
