package sre_test

import (
	"math"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/sre"
)

func TestRCAScoringMatrix_OOMKillerCascade(t *testing.T) {
	signals := sre.TelemetrySignals{
		DmesgOOMDetected:        true,
		CgroupMemoryPercent:     99.2,
		SwapPercent:             96.0,
		MemoryPressureCond:      true,
		OOMKilledEventsDetected: true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report == nil {
		t.Fatal("expected non-nil DynamicRCAReport")
	}

	if report.PrimaryCause != sre.HypothesisOOMKillerCascade {
		t.Errorf("expected PrimaryCause %s, got %s", sre.HypothesisOOMKillerCascade, report.PrimaryCause)
	}

	expectedScore := 1.0
	expectedConfidence := 0.99

	if math.Abs(report.Confidence-expectedConfidence) > 0.001 {
		t.Errorf("expected confidence %.2f, got %.4f", expectedConfidence, report.Confidence)
	}

	var oomHyp *sre.HypothesisEvaluation
	for i := range report.Hypotheses {
		if report.Hypotheses[i].Hypothesis == sre.HypothesisOOMKillerCascade {
			oomHyp = &report.Hypotheses[i]
			break
		}
	}
	if oomHyp == nil {
		t.Fatal("HypothesisOOMKillerCascade not found in report hypotheses")
	}
	if math.Abs(oomHyp.Score-expectedScore) > 0.001 {
		t.Errorf("expected oomHyp.Score %.2f, got %.4f", expectedScore, oomHyp.Score)
	}
	if len(oomHyp.MatchedIndicators) != 5 {
		t.Errorf("expected 5 matched indicators, got %d", len(oomHyp.MatchedIndicators))
	}
}

func TestRCAScoringMatrix_PartialEvidenceWeightedScoring(t *testing.T) {
	// DmesgOOMDetected (weight 5.0) and MemoryPressureCond (weight 4.0)
	// Weights: Dmesg(5.0) + Cgroup(4.0) + Swap(3.0) + MemPressure(4.0) + OOMEvents(5.0) = Total 21.0
	// Matched: 5.0 + 4.0 = 9.0
	// Score: 9.0 / 21.0 = ~0.42857
	signals := sre.TelemetrySignals{
		DmesgOOMDetected:   true,
		MemoryPressureCond: true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report == nil {
		t.Fatal("expected non-nil DynamicRCAReport")
	}

	var oomHyp *sre.HypothesisEvaluation
	for i := range report.Hypotheses {
		if report.Hypotheses[i].Hypothesis == sre.HypothesisOOMKillerCascade {
			oomHyp = &report.Hypotheses[i]
			break
		}
	}
	if oomHyp == nil {
		t.Fatal("HypothesisOOMKillerCascade not found")
	}

	expectedScore := 9.0 / 21.0
	if math.Abs(oomHyp.Score-expectedScore) > 0.005 {
		t.Errorf("expected score %.4f, got %.4f", expectedScore, oomHyp.Score)
	}
	if math.Abs(oomHyp.Confidence-expectedScore) > 0.005 {
		t.Errorf("expected confidence %.4f, got %.4f", expectedScore, oomHyp.Confidence)
	}
}

func TestRCAScoringMatrix_DiskIOHangAndDState(t *testing.T) {
	signals := sre.TelemetrySignals{
		DStateProcessCount: 16,
		DiskAwaitMs:        6200.0,
		FilesystemReadOnly: true,
		DiskPressureCond:   true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report.PrimaryCause != sre.HypothesisDiskIOHangAndDState {
		t.Errorf("expected PrimaryCause %s, got %s", sre.HypothesisDiskIOHangAndDState, report.PrimaryCause)
	}
	if report.Confidence != 0.99 {
		t.Errorf("expected confidence 0.99 for all indicators matched, got %.4f", report.Confidence)
	}
}

func TestRCAScoringMatrix_PLEGRuntimeDeadlock(t *testing.T) {
	signals := sre.TelemetrySignals{
		PLEGUnhealthyLog:        true,
		ContainerdSocketTimeout: true,
		KubeletPortTimeout:      true,
		NodeReadyUnknown:        true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report.PrimaryCause != sre.HypothesisPLEGRuntimeDeadlock {
		t.Errorf("expected PrimaryCause %s, got %s", sre.HypothesisPLEGRuntimeDeadlock, report.PrimaryCause)
	}
	if report.Confidence != 0.99 {
		t.Errorf("expected confidence 0.99, got %.4f", report.Confidence)
	}
}

func TestRCAScoringMatrix_PIDExhaustionForkBomb(t *testing.T) {
	signals := sre.TelemetrySignals{
		AllocatedPIDPercent: 96.5,
		PIDPressureCond:     true,
		ForkRejectedLog:     true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report.PrimaryCause != sre.HypothesisPIDExhaustionForkBomb {
		t.Errorf("expected PrimaryCause %s, got %s", sre.HypothesisPIDExhaustionForkBomb, report.PrimaryCause)
	}
	if report.Confidence != 0.99 {
		t.Errorf("expected confidence 0.99, got %.4f", report.Confidence)
	}
}

func TestRCAScoringMatrix_HardwarePowerOrKernelPanic(t *testing.T) {
	signals := sre.TelemetrySignals{
		PingTimeout:    true,
		AgentTimeout:   true,
		IPMIPowerOff:   true,
		IPMISensorTrip: true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report.PrimaryCause != sre.HypothesisHardwarePowerOrKernelPanic {
		t.Errorf("expected PrimaryCause %s, got %s", sre.HypothesisHardwarePowerOrKernelPanic, report.PrimaryCause)
	}
	if report.Confidence != 0.99 {
		t.Errorf("expected confidence 0.99, got %.4f", report.Confidence)
	}
}

func TestRCAScoringMatrix_HealthyNode(t *testing.T) {
	signals := sre.TelemetrySignals{
		NodeReadyTrue: true,
	}

	report := sre.EvaluateRCAMatrix(signals)
	if report.PrimaryCause != sre.HypothesisNodeHealthy {
		t.Errorf("expected PrimaryCause %s, got %s", sre.HypothesisNodeHealthy, report.PrimaryCause)
	}
	if report.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %.4f", report.Confidence)
	}
}
