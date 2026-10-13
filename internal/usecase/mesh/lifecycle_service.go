// Package mesh provides service mesh lifecycle orchestration, admission mutation,
// and WAF regular expression ReDoS protection.
package mesh

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	domainmesh "github.com/datdt/k8sselfhost/internal/domain/mesh"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	"go.uber.org/zap"
)

type contextKey string

const (
	ContextKeyTargetK8sVersion contextKey = "mesh.target_k8s_version"
	defaultK8sVersion                      = "1.28"
	defaultWAFTimeout                      = 5 * time.Millisecond
)

// WithTargetK8sVersion binds a target Kubernetes cluster version to the context.
func WithTargetK8sVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, ContextKeyTargetK8sVersion, version)
}

// JSONPatchOp represents an RFC 6902 JSON Patch operation.
type JSONPatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

var exemptNamespaces = map[string]struct{}{"kube-system": {}, "kube-public": {}, "kube-node-lease": {}, "istio-system": {}}

var (
	reBackreferences = regexp.MustCompile(`\\[1-9]|\\[kK][<']`)
	reLookarounds    = regexp.MustCompile(`\(\?(?:=|!|<=|<!)`)
)

func hasNestedQuantifier(pattern string) bool {
	type groupInfo struct{ hasRepetition bool }
	var stack []groupInfo
	inCharClass, escaped := false, false

	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '[' && !inCharClass {
			inCharClass = true
			continue
		}
		if ch == ']' && inCharClass {
			inCharClass = false
			continue
		}
		if inCharClass {
			continue
		}

		if ch == '(' {
			stack = append(stack, groupInfo{hasRepetition: false})
			continue
		}
		if ch == ')' && len(stack) > 0 {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			nextIdx := i + 1
			for nextIdx < len(pattern) && (pattern[nextIdx] == ' ' || pattern[nextIdx] == '\t') {
				nextIdx++
			}
			hasOuterRepetition := false
			if nextIdx < len(pattern) {
				nextCh := pattern[nextIdx]
				if nextCh == '+' || nextCh == '*' || nextCh == '{' {
					hasOuterRepetition = true
				}
			}

			if top.hasRepetition && hasOuterRepetition {
				return true
			}

			if len(stack) > 0 && (top.hasRepetition || hasOuterRepetition) {
				stack[len(stack)-1].hasRepetition = true
			}
			continue
		}

		if len(stack) > 0 && (ch == '+' || ch == '*' || ch == '{') {
			stack[len(stack)-1].hasRepetition = true
		}
	}
	return false
}

// LifecycleService implements domainmesh.MeshLifecycleManager.
type LifecycleService struct {
	client            kubernetes.Interface
	defaultK8sVersion string
	wafTimeout        time.Duration
	logger            *zap.Logger
}

// Option configures LifecycleService instances.
type Option func(*LifecycleService)

func WithKubernetesClient(client kubernetes.Interface) Option { return func(s *LifecycleService) { s.client = client } }
func WithDefaultK8sVersion(v string) Option { return func(s *LifecycleService) { if strings.TrimSpace(v) != "" { s.defaultK8sVersion = v } } }
func WithWAFTimeout(t time.Duration) Option { return func(s *LifecycleService) { if t > 0 { s.wafTimeout = t } } }
func WithLogger(l *zap.Logger) Option { return func(s *LifecycleService) { s.logger = l } }

