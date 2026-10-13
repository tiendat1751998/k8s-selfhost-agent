package sre

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	domainsre "github.com/datdt/k8sselfhost/internal/domain/sre"
)

// AnalyzeNodeRCA inspects node conditions, lease status, and cluster events to infer root cause.
func AnalyzeNodeRCA(node *corev1.Node, leaseHealthy bool, eventMessages []string) RCAAnalysis {
	if node == nil {
		return RCAAnalysis{
			ProbableCause:  "NODE_NOT_FOUND",
			Confidence:     0.0,
			Recommendation: "Target node could not be located in cluster state.",
		}
	}

	var signals domainsre.TelemetrySignals

	for _, cond := range node.Status.Conditions {
		switch cond.Type {
		case corev1.NodeReady:
			if cond.Status == corev1.ConditionTrue {
				signals.NodeReadyTrue = true
			} else {
				if cond.Status == corev1.ConditionUnknown {
					signals.NodeReadyUnknown = true
				}
				reasonUpper := strings.ToUpper(cond.Reason)
				msgUpper := strings.ToUpper(cond.Message)
				if strings.Contains(reasonUpper, "PLEG") || strings.Contains(msgUpper, "PLEG") {
					signals.PLEGUnhealthyLog = true
				}
				if cond.Reason == "KubeletStopped" || cond.Reason == "NodeStatusUnknown" {
					signals.KubeletPortTimeout = true
				}
			}
		case corev1.NodeMemoryPressure:
			if cond.Status == corev1.ConditionTrue {
				signals.MemoryPressureCond = true
			}
		case corev1.NodeDiskPressure:
			if cond.Status == corev1.ConditionTrue {
				signals.DiskPressureCond = true
			}
		case corev1.NodePIDPressure:
			if cond.Status == corev1.ConditionTrue {
				signals.PIDPressureCond = true
			}
		default:
			if strings.Contains(strings.ToUpper(string(cond.Type)), "PLEG") && cond.Status == corev1.ConditionTrue {
				signals.PLEGUnhealthyLog = true
			}
		}
	}

	for _, msg := range eventMessages {
		upper := strings.ToUpper(msg)
		if strings.Contains(upper, "OOM") || strings.Contains(upper, "OUT OF MEMORY") || strings.Contains(upper, "SYSTEMOOM") {
			signals.DmesgOOMDetected = true
			signals.OOMKilledEventsDetected = true
		}
		if strings.Contains(upper, "PLEG") {
			signals.PLEGUnhealthyLog = true
		}
		if strings.Contains(upper, "CONTAINERD") || strings.Contains(upper, "RUNTIME TIMEOUT") {
			signals.ContainerdSocketTimeout = true
		}
		if strings.Contains(upper, "DISK") || strings.Contains(upper, "IOWAIT") {
			signals.DiskPressureCond = true
			signals.DiskAwaitMs = 5000.0
		}
		if strings.Contains(upper, "READ-ONLY") {
			signals.FilesystemReadOnly = true
		}
		if strings.Contains(upper, "HEARTBEAT") || strings.Contains(upper, "PING TIMEOUT") || strings.Contains(upper, "UNRESPONSIVE") {
			signals.PingTimeout = true
		}
		if strings.Contains(upper, "AGENT TIMEOUT") {
			signals.AgentTimeout = true
		}
		if strings.Contains(upper, "FORK") || strings.Contains(upper, "EAGAIN") {
			signals.ForkRejectedLog = true
		}
	}

	if !leaseHealthy {
		if !signals.MemoryPressureCond && !signals.DiskPressureCond && !signals.PIDPressureCond && !signals.PLEGUnhealthyLog {
			signals.PingTimeout = true
			signals.AgentTimeout = true
		}
	}

	report := domainsre.EvaluateRCAMatrix(signals)
	return RCAAnalysis{
		ProbableCause:  report.PrimaryCause,
		Confidence:     report.Confidence,
		Recommendation: report.Recommendation,
		Signals:        report.MatchingEvidence,
		DynamicReport:  report,
	}
}

