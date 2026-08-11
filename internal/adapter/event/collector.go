// Package event provides Kubernetes data collection for incident analysis.
package event

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/ports"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// MaxLogLines is the maximum number of log lines to retrieve from a pod.
const MaxLogLines int64 = 200

// Collector gathers Kubernetes diagnostic data for incident analysis.
type Collector struct {
	clientset *kubernetes.Clientset
}

// NewCollector creates a new Kubernetes data collector.
func NewCollector(clientset *kubernetes.Clientset) *Collector {
	return &Collector{clientset: clientset}
}

// CollectedData holds all diagnostic data gathered for an incident.
type CollectedData = ports.CollectedData

// EventSummary holds a simplified Kubernetes event.
type EventSummary = ports.EventSummary

// NodeMetrics holds basic node resource metrics.
type NodeMetrics = ports.NodeMetrics

// PodMetrics holds basic pod resource metrics.
type PodMetrics = ports.PodMetrics

func (c *Collector) Collect(ctx context.Context, namespace, podName string) (*CollectedData, error) {
	log := logger.WithContext(ctx)
	data := &CollectedData{
		PodLogs: make(map[string]string),
	}

	log.Info("collecting diagnostic data",
		zap.String("namespace", namespace),
		zap.String("pod", podName),
	)

	// Create a timeout context for the entire collection process
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	eg, egCtx := errgroup.WithContext(timeoutCtx)

	// Collect pod logs
	eg.Go(func() error {
		if err := c.collectPodLogs(egCtx, namespace, podName, data); err != nil {
			log.Warn("failed to collect pod logs", zap.Error(err))
		}
		return nil
	})

	// Collect events
	eg.Go(func() error {
		if err := c.collectEvents(egCtx, namespace, podName, data); err != nil {
			log.Warn("failed to collect events", zap.Error(err))
		}
		return nil
	})

	// Collect pod describe
	eg.Go(func() error {
		if err := c.collectPodDescribe(egCtx, namespace, podName, data); err != nil {
			log.Warn("failed to collect pod describe", zap.Error(err))
		}
		return nil
	})

	// Collect owner resources (deployment/statefulset YAML)
	eg.Go(func() error {
		if err := c.collectOwnerResources(egCtx, namespace, podName, data); err != nil {
			log.Warn("failed to collect owner resources", zap.Error(err))
		}
		return nil
	})

	// Collect service YAML
	eg.Go(func() error {
		if err := c.collectServiceYAML(egCtx, namespace, podName, data); err != nil {
			log.Warn("failed to collect service YAML", zap.Error(err))
		}
		return nil
	})

	// Collect ingress YAML
	eg.Go(func() error {
		if err := c.collectIngressYAML(egCtx, namespace, data); err != nil {
			log.Warn("failed to collect ingress YAML", zap.Error(err))
		}
		return nil
	})

	// Collect node metrics
	eg.Go(func() error {
		if err := c.collectNodeMetrics(egCtx, namespace, podName, data); err != nil {
			log.Warn("failed to collect node metrics", zap.Error(err))
		}
		return nil
	})

	_ = eg.Wait()

	log.Info("diagnostic data collection complete",
		zap.Int("log_containers", len(data.PodLogs)),
		zap.Int("events", len(data.Events)),
	)

	return data, nil
}

func (c *Collector) collectPodLogs(ctx context.Context, namespace, podName string, data *CollectedData) error {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod: %w", err)
	}

	for _, container := range pod.Spec.Containers {
		logs, logErr := c.getContainerLogs(ctx, namespace, podName, container.Name, false)
		if logErr != nil {
			data.PodLogs[container.Name] = fmt.Sprintf("error: %v", logErr)
			continue
		}
		data.PodLogs[container.Name] = logs

		// Also try to get previous container logs if there were restarts
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == container.Name && cs.RestartCount > 0 {
				prevLogs, prevErr := c.getContainerLogs(ctx, namespace, podName, container.Name, true)
				if prevErr == nil {
					data.PodLogs[container.Name+"-previous"] = prevLogs
				}
				break
			}
		}
	}

	return nil
}

