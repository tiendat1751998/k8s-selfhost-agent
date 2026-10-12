package network

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	domainnet "github.com/datdt/k8sselfhost/internal/domain/network"
)

// DefaultDNSMetricsProvider provides thread-safe observation and metric reporting for CoreDNS.
type DefaultDNSMetricsProvider struct {
	mu         sync.RWMutex
	latencies  map[string]time.Duration
	errorRates map[string]float64
}

// NewDefaultDNSMetricsProvider initializes a new DefaultDNSMetricsProvider.
func NewDefaultDNSMetricsProvider() *DefaultDNSMetricsProvider {
	return &DefaultDNSMetricsProvider{
		latencies:  make(map[string]time.Duration),
		errorRates: make(map[string]float64),
	}
}

// SetMetrics configures synthetic or observed metrics for a cluster.
func (p *DefaultDNSMetricsProvider) SetMetrics(clusterID string, p99Latency time.Duration, errorRate float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latencies[clusterID] = p99Latency
	p.errorRates[clusterID] = errorRate
}

// GetDNSMetrics retrieves P99 latency and error rate for a cluster.
func (p *DefaultDNSMetricsProvider) GetDNSMetrics(_ context.Context, clusterID string) (time.Duration, float64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	latency, ok := p.latencies[clusterID]
	if !ok {
		latency = 5 * time.Millisecond // default healthy latency
	}
	rate := p.errorRates[clusterID]
	return latency, rate, nil
}

// JSONPatchOp represents an RFC 6902 JSON patch operation.
type JSONPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// MutatePodSpecDNSConfig injects ndots:2 and single-request-reopen into PodSpec if not already present.
func MutatePodSpecDNSConfig(spec *corev1.PodSpec) bool {
	if spec == nil {
		return false
	}

	mutated := false
	if spec.DNSConfig == nil {
		spec.DNSConfig = &corev1.PodDNSConfig{}
		mutated = true
	}

	two := "2"
	hasNdots := false
	hasSingleRequestReopen := false

	for i := range spec.DNSConfig.Options {
		opt := &spec.DNSConfig.Options[i]
		if opt.Name == "ndots" {
			hasNdots = true
			if opt.Value == nil || *opt.Value != two {
				opt.Value = &two
				mutated = true
			}
		} else if opt.Name == "single-request-reopen" {
			hasSingleRequestReopen = true
		}
	}

	if !hasNdots {
		spec.DNSConfig.Options = append(spec.DNSConfig.Options, corev1.PodDNSConfigOption{
			Name:  "ndots",
			Value: &two,
		})
		mutated = true
	}

	if !hasSingleRequestReopen {
		spec.DNSConfig.Options = append(spec.DNSConfig.Options, corev1.PodDNSConfigOption{
			Name: "single-request-reopen",
		})
		mutated = true
	}

	return mutated
}

// GenerateDNSConfigJSONPatch calculates the RFC 6902 JSON patch to inject ndots:2 and single-request-reopen.
func GenerateDNSConfigJSONPatch(pod *corev1.Pod) ([]byte, bool, error) {
	if pod == nil {
		return nil, false, fmt.Errorf("pod cannot be nil")
	}

	originalCopy := pod.DeepCopy()
	mutated := MutatePodSpecDNSConfig(&originalCopy.Spec)
	if !mutated {
		return []byte("[]"), false, nil
	}

	var patches []JSONPatchOp
	if pod.Spec.DNSConfig == nil {
		patches = append(patches, JSONPatchOp{
			Op:    "add",
			Path:  "/spec/dnsConfig",
			Value: originalCopy.Spec.DNSConfig,
		})
	} else {
		patches = append(patches, JSONPatchOp{
			Op:    "replace",
			Path:  "/spec/dnsConfig/options",
			Value: originalCopy.Spec.DNSConfig.Options,
		})
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		return nil, false, fmt.Errorf("marshalling json patch: %w", err)
	}

	return patchBytes, true, nil
}

