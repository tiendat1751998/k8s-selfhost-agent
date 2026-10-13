package sre

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Reason constants for capacity shedding.
const (
	ReasonMasterOOMPrevention = "MASTER_OOM_PREVENTION"
)

// Pod priority tier constants.
const (
	Tier0StatefulAndIngress = 0
	Tier1CoreAPIAndWeb      = 1
	Tier2BackgroundWorkers  = 2
	Tier3ObservabilityTests = 3
)

// AdmittedPodDecision records an admitted workload pod.
type AdmittedPodDecision struct {
	PodName          string `json:"pod_name"`
	Namespace        string `json:"namespace"`
	Tier             int    `json:"tier"`
	RequestedCPU     int64  `json:"requested_cpu_millis"`
	RequestedMemBytes int64 `json:"requested_mem_bytes"`
}

// SheddedPodDecision records a shedded workload pod to protect master from cascading OOM.
type SheddedPodDecision struct {
	PodName          string `json:"pod_name"`
	Namespace        string `json:"namespace"`
	Tier             int    `json:"tier"`
	Reason           string `json:"reason"`
	RequestedCPU     int64  `json:"requested_cpu_millis"`
	RequestedMemBytes int64 `json:"requested_mem_bytes"`
}

// CapacityAdmissionPlan details the admission and priority shedding decisions for a 1M+1W cluster.
type CapacityAdmissionPlan struct {
	MasterNodeName            string                `json:"master_node_name"`
	MasterAllocatableCPU      int64                 `json:"master_allocatable_cpu_millis"`
	MasterAllocatableMemBytes int64                 `json:"master_allocatable_mem_bytes"`
	MasterAvailableCPU        int64                 `json:"master_available_cpu_millis"`
	MasterAvailableMemBytes   int64                 `json:"master_available_mem_bytes"`
	AdmittedPods              []AdmittedPodDecision `json:"admitted_pods"`
	SheddedPods               []SheddedPodDecision  `json:"shedded_pods"`
	TotalAdmitted             int                   `json:"total_admitted"`
	TotalShedded              int                   `json:"total_shedded"`
}

// CapacityAdmissionEngine evaluates master headroom and prioritizes workload admission.
type CapacityAdmissionEngine struct{}

// NewCapacityAdmissionEngine constructs a new CapacityAdmissionEngine.
func NewCapacityAdmissionEngine() *CapacityAdmissionEngine {
	return &CapacityAdmissionEngine{}
}

// ClassifyPodTier classifies a pod into 4 priority tiers:
// Tier 0: Stateful pods (with PVCs), Ingress controllers.
// Tier 1: Core API & web services.
// Tier 2: Background workers, report processors, async jobs.
// Tier 3: Observability collectors, promtail, test runners.
func ClassifyPodTier(pod *corev1.Pod) int {
	if pod == nil {
		return Tier3ObservabilityTests
	}

	// Tier 0: Stateful (PVC / StatefulSet) or Ingress
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			return Tier0StatefulAndIngress
		}
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "StatefulSet" {
			return Tier0StatefulAndIngress
		}
	}
	nameLower := strings.ToLower(pod.Name)
	nsLower := strings.ToLower(pod.Namespace)
	if strings.Contains(nameLower, "ingress") || strings.Contains(nsLower, "ingress") {
		return Tier0StatefulAndIngress
	}

	// Tier 1: Core API & web services
	tier1Keywords := []string{"api", "web", "backend", "core", "server", "gateway", "auth"}
	for _, kw := range tier1Keywords {
		if strings.Contains(nameLower, kw) || strings.Contains(nsLower, kw) {
			return Tier1CoreAPIAndWeb
		}
	}

	// Tier 2: Background workers, report processors, async jobs
	tier2Keywords := []string{"worker", "job", "processor", "cron", "queue", "consumer", "async", "batch"}
	for _, kw := range tier2Keywords {
		if strings.Contains(nameLower, kw) || strings.Contains(nsLower, kw) {
			return Tier2BackgroundWorkers
		}
	}

	// Tier 3: Observability collectors, promtail, test runners, or other
	return Tier3ObservabilityTests
}

// CalculatePodRequests sums CPU and memory requests across all containers in a pod.
func CalculatePodRequests(pod *corev1.Pod) (cpuMillis int64, memBytes int64) {
	if pod == nil {
		return 0, 0
	}
	for _, c := range pod.Spec.Containers {
		if req := c.Resources.Requests; req != nil {
			if cpu := req.Cpu(); cpu != nil {
				cpuMillis += cpu.MilliValue()
			}
			if mem := req.Memory(); mem != nil {
				memBytes += mem.Value()
			}
		}
	}
	// Default minimum reservation if not specified (50m CPU, 64Mi RAM)
	if cpuMillis == 0 {
		cpuMillis = 50
	}
	if memBytes == 0 {
		memBytes = 64 * 1024 * 1024
	}
	return cpuMillis, memBytes
}

