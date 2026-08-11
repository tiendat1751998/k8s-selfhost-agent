// Package event provides the Kubernetes Event Watcher that detects incidents.
package event

import (
	"context"
	"fmt"
	"sync"
	"time"


	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// IncidentHandler is a callback function that processes detected incidents.
type IncidentHandler func(ctx context.Context, inc *incident.Incident) error

// EventWatcher watches Kubernetes events and pod statuses to detect incidents.
type EventWatcher struct {
	clientset  *kubernetes.Clientset
	handler    IncidentHandler
	namespaces []string
	mu         sync.RWMutex
	running    bool
}

// NewEventWatcher creates a new EventWatcher for the given namespaces.
// If namespaces is empty, it watches all namespaces.
func NewEventWatcher(clientset *kubernetes.Clientset, handler IncidentHandler, namespaces []string) *EventWatcher {
	return &EventWatcher{
		clientset:  clientset,
		handler:    handler,
		namespaces: namespaces,
	}
}

// Start begins watching for Kubernetes events. It blocks until the context is cancelled.
func (w *EventWatcher) Start(ctx context.Context) error {
	log := logger.WithContext(ctx)

	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("event watcher is already running")
	}
	w.running = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	log.Info("starting event watcher",
		zap.Strings("namespaces", w.namespaces),
	)

	var wg sync.WaitGroup

	if len(w.namespaces) == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.watchNamespace(ctx, "")
		}()
	} else {
		for _, ns := range w.namespaces {
			wg.Add(1)
			go func(namespace string) {
				defer wg.Done()
				w.watchNamespace(ctx, namespace)
			}(ns)
		}
	}

	wg.Wait()
	log.Info("event watcher stopped")
	return nil
}

func (w *EventWatcher) watchNamespace(ctx context.Context, namespace string) {
	log := logger.WithContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.watchPodEvents(ctx, namespace); err != nil {
			log.Error("pod event watch error, restarting",
				zap.String("namespace", namespace),
				zap.Error(err),
			)
			t := time.NewTimer(5 * time.Second)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
				t.Stop()
			}
		}
	}
}

func (w *EventWatcher) watchPodEvents(ctx context.Context, namespace string) error {
	log := logger.WithContext(ctx)

	watcher, err := w.clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return fmt.Errorf("creating pod watcher for namespace '%s': %w", namespace, err)
	}
	defer watcher.Stop()

	log.Info("watching pods",
		zap.String("namespace", namespaceDisplay(namespace)),
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("pod watcher channel closed")
			}
			if event.Type == watch.Modified || event.Type == watch.Added {
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}
				w.processPod(ctx, pod)
			}
		}
	}
}

func (w *EventWatcher) processPod(ctx context.Context, pod *corev1.Pod) {
	log := logger.WithContext(ctx)

	for _, cs := range pod.Status.ContainerStatuses {
		if inc := w.detectContainerIncident(pod, cs); inc != nil {
			if err := w.handler(ctx, inc); err != nil {
				log.Error("failed to handle incident",
					zap.String("pod", pod.Name),
					zap.String("namespace", pod.Namespace),
					zap.Error(err),
				)
			}
		}
	}

	for _, cs := range pod.Status.InitContainerStatuses {
		if inc := w.detectContainerIncident(pod, cs); inc != nil {
			if err := w.handler(ctx, inc); err != nil {
				log.Error("failed to handle init container incident",
					zap.String("pod", pod.Name),
					zap.String("namespace", pod.Namespace),
					zap.Error(err),
				)
			}
		}
	}

	if inc := w.detectSchedulingIncident(pod); inc != nil {
		if err := w.handler(ctx, inc); err != nil {
			log.Error("failed to handle scheduling incident",
				zap.String("pod", pod.Name),
				zap.Error(err),
			)
		}
	}
}

func (w *EventWatcher) detectContainerIncident(pod *corev1.Pod, cs corev1.ContainerStatus) *incident.Incident {
	// Check Waiting state
	if cs.State.Waiting != nil {
		reason := cs.State.Waiting.Reason
		switch reason {
		case "CrashLoopBackOff":
			return w.createIncident(pod, incident.TypeCrashLoopBackOff, incident.SeverityHigh,
				fmt.Sprintf("Container '%s' is in CrashLoopBackOff (restarts: %d)", cs.Name, cs.RestartCount))

		case "ImagePullBackOff", "ErrImagePull":
			return w.createIncident(pod, incident.TypeImagePullBackOff, incident.SeverityHigh,
				fmt.Sprintf("Container '%s' failed to pull image: %s", cs.Name, cs.State.Waiting.Message))
		}
	}

	// Check Terminated state
	if cs.State.Terminated != nil {
		reason := cs.State.Terminated.Reason
		if reason == "OOMKilled" {
			return w.createIncident(pod, incident.TypeOOMKilled, incident.SeverityCritical,
				fmt.Sprintf("Container '%s' was OOMKilled (exit code: %d)", cs.Name, cs.State.Terminated.ExitCode))
		}
	}

	// Check probe failures through restart count threshold
	if cs.RestartCount > 5 && cs.State.Waiting == nil && cs.State.Terminated == nil {
		if !cs.Ready {
			return w.createIncident(pod, incident.TypeProbeFailed, incident.SeverityMedium,
				fmt.Sprintf("Container '%s' not ready with %d restarts, possible probe failure", cs.Name, cs.RestartCount))
		}
	}

	return nil
}

func (w *EventWatcher) detectSchedulingIncident(pod *corev1.Pod) *incident.Incident {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			if condition.Reason == "Unschedulable" {
				return w.createIncident(pod, incident.TypeFailedScheduling, incident.SeverityHigh,
					fmt.Sprintf("Pod cannot be scheduled: %s", condition.Message))
			}
		}
	}
	return nil
}

func (w *EventWatcher) createIncident(pod *corev1.Pod, incType incident.Type, severity incident.Severity, message string) *incident.Incident {
	clusterName := "default"

	inc, err := incident.New(clusterName, pod.Namespace, pod.Name, incType, severity, message)
	if err != nil {
		return nil
	}

	inc.AddRawData("node_name", pod.Spec.NodeName)
	inc.AddRawData("pod_phase", string(pod.Status.Phase))
	if pod.Status.StartTime != nil {
		inc.AddRawData("start_time", pod.Status.StartTime.Format(time.RFC3339))
	}

	return inc
}

func namespaceDisplay(namespace string) string {
	if namespace == "" {
		return "all"
	}
	return namespace
}


