// Package sre defines domain entities and models for Site Reliability Engineering and Root Cause Analysis.
package sre

import (
	"math"
)

// Failure Hypothesis Constants.
const (
	HypothesisOOMKillerCascade           = "OOM_KILLER_CASCADE"
	HypothesisDiskIOHangAndDState        = "DISK_IO_HANG_AND_DSTATE"
	HypothesisPLEGRuntimeDeadlock        = "PLEG_RUNTIME_DEADLOCK"
	HypothesisPIDExhaustionForkBomb      = "PID_EXHAUSTION_FORK_BOMB"
	HypothesisHardwarePowerOrKernelPanic = "HARDWARE_POWER_OR_KERNEL_PANIC"
	HypothesisNodeHealthy                = "NODE_HEALTHY"
	HypothesisUnknown                    = "UNKNOWN"
)

// Indicator evaluates a single symptom and returns whether it was detected and its description.
type Indicator struct {
	Name     string
	Weight   float64
	Detected bool
	Evidence string
}

// TelemetrySignals collects observable symptoms across host kernel, cgroups, Kubelet, and runtime.
type TelemetrySignals struct {
	// Node readiness
	NodeReadyTrue    bool
	NodeReadyUnknown bool

	// OOM / Memory Pressure
	DmesgOOMDetected        bool
	CgroupMemoryPercent     float64
	SwapPercent             float64
	MemoryPressureCond      bool
	OOMKilledEventsDetected bool

	// Disk / I/O Hang
	DStateProcessCount int
	DiskAwaitMs        float64
	FilesystemReadOnly bool
	DiskPressureCond   bool

	// PLEG / Runtime Deadlock
	PLEGUnhealthyLog        bool
	ContainerdSocketTimeout bool
	KubeletPortTimeout      bool

	// PID Exhaustion
	AllocatedPIDPercent float64
	PIDPressureCond     bool
	ForkRejectedLog     bool

	// Hardware Power / Kernel Panic
	PingTimeout    bool
	AgentTimeout   bool
	IPMIPowerOff   bool
	IPMISensorTrip bool
}

// HypothesisEvaluation contains the mathematical score and matching indicators for a hypothesis.
type HypothesisEvaluation struct {
	Hypothesis        string   `json:"hypothesis"`
	Score             float64  `json:"score"`
	Confidence        float64  `json:"confidence"`
	MatchedWeight     float64  `json:"matched_weight"`
	TotalWeight       float64  `json:"total_weight"`
	MatchedIndicators []string `json:"matched_indicators"`
	Recommendation    string   `json:"recommendation"`
}

// DynamicRCAReport provides evidence-based root cause analysis without hardcoded heuristics.
type DynamicRCAReport struct {
	PrimaryCause     string                 `json:"primary_cause"`
	Confidence       float64                `json:"confidence"`
	Hypotheses       []HypothesisEvaluation `json:"hypotheses"`
	MatchingEvidence []string               `json:"matching_evidence"`
	Recommendation   string                 `json:"recommendation"`
}

// EvaluateRCAMatrix calculates dynamic evidence scores for all 5 failure hypotheses:
// Score(F_k) = sum(w_i * detected) / sum(w_i)
// Confidence = min(0.99, Score(F_k))
func EvaluateRCAMatrix(signals TelemetrySignals) *DynamicRCAReport {
	evaluations := []HypothesisEvaluation{
		evaluateOOMKillerCascade(signals),
		evaluateDiskIOHangAndDState(signals),
		evaluatePLEGRuntimeDeadlock(signals),
		evaluatePIDExhaustionForkBomb(signals),
		evaluateHardwarePowerOrKernelPanic(signals),
	}

	var bestHyp *HypothesisEvaluation
	allEvidence := make([]string, 0)

	for i := range evaluations {
		eval := &evaluations[i]
		allEvidence = append(allEvidence, eval.MatchedIndicators...)
		if bestHyp == nil || eval.Score > bestHyp.Score {
			bestHyp = eval
		}
	}

	if bestHyp == nil || bestHyp.Score <= 0.0 {
		if signals.NodeReadyTrue {
			return &DynamicRCAReport{
				PrimaryCause:     HypothesisNodeHealthy,
				Confidence:       1.00,
				Hypotheses:       evaluations,
				MatchingEvidence: []string{"NodeReady: True"},
				Recommendation:   "Node is operational and reports healthy conditions.",
			}
		}
		return &DynamicRCAReport{
			PrimaryCause:     HypothesisUnknown,
			Confidence:       0.0,
			Hypotheses:       evaluations,
			MatchingEvidence: allEvidence,
			Recommendation:   "Insufficient signal evidence to determine failure root cause.",
		}
	}

	return &DynamicRCAReport{
		PrimaryCause:     bestHyp.Hypothesis,
		Confidence:       bestHyp.Confidence,
		Hypotheses:       evaluations,
		MatchingEvidence: bestHyp.MatchedIndicators,
		Recommendation:   bestHyp.Recommendation,
	}
}

func computeHypothesisScore(hypothesis, recommendation string, indicators []Indicator) HypothesisEvaluation {
	var totalWeight, matchedWeight float64
	matched := make([]string, 0, len(indicators))

	for _, ind := range indicators {
		totalWeight += ind.Weight
		if ind.Detected {
			matchedWeight += ind.Weight
			if ind.Evidence != "" {
				matched = append(matched, ind.Evidence)
			} else {
				matched = append(matched, ind.Name)
			}
		}
	}

	var score float64
	if totalWeight > 0 {
		score = matchedWeight / totalWeight
	}

	conf := score
	if conf > 0.99 {
		conf = 0.99
	}
	conf = math.Round(conf*10000) / 10000

	return HypothesisEvaluation{
		Hypothesis:        hypothesis,
		Score:             score,
		Confidence:        conf,
		MatchedWeight:     matchedWeight,
		TotalWeight:       totalWeight,
		MatchedIndicators: matched,
		Recommendation:    recommendation,
	}
}

