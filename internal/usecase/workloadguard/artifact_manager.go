// Package workloadguard provides the usecase orchestration engine for OCI image digest
// resolution, ephemeral storage enforcement, and pod PID exhaustion containment.
package workloadguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/workloadguard"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Supported failure policy constants.
const (
	FailurePolicyIgnore = "Ignore"
	FailurePolicyFail   = "Fail"
)

// Floating tag patterns that must be rejected to enforce immutability.
var floatingTags = map[string]struct{}{
	"latest":  {},
	"dev":     {},
	"staging": {},
	"canary":  {},
}

// HTTPClient defines the client interface for container registry queries.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Option configures an ArtifactManager instance.
type Option func(*ArtifactManager)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(client HTTPClient) Option {
	return func(m *ArtifactManager) {
		m.client = client
	}
}

// WithFailurePolicy sets the circuit-breaker failure policy ("Ignore" or "Fail").
func WithFailurePolicy(policy string) Option {
	return func(m *ArtifactManager) {
		m.failurePolicy = policy
	}
}

// WithRegistryHostOverride overrides remote registry routing (useful for test servers).
func WithRegistryHostOverride(host string) Option {
	return func(m *ArtifactManager) {
		m.hostOverride = host
	}
}

// WithLogger injects a structured logger into ArtifactManager.
func WithLogger(logger *zap.Logger) Option {
	return func(m *ArtifactManager) {
		m.logger = logger
	}
}

// ArtifactManager orchestrates workload artifact guards.
type ArtifactManager struct {
	client         HTTPClient
	failurePolicy  string
	hostOverride   string
	cache          sync.Map // string -> *domain.ImageDigestResolution
	digestCounter  domain.PaddedDigestResolutionCounter
	storageCounter domain.PaddedStorageCheckCounter
	pidCounter     domain.PaddedPIDAdmissionCounter
	logger         *zap.Logger
}

// NewArtifactManager creates a fully initialized ArtifactManager.
func NewArtifactManager(opts ...Option) *ArtifactManager {
	mgr := &ArtifactManager{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		failurePolicy: FailurePolicyIgnore,
		logger:        zap.NewNop(),
	}
	for _, opt := range opts {
		opt(mgr)
	}
	return mgr
}

// DigestCounter returns the 64-byte padded digest counter.
func (m *ArtifactManager) DigestCounter() *domain.PaddedDigestResolutionCounter {
	return &m.digestCounter
}

// StorageCounter returns the 64-byte padded storage check counter.
func (m *ArtifactManager) StorageCounter() *domain.PaddedStorageCheckCounter {
	return &m.storageCounter
}

// PIDCounter returns the 64-byte padded PID limiter counter.
func (m *ArtifactManager) PIDCounter() *domain.PaddedPIDAdmissionCounter {
	return &m.pidCounter
}

// ResolveImageToDigest resolves a container image reference to an immutable SHA256 digest.
func (m *ArtifactManager) ResolveImageToDigest(ctx context.Context, image string) (*domain.ImageDigestResolution, error) {
	cleanImage := strings.TrimSpace(image)
	if cleanImage == "" {
		return nil, domain.ErrMutableImageTagRejected
	}

	// 1. If already referenced by immutable sha256 digest, accept immediately
	if strings.Contains(cleanImage, "@sha256:") {
		m.digestCounter.Inc()
		return &domain.ImageDigestResolution{
			OriginalImage:  cleanImage,
			ResolvedDigest: cleanImage,
			IsImmutable:    true,
		}, nil
	}

	// 2. Parse repository and tag
	repo, tag := parseImageRepoAndTag(cleanImage)
	tagLower := strings.ToLower(tag)

	// Floating tag check
	if _, isFloating := floatingTags[tagLower]; isFloating || tag == "" {
		return nil, domain.ErrMutableImageTagRejected
	}

	// 3. High-frequency cache lookup
	if cached, ok := m.cache.Load(cleanImage); ok {
		m.digestCounter.Inc()
		return cached.(*domain.ImageDigestResolution), nil
	}

	// 4. Query registry for immutable Docker-Content-Digest header
	digest, err := m.fetchDigestFromRegistry(ctx, repo, tag)
	if err != nil {
		if m.failurePolicy == FailurePolicyIgnore {
			m.logger.Warn("Registry unreachable, continuing under failurePolicy: Ignore",
				zap.String("image", cleanImage),
				zap.Error(err),
			)
			return &domain.ImageDigestResolution{
				OriginalImage:  cleanImage,
				ResolvedDigest: cleanImage,
				IsImmutable:    false,
			}, nil
		}
		return nil, fmt.Errorf("resolving image %s from registry: %w", cleanImage, err)
	}

	resolvedDigest := fmt.Sprintf("%s@%s", repo, digest)
	res := &domain.ImageDigestResolution{
		OriginalImage:  cleanImage,
		ResolvedDigest: resolvedDigest,
		IsImmutable:    true,
	}

	m.cache.Store(cleanImage, res)
	m.digestCounter.Inc()
	return res, nil
}

