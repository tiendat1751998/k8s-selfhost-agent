package kubernetes

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DiscoveryAdapter struct {}

func NewDiscoveryAdapter() *DiscoveryAdapter {
	return &DiscoveryAdapter{}
}

func getClientFromKubeconfig(kubeconfig []byte) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parsing kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}
	return clientset, nil
}

func (d *DiscoveryAdapter) ValidateConnectivity(ctx context.Context, kubeconfig []byte) error {
	clientset, err := getClientFromKubeconfig(kubeconfig)
	if err != nil {
		return err
	}

	_, err = clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("listing namespaces: %w", err)
	}
	return nil
}

func (d *DiscoveryAdapter) DiscoverResources(ctx context.Context, kubeconfig []byte) (map[string]interface{}, error) {
	clientset, err := getClientFromKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	resources := make(map[string]interface{})

	versionInfo, err := clientset.Discovery().ServerVersion()
	if err == nil {
		resources["version"] = versionInfo.GitVersion
	}

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		resources["nodes_count"] = len(nodes.Items)
	}

	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil {
		resources["namespaces_count"] = len(namespaces.Items)
	}

	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		resources["deployments_count"] = len(deployments.Items)
	}

	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err == nil {
		resources["services_count"] = len(services.Items)
	}

	return resources, nil
}
