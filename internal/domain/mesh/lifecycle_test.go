package mesh_test

import (
	"errors"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/mesh"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestDomainMeshErrors(t *testing.T) {
	if mesh.ErrWAFNonLinearRegex == nil {
		t.Fatal("expected ErrWAFNonLinearRegex to be defined")
	}
	var domErr *domainerrors.DomainError
	if !errors.As(mesh.ErrWAFNonLinearRegex, &domErr) {
		t.Fatalf("expected ErrWAFNonLinearRegex to be DomainError, got %T", mesh.ErrWAFNonLinearRegex)
	}
	if domErr.Code != mesh.CodeWAFNonLinearRegex {
		t.Errorf("expected code %s, got %s", mesh.CodeWAFNonLinearRegex, domErr.Code)
	}

	if mesh.ErrMeshProxyStartupTimeout == nil {
		t.Fatal("expected ErrMeshProxyStartupTimeout to be defined")
	}
	if !errors.As(mesh.ErrMeshProxyStartupTimeout, &domErr) {
		t.Fatalf("expected ErrMeshProxyStartupTimeout to be DomainError, got %T", mesh.ErrMeshProxyStartupTimeout)
	}
	if domErr.Code != mesh.CodeMeshProxyStartupTimeout {
		t.Errorf("expected code %s, got %s", mesh.CodeMeshProxyStartupTimeout, domErr.Code)
	}
}

func TestDomainSidecarStartupPolicy(t *testing.T) {
	if mesh.SidecarPolicyHoldApp != "HOLD_APP_UNTIL_PROXY_READY" {
		t.Errorf("unexpected SidecarPolicyHoldApp: %s", mesh.SidecarPolicyHoldApp)
	}
	if mesh.SidecarPolicyNativeSidecar != "K8S_NATIVE_SIDECAR" {
		t.Errorf("unexpected SidecarPolicyNativeSidecar: %s", mesh.SidecarPolicyNativeSidecar)
	}
}

func TestRegexVerificationReport(t *testing.T) {
	report := mesh.RegexVerificationReport{
		Pattern:           `^[a-z]+$`,
		IsLinearDFA:       true,
		MaxEvalTime:       100 * time.Microsecond,
		HasReDoSBacktrack: false,
		Verdict:           "APPROVED",
	}

	if report.Verdict != "APPROVED" {
		t.Errorf("expected APPROVED, got %s", report.Verdict)
	}
	if !report.IsLinearDFA {
		t.Error("expected IsLinearDFA to be true")
	}
}
