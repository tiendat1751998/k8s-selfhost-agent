// Package security provides domain entities, policy rules, and interfaces
// for Kubernetes admission control and rootless container build execution.
package security

import (
	"context"
	"errors"

	admissionv1 "k8s.io/api/admission/v1"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// Domain error codes under RFC 7807 specification.
const (
	CodeDinDNotAllowed           = "ERR_DIND_NOT_ALLOWED"
	CodeAdmissionPolicyViolation = "ERR_ADMISSION_MUTATION_FAILED"
)

// Sentinel domain errors for admission control and container build isolation.
var (
	ErrDinDNotAllowed = &domainerrors.DomainError{
		Code:    CodeDinDNotAllowed,
		Message: "Mounting docker.sock or privileged DinD is forbidden. Use Kaniko rootless builder.",
		Err:     errors.New("mounting docker.sock or containerd.sock (DinD) is forbidden"),
	}
	ErrAdmissionPolicyViolation = &domainerrors.DomainError{
		Code:    CodeAdmissionPolicyViolation,
		Message: "Admission webhook rejected pod manifest due to policy violations.",
		Err:     errors.New("admission policy violation detected"),
	}
)

// AdmissionRuleViolation records an individual policy check failure.
type AdmissionRuleViolation struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// AdmissionValidationResult represents the comprehensive verdict of admission evaluation.
type AdmissionValidationResult struct {
	Allowed    bool                     `json:"allowed"`
	StatusCode int32                    `json:"status_code"`
	Reason     string                   `json:"reason"`
	Violations []AdmissionRuleViolation `json:"violations,omitempty"`
}

// PolicyEnforcer defines the port for evaluating Kubernetes admission reviews.
type PolicyEnforcer interface {
	ValidatePodSpec(ctx context.Context, review *admissionv1.AdmissionReview) (*AdmissionValidationResult, error)
}

// RootlessBuildSpec specifies the parameters for an isolated container build.
type RootlessBuildSpec struct {
	BuildID      string            `json:"build_id"`
	ContextURL   string            `json:"context_url"` // Git repository or archive URL
	Dockerfile   string            `json:"dockerfile"`
	Destination  string            `json:"destination"` // Container registry destination image tag
	BuildArgs    map[string]string `json:"build_args,omitempty"`
	RegistryAuth string            `json:"registry_auth_secret,omitempty"`
	MemoryLimit  string            `json:"memory_limit,omitempty"`
	CPULimit     string            `json:"cpu_limit,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
}

// RootlessBuildRunner defines the port for executing rootless builds in Kubernetes.
type RootlessBuildRunner interface {
	LaunchKanikoBuild(ctx context.Context, spec RootlessBuildSpec) (string, error) // Returns PodName
	StreamBuildLogs(ctx context.Context, buildID string) (<-chan string, error)
}
