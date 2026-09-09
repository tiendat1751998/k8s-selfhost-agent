package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

// ErrK8sUnavailable is returned when no Kubernetes client is available.
var ErrK8sUnavailable = fmt.Errorf("kubernetes not connected")

// ResourceRepo provides generic CRUD operations for Kubernetes resources using typed APIs.
type ResourceRepo struct {
	client        kubernetes.Interface
	clientManager *cluster.ClientManager
}

// NewResourceRepo creates a new live Kubernetes ResourceRepo.
func NewResourceRepo(client *kubernetes.Clientset, cm *cluster.ClientManager) *ResourceRepo {
	return &ResourceRepo{client: client, clientManager: cm}
}

// NewResourceRepoWithInterface creates a ResourceRepo with any kubernetes.Interface.
func NewResourceRepoWithInterface(client kubernetes.Interface, cm *cluster.ClientManager) *ResourceRepo {
	return &ResourceRepo{client: client, clientManager: cm}
}

func (r *ResourceRepo) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if r.clientManager != nil && clusterID != "" && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		if cli, err := r.clientManager.GetK8sClient(ctx, clusterID); err != nil || cli != nil {
			return cli, err
		}
	}
	if r.client != nil {
		if cs, ok := r.client.(*kubernetes.Clientset); ok && cs != nil {
			if cli, err := getClient(ctx, cs); err != nil || cli != nil {
				return cli, err
			}
		} else {
			return r.client, nil
		}
	}
	return nil, ErrK8sUnavailable
}

// ListNamespaces lists all namespaces in the specified cluster.
func (r *ResourceRepo) ListNamespaces(ctx context.Context, clusterID string) ([]corev1.Namespace, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	return list.Items, nil
}

// CreateNamespace creates a new namespace in the specified cluster.
func (r *ResourceRepo) CreateNamespace(ctx context.Context, clusterID, name string) (*corev1.Namespace, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
	created, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating namespace %s: %w", name, err)
	}
	return created, nil
}

// DeleteNamespace deletes a namespace in the specified cluster.
func (r *ResourceRepo) DeleteNamespace(ctx context.Context, clusterID, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}
	if err := client.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("deleting namespace %s: %w", name, err)
	}
	return nil
}

// ListResources lists Kubernetes resources of a given kind in the specified namespace.
func (r *ResourceRepo) ListResources(ctx context.Context, clusterID, kind, namespace string) ([]map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	k := normalizeKind(kind)
	if res, ok, err := r.listWorkloads(ctx, client, k, namespace); ok {
		return emptyIfNil(res, err)
	}
	if res, ok, err := r.listNetworking(ctx, client, k, namespace); ok {
		return emptyIfNil(res, err)
	}
	if res, ok, err := r.listStorageConfig(ctx, client, k, namespace); ok {
		return emptyIfNil(res, err)
	}
	if res, ok, err := r.listNodeOrCluster(ctx, client, k, namespace); ok {
		return emptyIfNil(res, err)
	}
	return nil, fmt.Errorf("unsupported resource kind: %s", kind)
}

// GetResource retrieves a single Kubernetes resource by kind, namespace, and name.
func (r *ResourceRepo) GetResource(ctx context.Context, clusterID, kind, namespace, name string) (map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	k := normalizeKind(kind)
	if namespace == "" && isNamespacedKind(k) {
		namespace = r.discoverNamespace(ctx, client, k, name)
	}
	if res, ok, err := r.getWorkload(ctx, client, k, namespace, name); ok {
		return res, err
	}
	if res, ok, err := r.getNetworking(ctx, client, k, namespace, name); ok {
		return res, err
	}
	if res, ok, err := r.getStorageConfig(ctx, client, k, namespace, name); ok {
		return res, err
	}
	if res, ok, err := r.getNodeOrCluster(ctx, client, k, namespace, name); ok {
		return res, err
	}
	return nil, fmt.Errorf("unsupported resource kind: %s", kind)
}

// CreateResource creates a Kubernetes resource from the manifest.
func (r *ResourceRepo) CreateResource(ctx context.Context, clusterID, kind, namespace string, manifest map[string]interface{}) (map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	k := normalizeKind(kind)
	if res, ok, err := r.createWorkload(ctx, client, k, namespace, manifest); ok {
		return res, err
	}
	if res, ok, err := r.createNetworking(ctx, client, k, namespace, manifest); ok {
		return res, err
	}
	if res, ok, err := r.createStorageConfig(ctx, client, k, namespace, manifest); ok {
		return res, err
	}
	if res, ok, err := r.createNodeOrCluster(ctx, client, k, namespace, manifest); ok {
		return res, err
	}
	return nil, fmt.Errorf("unsupported resource kind: %s", kind)
}

// UpdateResource updates an existing Kubernetes resource.
func (r *ResourceRepo) UpdateResource(ctx context.Context, clusterID, kind, namespace, name string, manifest map[string]interface{}) (map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	k := normalizeKind(kind)
	if res, ok, err := r.updateWorkload(ctx, client, k, namespace, name, manifest); ok {
		return res, err
	}
	if res, ok, err := r.updateNetworking(ctx, client, k, namespace, name, manifest); ok {
		return res, err
	}
	if res, ok, err := r.updateStorageConfig(ctx, client, k, namespace, name, manifest); ok {
		return res, err
	}
	if res, ok, err := r.updateNodeOrCluster(ctx, client, k, namespace, name, manifest); ok {
		return res, err
	}
	return nil, fmt.Errorf("unsupported resource kind: %s", kind)
}

// DeleteResource deletes a Kubernetes resource by kind, namespace, and name.
func (r *ResourceRepo) DeleteResource(ctx context.Context, clusterID, kind, namespace, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}
	k := normalizeKind(kind)
	if namespace == "" && isNamespacedKind(k) {
		namespace = r.discoverNamespace(ctx, client, k, name)
	}
	if ok, err := r.deleteWorkload(ctx, client, k, namespace, name); ok {
		return err
	}
	if ok, err := r.deleteNetworking(ctx, client, k, namespace, name); ok {
		return err
	}
	if ok, err := r.deleteStorageConfig(ctx, client, k, namespace, name); ok {
		return err
	}
	if ok, err := r.deleteNodeOrCluster(ctx, client, k, namespace, name); ok {
		return err
	}
	return fmt.Errorf("unsupported resource kind: %s", kind)
}
