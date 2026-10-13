// Package security provides application use cases for Kubernetes admission control,
// container isolation, and supply chain security gates.
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"

	domainsec "github.com/datdt/k8sselfhost/internal/domain/security"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// Policy rule identifiers
const (
	RuleDinDNotAllowed         = domainsec.CodeDinDNotAllowed
	RuleImmutableTagsRequired  = "ERR_IMMUTABLE_TAGS_REQUIRED"
	RulePrivilegedForbidden    = "ERR_PRIVILEGED_FORBIDDEN"
	RuleReadOnlyRootFSRequired = "ERR_READONLY_ROOTFS_REQUIRED"
)

// Socket paths forbidden under zero-trust invariant
var forbiddenSockets = []string{
	"/var/run/docker.sock",
	"/run/docker.sock",
	"/var/run/containerd/containerd.sock",
	"/run/containerd/containerd.sock",
}

// System namespaces exempt from admission policy
var exemptNamespaces = map[string]struct{}{
	"kube-system": {},
	"kube-public": {},
}

// AdmissionEnforcer enforces zero-trust admission policies against Pod specifications.
type AdmissionEnforcer struct{}

// NewAdmissionEnforcer creates a new AdmissionEnforcer instance.
func NewAdmissionEnforcer() *AdmissionEnforcer {
	return &AdmissionEnforcer{}
}

// ValidatePodSpec implements domainsec.PolicyEnforcer.
func (e *AdmissionEnforcer) ValidatePodSpec(ctx context.Context, review *admissionv1.AdmissionReview) (*domainsec.AdmissionValidationResult, error) {
	if review == nil || review.Request == nil {
		return nil, errors.NewValidation("review", "admission review request cannot be nil")
	}

	req := review.Request

	// Guardrail: Bypass evaluation for exempt system namespaces
	targetNamespace := strings.TrimSpace(req.Namespace)
	if _, exempt := exemptNamespaces[targetNamespace]; exempt {
		return &domainsec.AdmissionValidationResult{
			Allowed:    true,
			StatusCode: http.StatusOK,
			Reason:     fmt.Sprintf("Namespace '%s' is exempt from admission policy", targetNamespace),
		}, nil
	}

	if len(req.Object.Raw) == 0 {
		return nil, errors.NewValidation("object", "admission review request object is empty")
	}

	// Attempt unmarshaling as corev1.Pod or corev1.PodTemplateSpec
	var pod corev1.Pod
	var spec corev1.PodSpec

	if err := json.Unmarshal(req.Object.Raw, &pod); err == nil && len(pod.Spec.Containers) > 0 {
		spec = pod.Spec
		if pod.Namespace != "" && targetNamespace == "" {
			if _, exempt := exemptNamespaces[pod.Namespace]; exempt {
				return &domainsec.AdmissionValidationResult{
					Allowed:    true,
					StatusCode: http.StatusOK,
					Reason:     fmt.Sprintf("Namespace '%s' is exempt from admission policy", pod.Namespace),
				}, nil
			}
		}
	} else {
		var pts corev1.PodTemplateSpec
		if err := json.Unmarshal(req.Object.Raw, &pts); err == nil && len(pts.Spec.Containers) > 0 {
			spec = pts.Spec
		} else {
			return nil, errors.NewValidation("object", "failed to extract pod spec from admission review object")
		}
	}

	violations := e.evaluatePodSpec(&spec)

	if len(violations) == 0 {
		return &domainsec.AdmissionValidationResult{
			Allowed:    true,
			StatusCode: http.StatusOK,
			Reason:     "Pod manifest complies with security policies",
		}, nil
	}

	// Determine status code and reason
	hasDinD := false
	for _, v := range violations {
		if v.Rule == RuleDinDNotAllowed {
			hasDinD = true
			break
		}
	}

	statusCode := int32(http.StatusUnprocessableEntity)
	reason := fmt.Sprintf("Admission webhook rejected pod manifest due to %d policy violations.", len(violations))
	if hasDinD {
		statusCode = int32(http.StatusForbidden)
		reason = "Policy violation: Mounting docker.sock is strictly forbidden under Enterprise Zero-Trust policy."
	}

	return &domainsec.AdmissionValidationResult{
		Allowed:    false,
		StatusCode: statusCode,
		Reason:     reason,
		Violations: violations,
	}, nil
}