// NewLifecycleService creates a new LifecycleService instance.
func NewLifecycleService(opts ...Option) *LifecycleService {
	s := &LifecycleService{
		defaultK8sVersion: defaultK8sVersion,
		wafTimeout:        defaultWAFTimeout,
		logger:            zap.NewNop(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// InjectSidecarStartupGuard inspects a pod specification or AdmissionReview,
// injects proxy readiness guards, and returns either a mutated AdmissionReview or RFC 6902 JSON Patch.
func (s *LifecycleService) InjectSidecarStartupGuard(ctx context.Context, podSpecBytes []byte) ([]byte, error) {
	if len(podSpecBytes) == 0 {
		return nil, domainerrors.NewValidation("podSpec", "pod specification cannot be empty")
	}

	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(podSpecBytes, &review); err == nil && review.Request != nil && len(review.Request.Object.Raw) > 0 {
		return s.handleAdmissionReview(ctx, &review)
	}

	var pod corev1.Pod
	if err := yaml.Unmarshal(podSpecBytes, &pod); err != nil {
		return nil, fmt.Errorf("decoding pod specification: %w", err)
	}
	if s.isControlPlaneOrDaemonSet(&pod) {
		return []byte("[]"), nil
	}

	targetVersion := s.resolveTargetVersion(ctx, &pod)
	patchBytes, _, err := s.generateSidecarGuardPatch(&pod, targetVersion)
	return patchBytes, err
}

func (s *LifecycleService) handleAdmissionReview(ctx context.Context, review *admissionv1.AdmissionReview) ([]byte, error) {
	req := review.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		return nil, fmt.Errorf("decoding pod from admission review: %w", err)
	}
	if pod.Namespace == "" && req.Namespace != "" {
		pod.Namespace = req.Namespace
	}

	resp := &admissionv1.AdmissionResponse{UID: req.UID, Allowed: true}
	if !s.isControlPlaneOrDaemonSet(&pod) {
		targetVersion := s.resolveTargetVersion(ctx, &pod)
		patchBytes, mutated, err := s.generateSidecarGuardPatch(&pod, targetVersion)
		if err != nil {
			return nil, err
		}
		if mutated && len(patchBytes) > 0 {
			patchType := admissionv1.PatchTypeJSONPatch
			resp.PatchType = &patchType
			resp.Patch = patchBytes
		}
	}

	reviewResp := admissionv1.AdmissionReview{TypeMeta: review.TypeMeta, Response: resp}
	return json.Marshal(reviewResp)
}

func (s *LifecycleService) isControlPlaneOrDaemonSet(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if _, ok := exemptNamespaces[strings.TrimSpace(pod.Namespace)]; ok {
		return true
	}
	for k, v := range pod.Labels {
		if (k == "component" || k == "tier") && (v == "control-plane" || v == "kube-apiserver" || v == "kube-scheduler" || v == "kube-controller-manager" || v == "etcd") {
			return true
		}
		if (k == "app" || k == "k8s-app" || strings.Contains(k, "cni") || strings.Contains(k, "csi")) &&
			(v == "calico-node" || v == "flannel" || v == "cilium" || v == "kube-proxy" || strings.HasPrefix(v, "csi-") || strings.HasPrefix(v, "aws-node")) {
			return true
		}
	}
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			name := strings.ToLower(pod.Name)
			if strings.Contains(name, "calico") || strings.Contains(name, "cilium") || strings.Contains(name, "flannel") || strings.Contains(name, "csi") || strings.Contains(name, "proxy") {
				return true
			}
		}
	}
	return false
}

