package incident

import "testing"

func TestType_IsValid(t *testing.T) {
	validTypes := []Type{
		TypeCrashLoopBackOff, TypeOOMKilled, TypeImagePullBackOff,
		TypeFailedScheduling, TypeNodeNotReady, TypeProbeFailed,
		TypeNetworkFailure, TypeStorageFailure, TypeHPAFailure,
		TypeResourceExhaust, TypeIngressFailure, TypeServiceUnhealthy,
	}

	for _, typ := range validTypes {
		if !typ.IsValid() {
			t.Errorf("expected type '%s' to be valid", typ)
		}
	}

	invalid := Type("NotARealType")
	if invalid.IsValid() {
		t.Error("expected invalid type to return false")
	}
}

func TestSeverity_IsValid(t *testing.T) {
	validSeverities := []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}

	for _, sev := range validSeverities {
		if !sev.IsValid() {
			t.Errorf("expected severity '%s' to be valid", sev)
		}
	}

	invalid := Severity("extreme")
	if invalid.IsValid() {
		t.Error("expected invalid severity to return false")
	}
}

func TestType_String(t *testing.T) {
	if TypeCrashLoopBackOff.String() != "CrashLoopBackOff" {
		t.Errorf("unexpected string: %s", TypeCrashLoopBackOff.String())
	}
}

func TestStatus_String(t *testing.T) {
	if StatusDetected.String() != "detected" {
		t.Errorf("unexpected string: %s", StatusDetected.String())
	}
}

func TestSeverity_String(t *testing.T) {
	if SeverityCritical.String() != "critical" {
		t.Errorf("unexpected string: %s", SeverityCritical.String())
	}
}