// HandleDNSMutationAdmissionReview decodes an AdmissionReview, mutates Pod DNS config, and returns the response.
func HandleDNSMutationAdmissionReview(body []byte) ([]byte, error) {
	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		return nil, fmt.Errorf("unmarshalling admission review: %w", err)
	}

	if review.Request == nil {
		return nil, fmt.Errorf("admission review request is nil")
	}

	var pod corev1.Pod
	if err := json.Unmarshal(review.Request.Object.Raw, &pod); err != nil {
		return nil, fmt.Errorf("unmarshalling pod from admission request: %w", err)
	}

	patchBytes, mutated, err := GenerateDNSConfigJSONPatch(&pod)
	if err != nil {
		return nil, fmt.Errorf("generating dns patch: %w", err)
	}

	resp := &admissionv1.AdmissionResponse{
		UID:     review.Request.UID,
		Allowed: true,
	}

	if mutated && len(patchBytes) > 0 {
		patchType := admissionv1.PatchTypeJSONPatch
		resp.PatchType = &patchType
		resp.Patch = patchBytes
	}

	reviewResponse := admissionv1.AdmissionReview{
		TypeMeta: review.TypeMeta,
		Response: resp,
	}

	return json.Marshal(reviewResponse)
}

// performDNSHealthCheck inspects CoreDNS replica count, NodeLocal status, and latency thresholds.
func performDNSHealthCheck(
	ctx context.Context,
	clusterID string,
	client kubernetes.Interface,
	metrics domainnet.DNSMetricsProvider,
	logger *zap.Logger,
) (*domainnet.DNSHealthReport, error) {
	replicas := 0

	// 1. Check CoreDNS Deployment in kube-system
	deploy, err := client.AppsV1().Deployments("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
	if err == nil {
		if deploy.Status.ReadyReplicas > 0 {
			replicas = int(deploy.Status.ReadyReplicas)
		} else if deploy.Spec.Replicas != nil {
			replicas = int(*deploy.Spec.Replicas)
		}
	} else {
		// Fallback to kube-dns if coredns not found
		if kd, kdErr := client.AppsV1().Deployments("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{}); kdErr == nil {
			if kd.Status.ReadyReplicas > 0 {
				replicas = int(kd.Status.ReadyReplicas)
			} else if kd.Spec.Replicas != nil {
				replicas = int(*kd.Spec.Replicas)
			}
		}
	}

	// 2. Check NodeLocal DNSCache DaemonSet
	nodeLocalActive := false
	ds, err := client.AppsV1().DaemonSets("kube-system").Get(ctx, "node-local-dns", metav1.GetOptions{})
	if err == nil && ds.Status.NumberReady > 0 {
		nodeLocalActive = true
	}

	// 3. Query DNS metrics
	var p99Latency time.Duration = 5 * time.Millisecond
	var errorRate float64 = 0.0
	if metrics != nil {
		p99, rate, mErr := metrics.GetDNSMetrics(ctx, clusterID)
		if mErr == nil {
			p99Latency = p99
			errorRate = rate
		}
	}

	alertTriggered := false
	if p99Latency > 100*time.Millisecond || errorRate > 0.05 {
		alertTriggered = true
		logger.Warn("CoreDNS critical alert: high latency or error rate detected",
			zap.String("cluster_id", clusterID),
			zap.Duration("p99_latency", p99Latency),
			zap.Float64("error_rate", errorRate),
			zap.Int("coredns_replicas", replicas),
			zap.Bool("node_local_active", nodeLocalActive),
		)
	}

	return &domainnet.DNSHealthReport{
		ClusterID:       clusterID,
		CoreDNSReplicas: replicas,
		P99Latency:      p99Latency,
		ErrorRate:       errorRate,
		NodeLocalActive: nodeLocalActive,
		AlertTriggered:  alertTriggered,
	}, nil
}

// performInjectDNSConfigOptimization mutates all Pod Deployments in namespace to add ndots:2 and single-request-reopen.
func performInjectDNSConfigOptimization(
	ctx context.Context,
	clusterID, namespace string,
	client kubernetes.Interface,
	logger *zap.Logger,
) error {
	ns := namespace
	if ns == "" {
		ns = metav1.NamespaceAll
	}

	deployList, err := client.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing deployments for dns optimization: %w", err)
	}

	mutatedCount := 0
	for i := range deployList.Items {
		deploy := &deployList.Items[i]
		if MutatePodSpecDNSConfig(&deploy.Spec.Template.Spec) {
			_, updateErr := client.AppsV1().Deployments(deploy.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
			if updateErr != nil {
				logger.Warn("Failed to update deployment with dns optimization",
					zap.String("deployment", deploy.Name),
					zap.String("namespace", deploy.Namespace),
					zap.Error(updateErr),
				)
				continue
			}
			mutatedCount++
		}
	}

	logger.Info("DNS config optimization injected successfully",
		zap.String("cluster_id", clusterID),
		zap.String("namespace", namespace),
		zap.Int("deployments_mutated", mutatedCount),
	)

	return nil
}