func (e *AdmissionEnforcer) evaluatePodSpec(spec *corev1.PodSpec) []domainsec.AdmissionRuleViolation {
	var violations []domainsec.AdmissionRuleViolation

	// Check 1: Volume hostPath mounts pointing to docker.sock or containerd.sock
	for _, vol := range spec.Volumes {
		if vol.HostPath != nil {
			path := strings.TrimSpace(vol.HostPath.Path)
			if isForbiddenSocket(path) {
				violations = append(violations, domainsec.AdmissionRuleViolation{
					Field:   fmt.Sprintf("spec.volumes[%s].hostPath.path", vol.Name),
					Rule:    RuleDinDNotAllowed,
					Message: fmt.Sprintf("HostPath volume '%s' mounts Docker or containerd socket (%s), which is forbidden under zero-trust policy", vol.Name, path),
				})
			}
		}
	}

	// Evaluate regular containers
	for _, c := range spec.Containers {
		violations = append(violations, e.evaluateContainer(c.Name, c.Image, c.SecurityContext, c.VolumeMounts)...)
	}

	// Evaluate init containers
	for _, ic := range spec.InitContainers {
		violations = append(violations, e.evaluateContainer(ic.Name, ic.Image, ic.SecurityContext, ic.VolumeMounts)...)
	}

	// Evaluate ephemeral containers
	for _, ec := range spec.EphemeralContainers {
		violations = append(violations, e.evaluateContainer(ec.Name, ec.Image, ec.SecurityContext, ec.VolumeMounts)...)
	}

	return violations
}

func (e *AdmissionEnforcer) evaluateContainer(name, image string, secCtx *corev1.SecurityContext, volumeMounts []corev1.VolumeMount) []domainsec.AdmissionRuleViolation {
	var violations []domainsec.AdmissionRuleViolation

	// Check 1 (cont): VolumeMounts pointing to forbidden sockets
	for _, vm := range volumeMounts {
		mountPath := strings.TrimSpace(vm.MountPath)
		if isForbiddenSocket(mountPath) {
			violations = append(violations, domainsec.AdmissionRuleViolation{
				Field:   fmt.Sprintf("spec.containers[%s].volumeMounts[%s].mountPath", name, vm.Name),
				Rule:    RuleDinDNotAllowed,
				Message: fmt.Sprintf("Container '%s' volumeMount '%s' mounts forbidden socket (%s)", name, vm.Name, mountPath),
			})
		}
	}

	// Check 2: Image tag ending in :latest or missing tag
	if tagErr := validateImageTag(image); tagErr != "" {
		violations = append(violations, domainsec.AdmissionRuleViolation{
			Field:   fmt.Sprintf("spec.containers[%s].image", name),
			Rule:    RuleImmutableTagsRequired,
			Message: fmt.Sprintf("Container '%s': %s", name, tagErr),
		})
	}

	// Check 3: securityContext.privileged == true
	if secCtx != nil && secCtx.Privileged != nil && *secCtx.Privileged {
		violations = append(violations, domainsec.AdmissionRuleViolation{
			Field:   fmt.Sprintf("spec.containers[%s].securityContext.privileged", name),
			Rule:    RulePrivilegedForbidden,
			Message: fmt.Sprintf("Container '%s' has privileged mode enabled, which is strictly forbidden", name),
		})
	}

	// Check 4: Enforce securityContext.readOnlyRootFilesystem == true
	if secCtx == nil || secCtx.ReadOnlyRootFilesystem == nil || !*secCtx.ReadOnlyRootFilesystem {
		violations = append(violations, domainsec.AdmissionRuleViolation{
			Field:   fmt.Sprintf("spec.containers[%s].securityContext.readOnlyRootFilesystem", name),
			Rule:    RuleReadOnlyRootFSRequired,
			Message: fmt.Sprintf("Container '%s' must set securityContext.readOnlyRootFilesystem to true", name),
		})
	}

	return violations
}

func isForbiddenSocket(path string) bool {
	clean := strings.TrimRight(path, "/")
	for _, forbidden := range forbiddenSockets {
		forbiddenClean := strings.TrimRight(forbidden, "/")
		if clean == forbiddenClean {
			return true
		}
	}
	return false
}

func validateImageTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return "Image reference cannot be empty"
	}

	// If explicit latest tag suffix exists, reject immediately
	if strings.HasSuffix(image, ":latest") {
		return "Immutable tags required: tag ':latest' is strictly forbidden"
	}

	// Check if pinned by immutable cryptographic digest (e.g., app@sha256:...)
	if atIdx := strings.Index(image, "@"); atIdx != -1 {
		prefix := image[:atIdx]
		if strings.HasSuffix(prefix, ":latest") {
			return "Immutable tags required: tag ':latest' is strictly forbidden"
		}
		digest := image[atIdx+1:]
		if strings.HasPrefix(digest, "sha256:") && len(digest) > 7 {
			return "" // Compliant immutable SHA256 digest
		}
	}

	// Extract the image name part after the last registry slash
	lastSlash := strings.LastIndex(image, "/")
	imagePart := image
	if lastSlash != -1 {
		imagePart = image[lastSlash+1:]
	}

	// Check colon for tag in the imagePart
	colonIdx := strings.LastIndex(imagePart, ":")
	if colonIdx == -1 {
		return "Immutable tags required: image is missing an explicit tag"
	}

	tag := imagePart[colonIdx+1:]
	if tag == "" || tag == "latest" || strings.HasSuffix(tag, "latest") {
		return fmt.Sprintf("Immutable tags required: mutable tag '%s' is strictly forbidden", tag)
	}

	return ""
}