// IsMasterNode returns true if the node is labeled as a master/control-plane node.
func IsMasterNode(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		return true
	}
	if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	return false
}

// MasterHasNoScheduleTaint checks if the node has a control-plane NoSchedule/NoExecute taint.
func MasterHasNoScheduleTaint(node *corev1.Node) bool {
	if node == nil {
		return false
	}
	for _, t := range node.Spec.Taints {
		if (t.Key == "node-role.kubernetes.io/control-plane" || t.Key == "node-role.kubernetes.io/master") &&
			(t.Effect == corev1.TaintEffectNoSchedule || t.Effect == corev1.TaintEffectNoExecute) {
			return true
		}
	}
	return false
}

// ClassifyPod determines if a pod is a DaemonSet or Stateful pod (StatefulSet / has PVC).
func ClassifyPod(pod *corev1.Pod) (isDaemonSet, isStateful bool) {
	if pod == nil {
		return false, false
	}
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true, false
		}
		if owner.Kind == "StatefulSet" {
			return false, true
		}
	}
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			return false, true
		}
	}
	return false, false
}

// PreflightScan audits node conditions, topology, workloads, and volumes before fencing.
func (c *RemediationController) PreflightScan(ctx context.Context, clusterID, nodeName string) (*NodeFencingPreflight, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("nodeName is required")
	}

	client, err := c.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("resolving k8s client for cluster %s: %w", clusterID, err)
	}
	if client == nil || isNilK8sClient(client) {
		return nil, fmt.Errorf("kubernetes client unavailable for cluster '%s'", clusterID)
	}

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting node %s: %w", nodeName, err)
	}

	condSummaries := make([]NodeConditionSummary, 0, len(node.Status.Conditions))
	nodeStatus := "Unknown"
	for _, cond := range node.Status.Conditions {
		condSummaries = append(condSummaries, NodeConditionSummary{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
		})
		if cond.Type == corev1.NodeReady {
			switch cond.Status {
			case corev1.ConditionTrue:
				nodeStatus = "Ready"
			case corev1.ConditionFalse:
				nodeStatus = "NotReady"
			default:
				nodeStatus = "Unknown"
			}
		}
	}

	leaseHealthy := (nodeStatus == "Ready")
	lastHeartbeat := ""
	if lease, err := client.CoordinationV1().Leases("kube-node-lease").Get(ctx, nodeName, metav1.GetOptions{}); err == nil && lease != nil {
		if lease.Spec.RenewTime != nil {
			lastHeartbeat = lease.Spec.RenewTime.UTC().Format(time.RFC3339)
			leaseHealthy = time.Since(lease.Spec.RenewTime.Time) < 2*time.Minute
		}
	}

	nodeList, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing nodes in cluster %s: %w", clusterID, err)
	}
	masterCount := 0
	workerCount := 0
	masterName := ""
	masterNoSchedule := false
	for _, n := range nodeList.Items {
		if IsMasterNode(&n) {
			masterCount++
			if masterName == "" {
				masterName = n.Name
				masterNoSchedule = MasterHasNoScheduleTaint(&n)
			}
		} else {
			workerCount++
		}
	}
	isSingleWorker := (workerCount == 1)

	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing pods for node %s: %w", nodeName, err)
	}

	statefulPods := make([]PodSummary, 0)
	statelessPods := make([]PodSummary, 0)
	daemonSetPods := make([]PodSummary, 0)

	for _, pod := range podList.Items {
		if pod.Spec.NodeName != nodeName {
			continue
		}
		if _, isMirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
			continue
		}
		volNames := make([]string, 0, len(pod.Spec.Volumes))
		hasPVC := false
		for _, v := range pod.Spec.Volumes {
			volNames = append(volNames, v.Name)
			if v.PersistentVolumeClaim != nil {
				hasPVC = true
			}
		}
		summary := PodSummary{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Volumes:   volNames,
			HasPVC:    hasPVC,
		}
		isDS, isStateful := ClassifyPod(&pod)
		if isDS {
			summary.Kind = "DaemonSet"
			daemonSetPods = append(daemonSetPods, summary)
		} else if isStateful {
			summary.Kind = "StatefulSet"
			statefulPods = append(statefulPods, summary)
		} else {
			summary.Kind = "Stateless"
			statelessPods = append(statelessPods, summary)
		}
	}

	volumeInventory := make([]VolumeAttachmentSummary, 0)
	if vaList, err := client.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{}); err == nil {
		for _, va := range vaList.Items {
			if va.Spec.NodeName != nodeName {
				continue
			}
			pvName := ""
			if va.Spec.Source.PersistentVolumeName != nil {
				pvName = *va.Spec.Source.PersistentVolumeName
			}
			pvcName := ""
			scName := ""
			if pvName != "" {
				if pv, err := client.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{}); err == nil && pv != nil {
					if pv.Spec.ClaimRef != nil {
						pvcName = pv.Spec.ClaimRef.Name
					}
					scName = pv.Spec.StorageClassName
				}
			}
			st := "Detaching"
			if va.Status.Attached {
				st = "Attached"
			}
			volumeInventory = append(volumeInventory, VolumeAttachmentSummary{
				AttachmentName:       va.Name,
				PersistentVolumeName: pvName,
				PVCName:              pvcName,
				StorageClass:         scName,
				Attached:             va.Status.Attached,
				Status:               st,
			})
		}
	}

	eventMsgs := make([]string, 0)
	if evtList, err := client.CoreV1().Events("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, evt := range evtList.Items {
			if evt.InvolvedObject.Name == nodeName {
				eventMsgs = append(eventMsgs, evt.Message)
			}
		}
	}
	rca := AnalyzeNodeRCA(node, leaseHealthy, eventMsgs)

	var cbStatus *CircuitBreakerStatus
	if c.circuitBreaker != nil {
		unreadyCount := 0
		for _, n := range nodeList.Items {
			for _, cond := range n.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
					unreadyCount++
					break
				}
			}
		}
		cbStatus, _ = c.circuitBreaker.EvaluateBlastRadius(clusterID, unreadyCount, len(nodeList.Items))
	}

	var probeReport *NodeProbeReport
	if c.rescueManager != nil {
		probeReport, _ = c.rescueManager.ProbeNodeHealth(ctx, client, nodeName)
	}

	var admissionPlan *CapacityAdmissionPlan
	if c.capacityEngine != nil && masterName != "" {
		rawWorkerPods := make([]corev1.Pod, 0)
		for _, p := range podList.Items {
			if p.Spec.NodeName == nodeName {
				isDS, _ := ClassifyPod(&p)
				if !isDS {
					rawWorkerPods = append(rawWorkerPods, p)
				}
			}
		}
		admissionPlan, _ = c.capacityEngine.EvaluateMasterAdmission(ctx, client, masterName, rawWorkerPods)
	}

	return &NodeFencingPreflight{
		ClusterID:                clusterID,
		NodeName:                 nodeName,
		NodeStatus:               nodeStatus,
		Conditions:               condSummaries,
		LeaseHealthy:             leaseHealthy,
		LastHeartbeatTime:        lastHeartbeat,
		TotalNodes:               len(nodeList.Items),
		WorkerNodes:              workerCount,
		MasterNodes:              masterCount,
		IsSingleWorkerCluster:    isSingleWorker,
		MasterNodeName:           masterName,
		MasterHasNoScheduleTaint: masterNoSchedule,
		RCA:                      rca,
		StatefulPods:             statefulPods,
		StatelessPods:            statelessPods,
		DaemonSetPods:            daemonSetPods,
		VolumeInventory:          volumeInventory,
		CircuitBreaker:           cbStatus,
		ProbeReport:              probeReport,
		CapacityAdmissionPlan:    admissionPlan,
	}, nil
}