// fetchDigestFromRegistry performs HEAD/GET request to OCI manifest endpoint.
func (m *ArtifactManager) fetchDigestFromRegistry(ctx context.Context, repo, tag string) (string, error) {
	registryHost, repoPath := parseRegistryAndRepo(repo, m.hostOverride)
	scheme := "https"
	if strings.HasPrefix(registryHost, "127.0.0.1") || strings.HasPrefix(registryHost, "localhost") {
		scheme = "http"
	}

	reqURL := fmt.Sprintf("%s://%s/v2/%s/manifests/%s", scheme, registryHost, repoPath, tag)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, reqURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.index.v1+json")

	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		digest = resp.Header.Get("ETag")
		digest = strings.Trim(digest, `"`)
	}
	if digest == "" {
		return "", fmt.Errorf("registry response missing Docker-Content-Digest header")
	}

	return digest, nil
}

// ValidateEphemeralStoragePolicy validates container ephemeral-storage limits and /tmp emptyDir mounts.
func (m *ArtifactManager) ValidateEphemeralStoragePolicy(ctx context.Context, podSpecJSON []byte) (*domain.EphemeralStorageCheck, error) {
	pod, err := parsePodSpec(podSpecJSON)
	if err != nil {
		return nil, err
	}

	m.storageCounter.Inc()

	check := &domain.EphemeralStorageCheck{
		HasLimit:             true,
		TmpMountedAsEmptyDir: true,
		IsPassed:             true,
	}

	var totalLimitBytes int64
	containers := append(pod.Spec.Containers, pod.Spec.InitContainers...)
	if len(containers) == 0 {
		check.IsPassed = false
		check.HasLimit = false
		check.RejectReason = "pod specification contains no containers"
		return check, nil
	}

	// Map volume names to emptyDir check
	emptyDirVolumes := make(map[string]bool)
	for _, v := range pod.Spec.Volumes {
		if v.EmptyDir != nil {
			emptyDirVolumes[v.Name] = true
		}
	}

	for _, c := range containers {
		// 1. Audit ephemeral-storage resource limits
		ephLimit, hasEph := c.Resources.Limits[corev1.ResourceEphemeralStorage]
		if !hasEph || ephLimit.Value() <= 0 {
			check.HasLimit = false
			check.IsPassed = false
			check.RejectReason = fmt.Sprintf("container %q lacks ephemeral-storage resource limit", c.Name)
			return check, nil
		}
		totalLimitBytes += ephLimit.Value()

		// 2. Audit /tmp mount for emptyDir backing
		hasTmpMount := false
		for _, vm := range c.VolumeMounts {
			if vm.MountPath == "/tmp" {
				hasTmpMount = true
				if !emptyDirVolumes[vm.Name] {
					check.TmpMountedAsEmptyDir = false
					check.IsPassed = false
					check.RejectReason = fmt.Sprintf("container %q /tmp is mounted from volume %q which is not an emptyDir", c.Name, vm.Name)
					return check, nil
				}
			}
		}

		if !hasTmpMount {
			check.TmpMountedAsEmptyDir = false
			check.IsPassed = false
			check.RejectReason = fmt.Sprintf("container %q must mount /tmp backed by an emptyDir volume", c.Name)
			return check, nil
		}
	}

	check.LimitBytes = totalLimitBytes
	return check, nil
}