func (s *LifecycleService) resolveTargetVersion(ctx context.Context, pod *corev1.Pod) string {
	if val, ok := ctx.Value(ContextKeyTargetK8sVersion).(string); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	if pod != nil && pod.Annotations != nil {
		if v, ok := pod.Annotations["mesh.k8sselfhost.io/k8s-version"]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return s.defaultK8sVersion
}

func isK8s128OrHigher(v string) bool {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return true
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major > 1 {
		return true
	}
	if major < 1 || len(parts) < 2 {
		return false
	}
	minorStr := parts[1]
	for i, r := range minorStr {
		if r < '0' || r > '9' {
			minorStr = minorStr[:i]
			break
		}
	}
	minor, err := strconv.Atoi(minorStr)
	if err != nil {
		return true
	}
	return minor >= 28
}

func (s *LifecycleService) generateSidecarGuardPatch(pod *corev1.Pod, targetVersion string) ([]byte, bool, error) {
	var patches []JSONPatchOp

	if isK8s128OrHigher(targetVersion) {
		always := corev1.ContainerRestartPolicyAlways
		proxyIdx := -1
		for i, c := range pod.Spec.Containers {
			if c.Name == "istio-proxy" {
				proxyIdx = i
				break
			}
		}

		if proxyIdx >= 0 {
			proxyContainer := pod.Spec.Containers[proxyIdx]
			proxyContainer.RestartPolicy = &always
			patches = append(patches, JSONPatchOp{Op: "remove", Path: fmt.Sprintf("/spec/containers/%d", proxyIdx)})

			if len(pod.Spec.InitContainers) == 0 {
				patches = append(patches, JSONPatchOp{Op: "add", Path: "/spec/initContainers", Value: []corev1.Container{proxyContainer}})
			} else {
				patches = append(patches, JSONPatchOp{Op: "add", Path: fmt.Sprintf("/spec/initContainers/%d", len(pod.Spec.InitContainers)), Value: proxyContainer})
			}
		} else {
			initProxyIdx := -1
			for i, c := range pod.Spec.InitContainers {
				if c.Name == "istio-proxy" {
					initProxyIdx = i
					break
				}
			}
			if initProxyIdx >= 0 {
				patches = append(patches, JSONPatchOp{Op: "add", Path: fmt.Sprintf("/spec/initContainers/%d/restartPolicy", initProxyIdx), Value: "Always"})
			} else {
				proxyContainer := corev1.Container{
					Name:          "istio-proxy",
					Image:         "docker.io/istio/proxyv2:default",
					RestartPolicy: &always,
				}
				if len(pod.Spec.InitContainers) == 0 {
					patches = append(patches, JSONPatchOp{Op: "add", Path: "/spec/initContainers", Value: []corev1.Container{proxyContainer}})
				} else {
					patches = append(patches, JSONPatchOp{Op: "add", Path: fmt.Sprintf("/spec/initContainers/%d", len(pod.Spec.InitContainers)), Value: proxyContainer})
				}
			}
		}
	} else {
		const annotationKey = "proxy.istio.io/config"
		const escapedPath = "/metadata/annotations/proxy.istio.io~1config"

		if pod.Annotations == nil {
			patches = append(patches, JSONPatchOp{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{annotationKey: `{"holdApplicationUntilProxyStarts": true}`},
			})
		} else if existingConfig, ok := pod.Annotations[annotationKey]; !ok {
			patches = append(patches, JSONPatchOp{
				Op:    "add",
				Path:  escapedPath,
				Value: `{"holdApplicationUntilProxyStarts": true}`,
			})
		} else {
			var configMap map[string]interface{}
			if err := json.Unmarshal([]byte(existingConfig), &configMap); err == nil {
				configMap["holdApplicationUntilProxyStarts"] = true
				mergedBytes, _ := json.Marshal(configMap)
				patches = append(patches, JSONPatchOp{Op: "replace", Path: escapedPath, Value: string(mergedBytes)})
			} else {
				patches = append(patches, JSONPatchOp{Op: "replace", Path: escapedPath, Value: `{"holdApplicationUntilProxyStarts": true}`})
			}
		}
	}

	if len(patches) == 0 {
		return []byte("[]"), false, nil
	}
	bytes, err := json.Marshal(patches)
	if err != nil {
		return nil, false, fmt.Errorf("marshaling json patch: %w", err)
	}
	return bytes, true, nil
}

// VerifyWAFRegexSafety audits a regular expression using static checks, RE2 compilation,
// and a 10,000-character synthetic adversarial stress test with strict execution timeout.
func (s *LifecycleService) VerifyWAFRegexSafety(pattern string) (*domainmesh.RegexVerificationReport, error) {
	report := &domainmesh.RegexVerificationReport{
		Pattern:           pattern,
		IsLinearDFA:       false,
		HasReDoSBacktrack: true,
		Verdict:           "REJECTED",
	}

	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		report.Reason = "Pattern cannot be empty"
		return report, domainmesh.ErrWAFNonLinearRegex
	}

	if reBackreferences.MatchString(trimmed) {
		report.Reason = "Pattern contains PCRE backreferences (\\1..\\9, \\k<name>) incompatible with linear DFA"
		return report, domainmesh.ErrWAFNonLinearRegex
	}
	if reLookarounds.MatchString(trimmed) {
		report.Reason = "Pattern contains PCRE lookaheads or lookbehinds ((?=...), (?!...), etc.)"
		return report, domainmesh.ErrWAFNonLinearRegex
	}
	if hasNestedQuantifier(trimmed) {
		report.Reason = "Pattern contains catastrophic nested quantifiers vulnerable to ReDoS"
		return report, domainmesh.ErrWAFNonLinearRegex
	}

	re, err := regexp.Compile(trimmed)
	if err != nil {
		report.Reason = fmt.Sprintf("RE2 compilation rejected pattern: %v", err)
		return report, domainmesh.ErrWAFNonLinearRegex
	}

	adversarialPayload1 := strings.Repeat("a", 10000) + "!"
	adversarialPayload2 := strings.Repeat("a=", 5000) + "!"

	evalDone := make(chan bool, 1)
	start := time.Now()
	go func() {
		_ = re.MatchString(adversarialPayload1)
		_ = re.MatchString(adversarialPayload2)
		evalDone <- true
	}()

	select {
	case <-evalDone:
		elapsed := time.Since(start)
		report.MaxEvalTime = elapsed
		if elapsed > s.wafTimeout {
			report.Reason = fmt.Sprintf("Regex evaluation exceeded hard %v execution timeout (took %v)", s.wafTimeout, elapsed)
			return report, domainmesh.ErrWAFNonLinearRegex
		}
	case <-time.After(s.wafTimeout):
		report.MaxEvalTime = s.wafTimeout
		report.Reason = fmt.Sprintf("Regex evaluation exceeded hard %v execution timeout", s.wafTimeout)
		return report, domainmesh.ErrWAFNonLinearRegex
	}

	report.IsLinearDFA = true
	report.HasReDoSBacktrack = false
	report.Verdict = "APPROVED"
	report.Reason = "Passed static ReDoS audit and RE2 linear DFA execution benchmark"
	return report, nil
}