func (c *Collector) getContainerLogs(ctx context.Context, namespace, podName, containerName string, previous bool) (string, error) {
	tailLines := MaxLogLines
	opts := &corev1.PodLogOptions{
		Container: containerName,
		TailLines: &tailLines,
		Previous:  previous,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("opening log stream: %w", err)
	}
	defer stream.Close()

	limitReader := io.LimitReader(stream, 10*1024*1024) // 10MB limit
	logBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return "", fmt.Errorf("reading log stream: %w", err)
	}

	return string(logBytes), nil
}

func (c *Collector) collectEvents(ctx context.Context, namespace, podName string, data *CollectedData) error {
	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
	})
	if err != nil {
		return fmt.Errorf("listing events: %w", err)
	}

	for _, event := range events.Items {
		data.Events = append(data.Events, EventSummary{
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Count:     event.Count,
			FirstSeen: event.FirstTimestamp.Format("2006-01-02T15:04:05Z"),
			LastSeen:  event.LastTimestamp.Format("2006-01-02T15:04:05Z"),
		})
	}

	return nil
}

func (c *Collector) collectPodDescribe(ctx context.Context, namespace, podName string, data *CollectedData) error {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod for describe: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s\n", pod.Name))
	sb.WriteString(fmt.Sprintf("Namespace: %s\n", pod.Namespace))
	sb.WriteString(fmt.Sprintf("Node: %s\n", pod.Spec.NodeName))
	sb.WriteString(fmt.Sprintf("Phase: %s\n", pod.Status.Phase))

	if pod.Status.StartTime != nil {
		sb.WriteString(fmt.Sprintf("StartTime: %s\n", pod.Status.StartTime.Format("2006-01-02T15:04:05Z")))
	}

	sb.WriteString("\nConditions:\n")
	for _, cond := range pod.Status.Conditions {
		sb.WriteString(fmt.Sprintf("  %s: %s (Reason: %s, Message: %s)\n",
			cond.Type, cond.Status, cond.Reason, cond.Message))
	}

	sb.WriteString("\nContainers:\n")
	for _, cs := range pod.Status.ContainerStatuses {
		sb.WriteString(fmt.Sprintf("  %s: Ready=%t, RestartCount=%d\n",
			cs.Name, cs.Ready, cs.RestartCount))
		if cs.State.Waiting != nil {
			sb.WriteString(fmt.Sprintf("    Waiting: %s - %s\n", cs.State.Waiting.Reason, cs.State.Waiting.Message))
		}
		if cs.State.Terminated != nil {
			sb.WriteString(fmt.Sprintf("    Terminated: %s (ExitCode: %d)\n", cs.State.Terminated.Reason, cs.State.Terminated.ExitCode))
		}
	}

	data.PodDescribe = sb.String()
	return nil
}