func evaluateOOMKillerCascade(s TelemetrySignals) HypothesisEvaluation {
	indicators := []Indicator{
		{Name: "dmesg/journald OOM", Weight: 5.0, Detected: s.DmesgOOMDetected, Evidence: "Kernel dmesg/journald logged Out-Of-Memory killer invocations"},
		{Name: "cgroup memory >= 98%", Weight: 4.0, Detected: s.CgroupMemoryPercent >= 98.0, Evidence: "cgroup memory utilization exceeded 98%"},
		{Name: "swap >= 95%", Weight: 3.0, Detected: s.SwapPercent >= 95.0, Evidence: "Swap utilization exceeded 95%"},
		{Name: "MemoryPressure condition", Weight: 4.0, Detected: s.MemoryPressureCond, Evidence: "Node status reports MemoryPressure: True"},
		{Name: "OOMKilled events", Weight: 5.0, Detected: s.OOMKilledEventsDetected, Evidence: "K8s event stream contains OOMKilled container events"},
	}
	return computeHypothesisScore(
		HypothesisOOMKillerCascade,
		"Assign memory limits to workload pods and evict memory hog containers before node fencing.",
		indicators,
	)
}

func evaluateDiskIOHangAndDState(s TelemetrySignals) HypothesisEvaluation {
	indicators := []Indicator{
		{Name: "D-state processes >= 15", Weight: 4.0, Detected: s.DStateProcessCount >= 15, Evidence: "High uninterrupted D-state process count (>=15)"},
		{Name: "disk await >= 5000ms", Weight: 4.0, Detected: s.DiskAwaitMs >= 5000.0, Evidence: "Storage I/O await latency exceeded 5000ms"},
		{Name: "filesystem read-only remount", Weight: 5.0, Detected: s.FilesystemReadOnly, Evidence: "Filesystem remounted as Read-Only due to underlying storage block errors"},
		{Name: "DiskPressure condition", Weight: 4.0, Detected: s.DiskPressureCond, Evidence: "Node status reports DiskPressure: True"},
	}
	return computeHypothesisScore(
		HypothesisDiskIOHangAndDState,
		"Inspect underlying storage volume controllers and purge unattached disk artifacts.",
		indicators,
	)
}

func evaluatePLEGRuntimeDeadlock(s TelemetrySignals) HypothesisEvaluation {
	indicators := []Indicator{
		{Name: "PLEG unhealthy log", Weight: 5.0, Detected: s.PLEGUnhealthyLog, Evidence: "Kubelet logs indicate PLEG is not healthy"},
		{Name: "containerd.sock probe timeout", Weight: 4.0, Detected: s.ContainerdSocketTimeout, Evidence: "CRI containerd.sock probe timed out"},
		{Name: "Kubelet 10250 timeout while host alive", Weight: 4.0, Detected: s.KubeletPortTimeout, Evidence: "Kubelet port 10250 unresponsive while host OS ping responds"},
		{Name: "NodeReady Unknown", Weight: 3.0, Detected: s.NodeReadyUnknown, Evidence: "NodeReady condition status is Unknown"},
	}
	return computeHypothesisScore(
		HypothesisPLEGRuntimeDeadlock,
		"Attempt graceful restart of containerd and kubelet daemons before initiating node storage fencing.",
		indicators,
	)
}

func evaluatePIDExhaustionForkBomb(s TelemetrySignals) HypothesisEvaluation {
	indicators := []Indicator{
		{Name: "allocated PIDs >= 95% of pid_max", Weight: 5.0, Detected: s.AllocatedPIDPercent >= 95.0, Evidence: "Allocated process threads exceeded 95% of pid_max limit"},
		{Name: "PIDPressure condition", Weight: 4.0, Detected: s.PIDPressureCond, Evidence: "Node status reports PIDPressure: True"},
		{Name: "fork rejected log", Weight: 5.0, Detected: s.ForkRejectedLog, Evidence: "Kernel logged fork rejection / EAGAIN resource temporarily unavailable"},
	}
	return computeHypothesisScore(
		HypothesisPIDExhaustionForkBomb,
		"Enforce container pids-limit in kubelet/cgroups and kill fork-bomb processes.",
		indicators,
	)
}

func evaluateHardwarePowerOrKernelPanic(s TelemetrySignals) HypothesisEvaluation {
	indicators := []Indicator{
		{Name: "ping timeout", Weight: 4.0, Detected: s.PingTimeout, Evidence: "Host ICMP ping completely unreachable"},
		{Name: "agent timeout", Weight: 4.0, Detected: s.AgentTimeout, Evidence: "Host telemetry daemon/agent unreachable"},
		{Name: "IPMI power off", Weight: 5.0, Detected: s.IPMIPowerOff, Evidence: "IPMI/BMC reported chassis power off"},
		{Name: "IPMI SEL sensor trip", Weight: 5.0, Detected: s.IPMISensorTrip, Evidence: "IPMI System Event Log reported critical hardware sensor failure"},
	}
	return computeHypothesisScore(
		HypothesisHardwarePowerOrKernelPanic,
		"Inspect host IPMI/BMC console for kernel panic or hardware power failure; initiate STONITH.",
		indicators,
	)
}