// InjectEphemeralStorageDefaults injects default 2Gi limits and /tmp emptyDir volume mounts.
func (m *ArtifactManager) InjectEphemeralStorageDefaults(ctx context.Context, podSpecJSON []byte) ([]byte, error) {
	pod, err := parsePodSpec(podSpecJSON)
	if err != nil {
		return nil, err
	}

	const tmpVolumeName = "k8sselfhost-ephemeral-tmp"

	// Ensure emptyDir volume exists
	volumeExists := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == tmpVolumeName {
			volumeExists = true
			break
		}
	}
	if !volumeExists {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: tmpVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewQuantity(domain.DefaultTmpEmptyDirSizeLimitBytes, resource.BinarySI),
				},
			},
		})
	}

	// Inject limits and mount into containers
	injectContainer := func(c *corev1.Container) {
		if c.Resources.Limits == nil {
			c.Resources.Limits = make(corev1.ResourceList)
		}
		if _, ok := c.Resources.Limits[corev1.ResourceEphemeralStorage]; !ok {
			c.Resources.Limits[corev1.ResourceEphemeralStorage] = resource.MustParse("2Gi")
		}

		hasTmpMount := false
		for _, vm := range c.VolumeMounts {
			if vm.MountPath == "/tmp" {
				hasTmpMount = true
				break
			}
		}
		if !hasTmpMount {
			c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
				Name:      tmpVolumeName,
				MountPath: "/tmp",
			})
		}
	}

	for i := range pod.Spec.Containers {
		injectContainer(&pod.Spec.Containers[i])
	}
	for i := range pod.Spec.InitContainers {
		injectContainer(&pod.Spec.InitContainers[i])
	}

	return json.Marshal(pod)
}

// EnforcePodPIDLimit injects pod PID boundaries and rejects fork-bomb risks.
func (m *ArtifactManager) EnforcePodPIDLimit(ctx context.Context, podSpecJSON []byte, defaultMaxPIDs int64) ([]byte, error) {
	if defaultMaxPIDs <= 0 {
		defaultMaxPIDs = domain.DefaultPodPIDLimit
	}

	if defaultMaxPIDs > domain.MaxHostKernelPIDs {
		return nil, domain.ErrPIDExhaustionRisk
	}

	// Parse arbitrary pod JSON to maintain user fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal(podSpecJSON, &rawMap); err != nil {
		return nil, fmt.Errorf("decoding pod JSON: %w", err)
	}

	// Check if wrapped in pod_spec
	targetMap := rawMap
	if sub, ok := rawMap["pod_spec"].(map[string]interface{}); ok {
		targetMap = sub
	}

	// Validate metadata annotations
	metadata, _ := targetMap["metadata"].(map[string]interface{})
	if metadata == nil {
		metadata = make(map[string]interface{})
		targetMap["metadata"] = metadata
	}

	annotations, _ := metadata["annotations"].(map[string]interface{})
	if annotations == nil {
		annotations = make(map[string]interface{})
		metadata["annotations"] = annotations
	}

	// Check existing requested limits for fork-bomb risks
	if existing, ok := annotations["cgroup.kubernetes.io/pod-pids-limit"].(string); ok {
		if val, err := strconv.ParseInt(existing, 10, 64); err == nil && val > domain.MaxHostKernelPIDs {
			return nil, domain.ErrPIDExhaustionRisk
		}
	}

	limitStr := strconv.FormatInt(defaultMaxPIDs, 10)
	annotations["cgroup.kubernetes.io/pod-pids-limit"] = limitStr
	annotations["security.kubernetes.io/pod-pids-limit"] = limitStr
	targetMap["podPidsLimit"] = defaultMaxPIDs

	m.pidCounter.Inc()
	return json.Marshal(rawMap)
}

// Helper functions for parsing images and pod specs.

func parseImageRepoAndTag(image string) (repo, tag string) {
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")

	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, ""
}

func parseRegistryAndRepo(repo, override string) (registry, repoPath string) {
	if override != "" {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 && strings.Contains(parts[0], ".") {
			return override, parts[1]
		}
		return override, repo
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) == 2 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		return parts[0], parts[1]
	}

	// Default to Docker Hub
	if !strings.Contains(repo, "/") {
		return "registry-1.docker.io", "library/" + repo
	}
	return "registry-1.docker.io", repo
}

func parsePodSpec(data []byte) (*corev1.Pod, error) {
	var envelope struct {
		PodSpec *corev1.Pod `json:"pod_spec"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.PodSpec != nil && len(envelope.PodSpec.Spec.Containers) > 0 {
		return envelope.PodSpec, nil
	}

	var pod corev1.Pod
	if err := json.Unmarshal(data, &pod); err == nil && len(pod.Spec.Containers) > 0 {
		return &pod, nil
	}

	// Try unmarshaling as raw PodSpec
	var spec corev1.PodSpec
	if err := json.Unmarshal(data, &spec); err == nil && len(spec.Containers) > 0 {
		return &corev1.Pod{Spec: spec}, nil
	}

	return nil, fmt.Errorf("unable to decode valid Pod or PodSpec from payload")
}
