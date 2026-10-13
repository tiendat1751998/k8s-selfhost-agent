// Package gitops implements application orchestration for GitOps resilience, DAG cycle audits,
// and three-way merge drift reconciliation.
package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"gopkg.in/yaml.v3"
)

const (
	// MaxDAGTraversalDepth bounds the DFS stack to protect against call stack overflow
	// on large mono-charts and deeply nested manifests.
	MaxDAGTraversalDepth = 1000

	// AnnotationSyncWave is the ArgoCD / GitOps sync wave annotation.
	AnnotationSyncWave = "argocd.argoproj.io/sync-wave"

	// AnnotationSyncWaveDependsOn specifies explicit inter-resource dependencies.
	AnnotationSyncWaveDependsOn = "argocd.argoproj.io/sync-wave-depends-on"

	// AnnotationGitOpsDependsOn is an alternative standard dependency annotation.
	AnnotationGitOpsDependsOn = "gitops.k8sselfhost.io/depends-on"
)

// GitCommander abstracts git CLI invocation for monotonic commit verification.
type GitCommander interface {
	IsAncestor(ctx context.Context, repoPath, ancestorSHA, descendantSHA string) (bool, error)
}

// DefaultGitCommander invokes the local git binary using exec.CommandContext.
type DefaultGitCommander struct{}

// IsAncestor checks if ancestorSHA is an ancestor of descendantSHA via git merge-base --is-ancestor.
func (g *DefaultGitCommander) IsAncestor(ctx context.Context, repoPath, ancestorSHA, descendantSHA string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "merge-base", "--is-ancestor", ancestorSHA, descendantSHA)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base failed: %w", err)
}

// LiveClusterStateProvider abstracts live cluster state retrieval for drift evaluation.
type LiveClusterStateProvider interface {
	GetLiveResource(ctx context.Context, namespace, kind, name string) ([]byte, error)
}

// SyncWaveManagerOption defines functional options for SyncWaveManager.
type SyncWaveManagerOption func(*SyncWaveManager)

// WithGitCommander injects a custom GitCommander.
func WithGitCommander(cmd GitCommander) SyncWaveManagerOption {
	return func(m *SyncWaveManager) {
		m.gitCmd = cmd
	}
}

// WithLiveClusterStateProvider injects a custom cluster provider.
func WithLiveClusterStateProvider(prov LiveClusterStateProvider) SyncWaveManagerOption {
	return func(m *SyncWaveManager) {
		m.clusterProvider = prov
	}
}

// SyncWaveManager coordinates GitOps DAG validation, webhook ordering guards, and drift reconciliation.
type SyncWaveManager struct {
	gitCmd          GitCommander
	clusterProvider LiveClusterStateProvider
	metrics         *domainGitops.SyncWaveMetrics
	mu              sync.RWMutex
}

