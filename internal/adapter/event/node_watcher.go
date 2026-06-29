// Package event provides the Node Watcher for detecting NodeNotReady incidents.
package event

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

// NodeWatcher watches Kubernetes node statuses to detect NodeNotReady incidents.
type NodeWatcher struct {
	clientset *kubernetes.Clientset
	handler   IncidentHandler
}

// NewNodeWatcher creates a new NodeWatcher.
func NewNodeWatcher(clientset *kubernetes.Clientset, handler IncidentHandler) *NodeWatcher {
	return &NodeWatcher{
		clientset: clientset,
		handler:   handler,
	}
}

// Start begins watching for node status changes. It blocks until the context is cancelled.
func (nw *NodeWatcher) Start(ctx context.Context) error {
	log := logger.WithContext(ctx)
	log.Info("starting node watcher")

	for {
		select {
		case <-ctx.Done():
			log.Info("node watcher stopped")
			return nil
		default:
		}

		if err := nw.watchNodes(ctx); err != nil {
			log.Error("node watch error, restarting",
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (nw *NodeWatcher) watchNodes(ctx context.Context) error {
	log := logger.WithContext(ctx)

	watcher, err := nw.clientset.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return fmt.Errorf("creating node watcher: %w", err)
	}
	defer watcher.Stop()

	log.Info("watching nodes for NotReady conditions")

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("node watcher channel closed")
			}
			if event.Type == watch.Modified {
				node, ok := event.Object.(*corev1.Node)
				if !ok {
					continue
				}
				nw.processNode(ctx, node)
			}
		}
	}
}

func (nw *NodeWatcher) processNode(ctx context.Context, node *corev1.Node) {
	log := logger.WithContext(ctx)

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
			inc, err := incident.New(
				"default",
				"kube-system",
				node.Name,
				incident.TypeNodeNotReady,
				incident.SeverityCritical,
				fmt.Sprintf("Node '%s' is not ready: %s (reason: %s)", node.Name, condition.Message, condition.Reason),
			)
			if err != nil {
				log.Error("failed to create node incident", zap.Error(err))
				return
			}

			inc.AddRawData("node_name", node.Name)
			inc.AddRawData("condition_reason", condition.Reason)
			inc.AddRawData("condition_message", condition.Message)

			if err := nw.handler(ctx, inc); err != nil {
				log.Error("failed to handle node incident",
					zap.String("node", node.Name),
					zap.Error(err),
				)
			}
		}
	}
}
