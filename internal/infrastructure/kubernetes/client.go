// Package kubernetes provides Kubernetes client initialization and shared utilities.
package kubernetes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

var lastConfig *rest.Config

var impersonatedClients sync.Map

// GetLastConfig retrieves the last built rest.Config.
func GetLastConfig() *rest.Config {
	return lastConfig
}

// NewClient creates a Kubernetes clientset. It first attempts in-cluster config,
// then falls back to the kubeconfig file at the given path.
func NewClient(kubeconfigPath string) (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		if kubeconfigPath == "" {
			return nil, fmt.Errorf("not running in cluster and no kubeconfig path provided: %w", err)
		}

		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("building kubeconfig from %s: %w", kubeconfigPath, err)
		}
	}

	// Prevent K8s API server overload with rate limiting and timeouts
	cfg.QPS = 50
	cfg.Burst = 100
	cfg.Timeout = 30 * time.Second

	lastConfig = cfg

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	return clientset, nil
}

// getClient returns a clientset configured with user impersonation if a userID is present in the context.
func getClient(ctx context.Context, defaultClient *kubernetes.Clientset) (*kubernetes.Clientset, error) {
	if defaultClient == nil {
		return nil, nil
	}
	cfg := GetLastConfig()
	if cfg == nil {
		return defaultClient, nil
	}

	userID := tenancy.UserIDFromContext(ctx)
	if userID == "" || userID == "guest" {
		return defaultClient, nil
	}

	// Create impersonated config
	if cached, ok := impersonatedClients.Load(userID); ok {
		return cached.(*kubernetes.Clientset), nil
	}

	impersonatedConfig := rest.CopyConfig(cfg)
	impersonatedConfig.Impersonate = rest.ImpersonationConfig{
		UserName: userID,
	}

	client, err := kubernetes.NewForConfig(impersonatedConfig)
	if err != nil {
		return nil, fmt.Errorf("creating impersonated client: %w", err)
	}
	impersonatedClients.Store(userID, client)
	return client, nil
}