// ConfigureGracefulTLSDrain patches an Envoy or Ingress deployment with termination drain configuration.
func (s *LifecycleService) ConfigureGracefulTLSDrain(ctx context.Context, namespace, deployment string, drainDuration time.Duration) error {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return domainerrors.NewValidation("namespace", "namespace cannot be empty")
	}
	dep := strings.TrimSpace(deployment)
	if dep == "" {
		return domainerrors.NewValidation("deployment", "deployment cannot be empty")
	}
	if drainDuration <= 0 {
		return domainerrors.NewValidation("drainDuration", "drain duration must be positive")
	}
	if s.client == nil {
		return nil
	}

	deploy, err := s.client.AppsV1().Deployments(ns).Get(ctx, dep, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s/%s: %w", ns, dep, err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}

	drainSeconds := int(drainDuration.Seconds())
	if drainSeconds == 0 {
		drainSeconds = 1
	}

	deploy.Spec.Template.Annotations["proxy.istio.io/drain-duration"] = fmt.Sprintf("%ds", drainSeconds)
	deploy.Spec.Template.Annotations["proxy.istio.io/exit-with-zero-drain"] = "true"
	deploy.Spec.Template.Annotations["ingress.kubernetes.io/drain-duration"] = fmt.Sprintf("%ds", drainSeconds)

	for i := range deploy.Spec.Template.Spec.Containers {
		c := &deploy.Spec.Template.Spec.Containers[i]
		if c.Name == "istio-proxy" || c.Name == "envoy" || len(deploy.Spec.Template.Spec.Containers) == 1 {
			if c.Lifecycle == nil {
				c.Lifecycle = &corev1.Lifecycle{}
			}
			c.Lifecycle.PreStop = &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", fmt.Sprintf("sleep %d", drainSeconds)},
				},
			}
		}
	}

	_, err = s.client.AppsV1().Deployments(ns).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating deployment %s/%s with graceful drain: %w", ns, dep, err)
	}
	return nil
}
