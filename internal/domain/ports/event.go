package ports

import (
	"context"
)

// EventPublisher defines the port for publishing events to a message broker.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload any) error
	PublishRaw(ctx context.Context, subject string, data []byte) error
}

// EventSummary holds a simplified Kubernetes event.
type EventSummary struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int32  `json:"count"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

// NodeMetrics holds basic node resource metrics.
type NodeMetrics struct {
	NodeName    string `json:"node_name"`
	CPUUsage    string `json:"cpu_usage"`
	MemoryUsage string `json:"memory_usage"`
	PodCount    int    `json:"pod_count"`
	Allocatable string `json:"allocatable"`
}

// PodMetrics holds basic pod resource metrics.
type PodMetrics struct {
	CPUUsage    string `json:"cpu_usage"`
	MemoryUsage string `json:"memory_usage"`
}

// CollectedData holds all diagnostic data gathered for an incident.
type CollectedData struct {
	PodLogs         map[string]string `json:"pod_logs"`
	Events          []EventSummary    `json:"events"`
	DeploymentYAML  string            `json:"deployment_yaml,omitempty"`
	StatefulSetYAML string            `json:"statefulset_yaml,omitempty"`
	ServiceYAML     string            `json:"service_yaml,omitempty"`
	IngressYAML     string            `json:"ingress_yaml,omitempty"`
	PodDescribe     string            `json:"pod_describe"`
	NodeMetrics     *NodeMetrics      `json:"node_metrics,omitempty"`
	PodMetrics      *PodMetrics       `json:"pod_metrics,omitempty"`
}

// DataCollector defines the port for collecting diagnostic data from Kubernetes.
type DataCollector interface {
	Collect(ctx context.Context, namespace, podName string) (*CollectedData, error)
}