// EvaluateMasterAdmission computes Master allocatable resources, existing usage, and prioritizes worker pods.
func (e *CapacityAdmissionEngine) EvaluateMasterAdmission(
	ctx context.Context,
	client kubernetes.Interface,
	masterNodeName string,
	workerPods []corev1.Pod,
) (*CapacityAdmissionPlan, error) {
	if masterNodeName == "" {
		return nil, fmt.Errorf("masterNodeName is required")
	}

	masterNode, err := client.CoreV1().Nodes().Get(ctx, masterNodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting master node %s: %w", masterNodeName, err)
	}

	var allocCPU, allocMem int64
	if cpu := masterNode.Status.Allocatable.Cpu(); cpu != nil {
		allocCPU = cpu.MilliValue()
	}
	if mem := masterNode.Status.Allocatable.Memory(); mem != nil {
		allocMem = mem.Value()
	}

	// Sum requests of existing pods running on Master
	masterPods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + masterNodeName,
	})
	var usedCPU, usedMem int64
	if err == nil && masterPods != nil {
		for _, p := range masterPods.Items {
			if p.Spec.NodeName != masterNodeName {
				continue
			}
			if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
				continue
			}
			cpuReq, memReq := CalculatePodRequests(&p)
			usedCPU += cpuReq
			usedMem += memReq
		}
	}

	availCPU := allocCPU - usedCPU
	if availCPU < 0 {
		availCPU = 0
	}
	availMem := allocMem - usedMem
	if availMem < 0 {
		availMem = 0
	}

	// Sort worker pods by priority tier ascending, then name
	type scoredPod struct {
		pod      corev1.Pod
		tier     int
		cpuReq   int64
		memReq   int64
	}

	scoredList := make([]scoredPod, 0, len(workerPods))
	for _, p := range workerPods {
		t := ClassifyPodTier(&p)
		c, m := CalculatePodRequests(&p)
		scoredList = append(scoredList, scoredPod{
			pod:    p,
			tier:   t,
			cpuReq: c,
			memReq: m,
		})
	}

	sort.SliceStable(scoredList, func(i, j int) bool {
		if scoredList[i].tier != scoredList[j].tier {
			return scoredList[i].tier < scoredList[j].tier
		}
		return scoredList[i].pod.Name < scoredList[j].pod.Name
	})

	admitted := make([]AdmittedPodDecision, 0)
	shedded := make([]SheddedPodDecision, 0)
	remainingCPU := availCPU
	remainingMem := availMem
	capacityExhaustedTier := -1

	for _, sp := range scoredList {
		if capacityExhaustedTier != -1 && sp.tier >= capacityExhaustedTier {
			// Lower priority pods shedded once higher tier headroom is exhausted
			shedded = append(shedded, SheddedPodDecision{
				PodName:           sp.pod.Name,
				Namespace:         sp.pod.Namespace,
				Tier:              sp.tier,
				Reason:            ReasonMasterOOMPrevention,
				RequestedCPU:      sp.cpuReq,
				RequestedMemBytes: sp.memReq,
			})
			continue
		}

		if sp.cpuReq <= remainingCPU && sp.memReq <= remainingMem {
			admitted = append(admitted, AdmittedPodDecision{
				PodName:           sp.pod.Name,
				Namespace:         sp.pod.Namespace,
				Tier:              sp.tier,
				RequestedCPU:      sp.cpuReq,
				RequestedMemBytes: sp.memReq,
			})
			remainingCPU -= sp.cpuReq
			remainingMem -= sp.memReq
		} else {
			capacityExhaustedTier = sp.tier
			shedded = append(shedded, SheddedPodDecision{
				PodName:           sp.pod.Name,
				Namespace:         sp.pod.Namespace,
				Tier:              sp.tier,
				Reason:            ReasonMasterOOMPrevention,
				RequestedCPU:      sp.cpuReq,
				RequestedMemBytes: sp.memReq,
			})
		}
	}

	return &CapacityAdmissionPlan{
		MasterNodeName:            masterNodeName,
		MasterAllocatableCPU:      allocCPU,
		MasterAllocatableMemBytes: allocMem,
		MasterAvailableCPU:        availCPU,
		MasterAvailableMemBytes:   availMem,
		AdmittedPods:              admitted,
		SheddedPods:               shedded,
		TotalAdmitted:             len(admitted),
		TotalShedded:              len(shedded),
	}, nil
}
