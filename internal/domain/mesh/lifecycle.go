// Package mesh defines domain entities, value objects, and interfaces for
// service mesh lifecycle management and Ingress ReDoS protection.
package mesh

import (
	"context"
	"errors"
	"time"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 error codes for service mesh operations.
const (
	CodeWAFNonLinearRegex       = "ERR_WAF_NON_LINEAR_REGEX"
	CodeMeshProxyStartupTimeout = "ERR_MESH_PROXY_STARTUP_TIMEOUT"
)

// Sentinel domain errors.
var (
	ErrWAFNonLinearRegex = &domainerrors.DomainError{
		Code:    CodeWAFNonLinearRegex,
		Message: "WAF rule rejected: Regular expression contains non-linear backtracking constructs vulnerable to ReDoS.",
		Err:     errors.New("regular expression contains non-linear backtracking constructs vulnerable to ReDoS"),
	}
	ErrMeshProxyStartupTimeout = &domainerrors.DomainError{
		Code:    CodeMeshProxyStartupTimeout,
		Message: "Proxy sidecar failed to reach ready state within 60s.",
		Err:     errors.New("proxy sidecar failed to reach ready state within 60s"),
	}
)

// SidecarStartupPolicy represents the strategy used to guarantee proxy readiness before application traffic.
type SidecarStartupPolicy string

const (
	SidecarPolicyHoldApp       SidecarStartupPolicy = "HOLD_APP_UNTIL_PROXY_READY"
	SidecarPolicyNativeSidecar SidecarStartupPolicy = "K8S_NATIVE_SIDECAR"
)

// RegexVerificationReport represents the static and dynamic ReDoS audit report for a regex rule.
type RegexVerificationReport struct {
	Pattern           string        `json:"pattern"`
	IsLinearDFA       bool          `json:"is_linear_dfa"`
	MaxEvalTime       time.Duration `json:"max_eval_time"`
	HasReDoSBacktrack bool          `json:"has_redos_backtrack"`
	Verdict           string        `json:"verdict"` // APPROVED, REJECTED
	Reason            string        `json:"reason,omitempty"`
}

// MeshLifecycleManager defines application operations for service mesh sidecar lifecycle,
// regular expression safety verification, and graceful TLS drainage.
type MeshLifecycleManager interface {
	InjectSidecarStartupGuard(ctx context.Context, podSpecBytes []byte) ([]byte, error)
	VerifyWAFRegexSafety(pattern string) (*RegexVerificationReport, error)
	ConfigureGracefulTLSDrain(ctx context.Context, namespace, deployment string, drainDuration time.Duration) error
}