// NewSyncWaveManager constructs an initialized SyncWaveManager.
func NewSyncWaveManager(opts ...SyncWaveManagerOption) *SyncWaveManager {
	m := &SyncWaveManager{
		gitCmd:  &DefaultGitCommander{},
		metrics: domainGitops.NewSyncWaveMetrics(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Metrics returns the cache-line padded operational metrics.
func (m *SyncWaveManager) Metrics() *domainGitops.SyncWaveMetrics {
	return m.metrics
}

// VerifyCommitMonotonicOrdering evaluates whether an incoming commit is strictly newer than current running commit.
func (m *SyncWaveManager) VerifyCommitMonotonicOrdering(ctx context.Context, repoPath string, incomingSHA, currentSHA string) (bool, error) {
	if incomingSHA == "" || currentSHA == "" {
		return false, errors.New("incomingSHA and currentSHA must not be empty")
	}

	if incomingSHA == currentSHA {
		m.metrics.WebhookDroppedObsoleteCount.Inc()
		return false, domainGitops.ErrGitOpsObsoleteWebhook.WithDetail(
			fmt.Sprintf("incoming commit %s matches current running commit %s; delivery dropped", incomingSHA, currentSHA),
		)
	}

	// 1. Check if incoming commit is an ancestor of the currently running commit.
	isAncestor, err := m.gitCmd.IsAncestor(ctx, repoPath, incomingSHA, currentSHA)
	if err != nil {
		return false, fmt.Errorf("ancestor evaluation failed: %w", err)
	}
	if isAncestor {
		m.metrics.WebhookDroppedObsoleteCount.Inc()
		return false, domainGitops.ErrGitOpsObsoleteWebhook.WithDetail(
			fmt.Sprintf("incoming commit %s is an ancestor of current commit %s (out-of-order delivery rejected)", incomingSHA, currentSHA),
		)
	}

	// 2. Check if current commit is an ancestor of incoming commit (strictly newer descendant).
	isDescendant, err := m.gitCmd.IsAncestor(ctx, repoPath, currentSHA, incomingSHA)
	if err != nil {
		return false, fmt.Errorf("descendant evaluation failed: %w", err)
	}
	if isDescendant {
		m.metrics.WebhookProcessedCount.Inc()
		return true, nil
	}

	// 3. Neither is an ancestor of the other (divergent branch/history).
	m.metrics.WebhookDroppedObsoleteCount.Inc()
	return false, domainGitops.ErrGitOpsObsoleteWebhook.WithDetail(
		fmt.Sprintf("incoming commit %s does not descend from current commit %s (divergent history)", incomingSHA, currentSHA),
	)
}

type manifestNode struct {
	Key       string
	Kind      string
	Name      string
	Wave      int
	DependsOn []string
}

// AuditSyncWaveCycle builds a DAG from manifest sync-wave annotations and references to detect cycles and inversions.
func (m *SyncWaveManager) AuditSyncWaveCycle(ctx context.Context, manifests [][]byte) (bool, string, error) {
	start := time.Now()
	defer func() {
		m.metrics.DAGTraversalNanosTotal.Add(time.Since(start).Nanoseconds())
	}()

	nodes := make(map[string]*manifestNode)
	for _, raw := range manifests {
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		node, err := parseManifestNode(raw)
		if err != nil {
			return false, "", fmt.Errorf("failed to parse manifest: %w", err)
		}
		nodes[node.Key] = node
	}

	// 1. Audit sync-wave inversion deadlocks (Wave A < Wave B but A depends on B).
	for _, node := range nodes {
		for _, depKey := range node.DependsOn {
			if target, ok := nodes[depKey]; ok {
				if node.Wave < target.Wave {
					details := fmt.Sprintf("sync-wave inversion deadlock: %s (wave %d) depends on %s (wave %d)",
						node.Key, node.Wave, target.Key, target.Wave)
					return true, details, domainGitops.ErrGitOpsCircularSyncWave.WithDetail(details)
				}
			}
		}
	}

	// 2. Detect cyclic dependency loops using 3-color DFS with depth guard.
	// 0: Unvisited, 1: Visiting (in call stack), 2: Visited
	visited := make(map[string]int)
	path := make([]string, 0, len(nodes))

	var dfs func(curr string, depth int) (bool, string)
	dfs = func(curr string, depth int) (bool, string) {
		if depth > MaxDAGTraversalDepth {
			return true, fmt.Sprintf("recursion depth limit exceeded (%d) at %s", MaxDAGTraversalDepth, curr)
		}
		visited[curr] = 1
		path = append(path, curr)

		node := nodes[curr]
		if node != nil {
			for _, depKey := range node.DependsOn {
				state := visited[depKey]
				if state == 1 { // cycle found
					cyclePath := formatCyclePath(path, depKey)
					return true, fmt.Sprintf("cycle detected: %s", cyclePath)
				}
				if state == 0 {
					if found, details := dfs(depKey, depth+1); found {
						return true, details
					}
				}
			}
		}

		visited[curr] = 2
		path = path[:len(path)-1]
		return false, ""
	}

	for key := range nodes {
		if visited[key] == 0 {
			if hasCycle, details := dfs(key, 0); hasCycle {
				return true, details, domainGitops.ErrGitOpsCircularSyncWave.WithDetail(details)
			}
		}
	}

	return false, "", nil
}

func parseManifestNode(raw []byte) (*manifestNode, error) {
	var parsed struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name        string            `yaml:"name"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"metadata"`
	}

	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s/%s", parsed.Kind, parsed.Metadata.Name)
	wave := 0
	if val, ok := parsed.Metadata.Annotations[AnnotationSyncWave]; ok && val != "" {
		if parsedWave, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			wave = parsedWave
		}
	}

	var dependsOn []string
	extractDeps := func(annotationKey string) {
		if val, ok := parsed.Metadata.Annotations[annotationKey]; ok && val != "" {
			parts := strings.Split(val, ",")
			for _, p := range parts {
				cleaned := strings.TrimSpace(p)
				if cleaned != "" {
					dependsOn = append(dependsOn, cleaned)
				}
			}
		}
	}
	extractDeps(AnnotationSyncWaveDependsOn)
	extractDeps(AnnotationGitOpsDependsOn)

	return &manifestNode{
		Key:       key,
		Kind:      parsed.Kind,
		Name:      parsed.Metadata.Name,
		Wave:      wave,
		DependsOn: dependsOn,
	}, nil
}

func formatCyclePath(path []string, depKey string) string {
	startIdx := -1
	for i, node := range path {
		if node == depKey {
			startIdx = i
			break
		}
	}
	if startIdx >= 0 {
		sub := append(path[startIdx:], depKey)
		return strings.Join(sub, " -> ")
	}
	return strings.Join(append(path, depKey), " -> ")
}

// DetectAndReconcileDrift audits managedFields for out-of-band kubectl edit modifications.
func (m *SyncWaveManager) DetectAndReconcileDrift(ctx context.Context, releaseName, namespace string, newChartManifest []byte) (*domainGitops.DriftReconciliationResult, error) {
	m.metrics.DriftEvaluationsCount.Inc()

	var newChartObj map[string]interface{}
	_ = yaml.Unmarshal(newChartManifest, &newChartObj)

	kind := "Deployment"
	name := releaseName
	if k, ok := newChartObj["kind"].(string); ok && k != "" {
		kind = k
	}
	if meta, ok := newChartObj["metadata"].(map[string]interface{}); ok {
		if n, ok := meta["name"].(string); ok && n != "" {
			name = n
		}
	}

	var liveRaw []byte
	if m.clusterProvider != nil {
		liveRaw, _ = m.clusterProvider.GetLiveResource(ctx, namespace, kind, name)
	}
	if len(liveRaw) == 0 {
		liveRaw = newChartManifest
	}

	var liveObj map[string]interface{}
	if err := json.Unmarshal(liveRaw, &liveObj); err != nil {
		_ = yaml.Unmarshal(liveRaw, &liveObj)
	}

	driftedFields := extractKubectlEditDrift(liveObj)
	if len(driftedFields) == 0 {
		return &domainGitops.DriftReconciliationResult{
			HasDrift:         false,
			DriftedFields:    nil,
			ConflictDetected: false,
			AutoResolved:     false,
		}, nil
	}

	// Check if any drifted field conflicts with desired fields in newChartManifest
	hasConflict := checkDriftConflict(driftedFields, newChartObj)
	if hasConflict {
		m.metrics.DriftConflictsCount.Inc()
		res := &domainGitops.DriftReconciliationResult{
			HasDrift:         true,
			DriftedFields:    driftedFields,
			ConflictDetected: true,
			AutoResolved:     false,
		}
		return res, domainGitops.ErrGitOpsDriftConflict.WithDetail(
			fmt.Sprintf("conflicting fields detected in release %s/%s: %v", namespace, releaseName, driftedFields),
		)
	}

	m.metrics.DriftAutoResolvedCount.Inc()
	return &domainGitops.DriftReconciliationResult{
		HasDrift:         true,
		DriftedFields:    driftedFields,
		ConflictDetected: false,
		AutoResolved:     true,
	}, nil
}

func extractKubectlEditDrift(liveObj map[string]interface{}) []string {
	meta, ok := liveObj["metadata"].(map[string]interface{})
	if !ok {
		return nil
	}
	managedFields, ok := meta["managedFields"].([]interface{})
	if !ok {
		return nil
	}

	var drifted []string
	for _, item := range managedFields {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		manager, _ := entry["manager"].(string)
		if strings.Contains(strings.ToLower(manager), "kubectl-edit") {
			if fieldsV1, ok := entry["fieldsV1"].(map[string]interface{}); ok {
				drifted = append(drifted, flattenFieldsV1(fieldsV1, "")...)
			}
		}
	}
	return drifted
}

func flattenFieldsV1(node map[string]interface{}, prefix string) []string {
	var res []string
	for k, v := range node {
		cleanKey := k
		if strings.HasPrefix(cleanKey, "f:") {
			cleanKey = strings.TrimPrefix(cleanKey, "f:")
		} else if strings.HasPrefix(cleanKey, "k:") {
			cleanKey = "[0]"
		}

		path := cleanKey
		if prefix != "" {
			if strings.HasPrefix(cleanKey, "[") {
				path = prefix + cleanKey
			} else {
				path = prefix + "." + cleanKey
			}
		}

		if sub, ok := v.(map[string]interface{}); ok && len(sub) > 0 {
			res = append(res, flattenFieldsV1(sub, path)...)
		} else {
			res = append(res, path)
		}
	}
	return res
}

func checkDriftConflict(driftedFields []string, newChartObj map[string]interface{}) bool {
	spec, ok := newChartObj["spec"].(map[string]interface{})
	if !ok {
		return false
	}

	for _, field := range driftedFields {
		if strings.Contains(field, "image") {
			// Check if new chart also specifies a container image
			if tmpl, ok := spec["template"].(map[string]interface{}); ok {
				if tSpec, ok := tmpl["spec"].(map[string]interface{}); ok {
					if containers, ok := tSpec["containers"].([]interface{}); ok {
						for _, c := range containers {
							if cMap, ok := c.(map[string]interface{}); ok {
								if img, ok := cMap["image"].(string); ok && img != "" {
									return true
								}
							}
						}
					}
				}
			}
		}
		if strings.Contains(field, "replicas") {
			if r, ok := spec["replicas"]; ok && r != nil {
				return true
			}
		}
	}
	return false
}