func (c *Collector) collectOwnerResources(ctx context.Context, namespace, podName string, data *CollectedData) error {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod for owner: %w", err)
	}

	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "ReplicaSet":
			if err := c.collectDeploymentFromRS(ctx, namespace, owner.Name, data); err != nil {
				return err
			}
		case "StatefulSet":
			if err := c.collectStatefulSet(ctx, namespace, owner.Name, data); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Collector) collectDeploymentFromRS(ctx context.Context, namespace, rsName string, data *CollectedData) error {
	rs, err := c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, rsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting replicaset: %w", err)
	}

	for _, owner := range rs.OwnerReferences {
		if owner.Kind == "Deployment" {
			deploy, getErr := c.clientset.AppsV1().Deployments(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if getErr != nil {
				return fmt.Errorf("getting deployment: %w", getErr)
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Name: %s\n", deploy.Name))
			sb.WriteString(fmt.Sprintf("Namespace: %s\n", deploy.Namespace))
			sb.WriteString(fmt.Sprintf("Replicas: %d/%d\n", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas))
			sb.WriteString(fmt.Sprintf("Strategy: %s\n", deploy.Spec.Strategy.Type))

			for _, container := range deploy.Spec.Template.Spec.Containers {
				sb.WriteString(fmt.Sprintf("\nContainer: %s\n", container.Name))
				sb.WriteString(fmt.Sprintf("  Image: %s\n", container.Image))
				if container.Resources.Limits != nil {
					sb.WriteString(fmt.Sprintf("  Limits: CPU=%s, Memory=%s\n",
						container.Resources.Limits.Cpu().String(),
						container.Resources.Limits.Memory().String()))
				}
				if container.Resources.Requests != nil {
					sb.WriteString(fmt.Sprintf("  Requests: CPU=%s, Memory=%s\n",
						container.Resources.Requests.Cpu().String(),
						container.Resources.Requests.Memory().String()))
				}
			}

			data.DeploymentYAML = sb.String()
			break
		}
	}

	return nil
}

func (c *Collector) collectStatefulSet(ctx context.Context, namespace, stsName string, data *CollectedData) error {
	sts, err := c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting statefulset: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Name: %s\n", sts.Name))
	sb.WriteString(fmt.Sprintf("Namespace: %s\n", sts.Namespace))
	sb.WriteString(fmt.Sprintf("Replicas: %d/%d\n", sts.Status.ReadyReplicas, *sts.Spec.Replicas))

	for _, container := range sts.Spec.Template.Spec.Containers {
		sb.WriteString(fmt.Sprintf("\nContainer: %s\n", container.Name))
		sb.WriteString(fmt.Sprintf("  Image: %s\n", container.Image))
	}

	data.StatefulSetYAML = sb.String()
	return nil
}

func (c *Collector) collectServiceYAML(ctx context.Context, namespace, podName string, data *CollectedData) error {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod for service: %w", err)
	}

	services, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}

	var sb strings.Builder
	for _, svc := range services.Items {
		if matchesSelector(pod.Labels, svc.Spec.Selector) {
			sb.WriteString(fmt.Sprintf("Service: %s\n", svc.Name))
			sb.WriteString(fmt.Sprintf("  Type: %s\n", svc.Spec.Type))
			sb.WriteString(fmt.Sprintf("  ClusterIP: %s\n", svc.Spec.ClusterIP))
			for _, port := range svc.Spec.Ports {
				sb.WriteString(fmt.Sprintf("  Port: %s %d -> %d\n", port.Name, port.Port, port.TargetPort.IntValue()))
			}
			sb.WriteString("\n")
		}
	}

	data.ServiceYAML = sb.String()
	return nil
}

func (c *Collector) collectIngressYAML(ctx context.Context, namespace string, data *CollectedData) error {
	ingresses, err := c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing ingresses: %w", err)
	}

	var sb strings.Builder
	for _, ing := range ingresses.Items {
		sb.WriteString(fmt.Sprintf("Ingress: %s\n", ing.Name))
		for _, rule := range ing.Spec.Rules {
			sb.WriteString(fmt.Sprintf("  Host: %s\n", rule.Host))
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					sb.WriteString(fmt.Sprintf("    Path: %s -> %s:%d\n",
						path.Path,
						path.Backend.Service.Name,
						path.Backend.Service.Port.Number))
				}
			}
		}
		sb.WriteString("\n")
	}

	data.IngressYAML = sb.String()
	return nil
}

func (c *Collector) collectNodeMetrics(ctx context.Context, namespace, podName string, data *CollectedData) error {
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod for node metrics: %w", err)
	}

	if pod.Spec.NodeName == "" {
		return nil
	}

	node, err := c.clientset.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node: %w", err)
	}

	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", pod.Spec.NodeName),
	})
	if err != nil {
		return fmt.Errorf("listing pods on node: %w", err)
	}

	data.NodeMetrics = &NodeMetrics{
		NodeName:    node.Name,
		CPUUsage:    node.Status.Allocatable.Cpu().String(),
		MemoryUsage: node.Status.Allocatable.Memory().String(),
		PodCount:    len(pods.Items),
		Allocatable: fmt.Sprintf("CPU: %s, Memory: %s, Pods: %s",
			node.Status.Allocatable.Cpu().String(),
			node.Status.Allocatable.Memory().String(),
			node.Status.Allocatable.Pods().String()),
	}

	return nil
}

// matchesSelector checks if a pod's labels match a service selector.
func matchesSelector(podLabels, selector map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}
