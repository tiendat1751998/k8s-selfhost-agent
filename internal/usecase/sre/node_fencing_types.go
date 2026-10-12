package sre

import (
	"errors"
)

// ErrPowerIsolationRequired is returned when storage fencing cannot proceed without confirmed power isolation (STONITH).
var ErrPowerIsolationRequired = errors.New("cannot fence storage: power isolation / STONITH must be confirmed to prevent split-brain data corruption")

// NodeConditionSummary holds a condensed representation of a node's condition.
type NodeConditionSummary struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// PodSummary summarizes a workload pod scheduled on the fenced node.
type PodSummary struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Kind      string   `json:"kind,omitempty"`
	Volumes   []string `json:"volumes,omitempty"`
	HasPVC    bool     `json:"has_pvc"`
}

// VolumeAttachmentSummary summarizes a CSI VolumeAttachment attached to the fenced node.
type VolumeAttachmentSummary struct {
	AttachmentName       string `json:"attachment_name"`
	PersistentVolumeName string `json:"persistent_volume_name"`
	PVCName              string `json:"pvc_name,omitempty"`
	StorageClass         string `json:"storage_class,omitempty"`
	Attached             bool   `json:"attached"`
	Status               string `json:"status"`
}

// RCAAnalysis contains root-cause-analysis heuristic conclusions and confidence scores.
type RCAAnalysis struct {
	ProbableCause  string   `json:"probable_cause"`
	Confidence     float64  `json:"confidence"`
	Recommendation string   `json:"recommendation"`
	Signals        []string `json:"signals,omitempty"`
}

// NodeFencingPreflight represents the complete pre-flight audit for node remediation and storage fencing.
type NodeFencingPreflight struct {
	ClusterID                string                    `json:"cluster_id"`
	NodeName                 string                    `json:"node_name"`
	NodeStatus               string                    `json:"node_status"`
	Conditions               []NodeConditionSummary    `json:"conditions"`
	LeaseHealthy             bool                      `json:"lease_healthy"`
	LastHeartbeatTime        string                    `json:"last_heartbeat_time,omitempty"`
	TotalNodes               int                       `json:"total_nodes"`
	WorkerNodes              int                       `json:"worker_nodes"`
	MasterNodes              int                       `json:"master_nodes"`
	IsSingleWorkerCluster    bool                      `json:"is_single_worker_cluster"`
	MasterNodeName           string                    `json:"master_node_name,omitempty"`
	MasterHasNoScheduleTaint bool                      `json:"master_has_no_schedule_taint"`
	RCA                      RCAAnalysis               `json:"rca"`
	StatefulPods             []PodSummary              `json:"stateful_pods"`
	StatelessPods            []PodSummary              `json:"stateless_pods"`
	DaemonSetPods            []PodSummary              `json:"daemonset_pods"`
	VolumeInventory          []VolumeAttachmentSummary `json:"volume_inventory"`
}

// FencingExecutionOptions configures the storage fencing and failover sequence.
type FencingExecutionOptions struct {
	ConfirmPowerIsolated   bool `json:"confirm_power_isolated"`
	EmergencyUntaintMaster bool `json:"emergency_untaint_master"`
	ForceVolumeDetach      bool `json:"force_volume_detach"`
	VolumeDetachTimeoutSec int  `json:"volume_detach_timeout_sec,omitempty"`
}

// FencingStep captures an individual milestone during fencing execution.
type FencingStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Timestamp  string `json:"timestamp"`
	DurationMs int64  `json:"duration_ms"`
	Details    string `json:"details,omitempty"`
}

// FencingExecutionReport records the outcome and RTO of the fencing procedure.
type FencingExecutionReport struct {
	ClusterID       string        `json:"cluster_id"`
	NodeName        string        `json:"node_name"`
	Success         bool          `json:"success"`
	StartTime       string        `json:"start_time"`
	CompletionTime  string        `json:"completion_time"`
	RTODurationMs   int64         `json:"rto_duration_ms"`
	Steps           []FencingStep `json:"steps"`
	EvictedPods     []string      `json:"evicted_pods"`
	MasterUntainted bool          `json:"master_untainted"`
}
