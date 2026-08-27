package kubernetes

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
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
	return &ResourceRepo{
		client:        client,
		clientManager: cm,
	}
}

// NewResourceRepoWithInterface creates a ResourceRepo with any kubernetes.Interface.
func NewResourceRepoWithInterface(client kubernetes.Interface, cm *cluster.ClientManager) *ResourceRepo {
	return &ResourceRepo{
		client:        client,
		clientManager: cm,
	}
}

func (r *ResourceRepo) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	if r.clientManager != nil && clusterID != "" && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		cli, err := r.clientManager.GetK8sClient(ctx, clusterID)
		if err != nil {
			return nil, err
		}
		if cli != nil {
			return cli, nil
		}
	}

	if r.client != nil {
		if cs, ok := r.client.(*kubernetes.Clientset); ok {
			if cs != nil {
				cli, err := getClient(ctx, cs)
				if err != nil {
					return nil, err
				}
				if cli != nil {
					return cli, nil
				}
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
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
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

// normalizeKind normalizes resource kind strings to standard plural format.
func normalizeKind(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "pod", "pods", "po":
		return "pods"
	case "configmap", "configmaps", "cm":
		return "configmaps"
	case "secret", "secrets":
		return "secrets"
	case "deployment", "deployments", "deploy":
		return "deployments"
	case "service", "services", "svc":
		return "services"
	case "ingress", "ingresses", "ing":
		return "ingresses"
	case "statefulset", "statefulsets", "sts":
		return "statefulsets"
	case "daemonset", "daemonsets", "ds":
		return "daemonsets"
	case "job", "jobs":
		return "jobs"
	case "cronjob", "cronjobs", "cj":
		return "cronjobs"
	case "persistentvolumeclaim", "persistentvolumeclaims", "pvc", "pvcs":
		return "persistentvolumeclaims"
	case "persistentvolume", "persistentvolumes", "pv":
		return "persistentvolumes"
	case "networkpolicy", "networkpolicies", "netpol":
		return "networkpolicies"
	case "serviceaccount", "serviceaccounts", "sa":
		return "serviceaccounts"
	case "horizontalpodautoscaler", "horizontalpodautoscalers", "hpa":
		return "horizontalpodautoscalers"
	case "storageclass", "storageclasses", "sc":
		return "storageclasses"
	case "node", "nodes", "no":
		return "nodes"
	case "event", "events", "ev":
		return "events"
	default:
		return k
	}
}

func toMap(obj interface{}, defaultAPIVersion, defaultKind string) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}

	if res["kind"] == nil || res["kind"] == "" {
		res["kind"] = defaultKind
	}
	if res["apiVersion"] == nil || res["apiVersion"] == "" {
		res["apiVersion"] = defaultAPIVersion
	}
	return res, nil
}

func fromMap(manifest map[string]interface{}, target interface{}) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func maskSecretMap(m map[string]interface{}) {
	if data, ok := m["data"].(map[string]interface{}); ok {
		masked := make(map[string]interface{}, len(data))
		for k := range data {
			masked[k] = "***"
		}
		m["data"] = masked
	}
	if stringData, ok := m["stringData"].(map[string]interface{}); ok {
		masked := make(map[string]interface{}, len(stringData))
		for k := range stringData {
			masked[k] = "***"
		}
		m["stringData"] = masked
	}
}

// ListResources lists Kubernetes resources of a given kind in the specified namespace.
func (r *ResourceRepo) ListResources(ctx context.Context, clusterID, kind, namespace string) ([]map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	k := normalizeKind(kind)
	var results []map[string]interface{}

	switch k {
	case "pods":
		list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing pods: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Pod")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "configmaps":
		list, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing configmaps: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "ConfigMap")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "secrets":
		list, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing secrets: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Secret")
			if err != nil {
				return nil, err
			}
			maskSecretMap(m)
			results = append(results, m)
		}

	case "deployments":
		list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing deployments: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "apps/v1", "Deployment")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "services":
		list, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing services: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Service")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "ingresses":
		list, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing ingresses: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "networking.k8s.io/v1", "Ingress")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "statefulsets":
		list, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing statefulsets: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "apps/v1", "StatefulSet")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "daemonsets":
		list, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing daemonsets: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "apps/v1", "DaemonSet")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "jobs":
		list, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing jobs: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "batch/v1", "Job")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "cronjobs":
		list, err := client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing cronjobs: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "batch/v1", "CronJob")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "persistentvolumeclaims":
		list, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing persistentvolumeclaims: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "PersistentVolumeClaim")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "persistentvolumes":
		list, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing persistentvolumes: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "PersistentVolume")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "networkpolicies":
		list, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing networkpolicies: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "networking.k8s.io/v1", "NetworkPolicy")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "serviceaccounts":
		list, err := client.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing serviceaccounts: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "ServiceAccount")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "horizontalpodautoscalers":
		list, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing horizontalpodautoscalers: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "autoscaling/v2", "HorizontalPodAutoscaler")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "storageclasses":
		list, err := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing storageclasses: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "storage.k8s.io/v1", "StorageClass")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "nodes":
		list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing nodes: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Node")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	case "events":
		list, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("listing events: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Event")
			if err != nil {
				return nil, err
			}
			results = append(results, m)
		}

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// GetResource retrieves a single Kubernetes resource by kind, namespace, and name.
func (r *ResourceRepo) GetResource(ctx context.Context, clusterID, kind, namespace, name string) (map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	k := normalizeKind(kind)

	switch k {
	case "pods":
		obj, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "Pod")

	case "configmaps":
		obj, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "ConfigMap")

	case "secrets":
		obj, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		m, err := toMap(obj, "v1", "Secret")
		if err != nil {
			return nil, err
		}
		maskSecretMap(m)
		return m, nil

	case "deployments":
		obj, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "apps/v1", "Deployment")

	case "services":
		obj, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "Service")

	case "ingresses":
		obj, err := client.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "networking.k8s.io/v1", "Ingress")

	case "statefulsets":
		obj, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "apps/v1", "StatefulSet")

	case "daemonsets":
		obj, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "apps/v1", "DaemonSet")

	case "jobs":
		obj, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "batch/v1", "Job")

	case "cronjobs":
		obj, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "batch/v1", "CronJob")

	case "persistentvolumeclaims":
		obj, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "PersistentVolumeClaim")

	case "persistentvolumes":
		obj, err := client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "PersistentVolume")

	case "networkpolicies":
		obj, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "networking.k8s.io/v1", "NetworkPolicy")

	case "serviceaccounts":
		obj, err := client.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "ServiceAccount")

	case "horizontalpodautoscalers":
		obj, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "autoscaling/v2", "HorizontalPodAutoscaler")

	case "storageclasses":
		obj, err := client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "storage.k8s.io/v1", "StorageClass")

	case "nodes":
		obj, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "Node")

	case "events":
		obj, err := client.CoreV1().Events(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return toMap(obj, "v1", "Event")

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

// CreateResource creates a Kubernetes resource from the manifest.
func (r *ResourceRepo) CreateResource(ctx context.Context, clusterID, kind, namespace string, manifest map[string]interface{}) (map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	k := normalizeKind(kind)

	switch k {
	case "configmaps":
		var obj corev1.ConfigMap
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding configmap manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().ConfigMaps(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating configmap: %w", err)
		}
		return toMap(created, "v1", "ConfigMap")

	case "secrets":
		var obj corev1.Secret
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding secret manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().Secrets(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating secret: %w", err)
		}
		m, err := toMap(created, "v1", "Secret")
		if err != nil {
			return nil, err
		}
		maskSecretMap(m)
		return m, nil

	case "deployments":
		var obj appsv1.Deployment
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding deployment manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AppsV1().Deployments(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating deployment: %w", err)
		}
		return toMap(created, "apps/v1", "Deployment")

	case "services":
		var obj corev1.Service
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding service manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().Services(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating service: %w", err)
		}
		return toMap(created, "v1", "Service")

	case "ingresses":
		var obj networkingv1.Ingress
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding ingress manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.NetworkingV1().Ingresses(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating ingress: %w", err)
		}
		return toMap(created, "networking.k8s.io/v1", "Ingress")

	case "statefulsets":
		var obj appsv1.StatefulSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding statefulset manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AppsV1().StatefulSets(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating statefulset: %w", err)
		}
		return toMap(created, "apps/v1", "StatefulSet")

	case "daemonsets":
		var obj appsv1.DaemonSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding daemonset manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AppsV1().DaemonSets(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating daemonset: %w", err)
		}
		return toMap(created, "apps/v1", "DaemonSet")

	case "jobs":
		var obj batchv1.Job
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding job manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.BatchV1().Jobs(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating job: %w", err)
		}
		return toMap(created, "batch/v1", "Job")

	case "cronjobs":
		var obj batchv1.CronJob
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding cronjob manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.BatchV1().CronJobs(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating cronjob: %w", err)
		}
		return toMap(created, "batch/v1", "CronJob")

	case "persistentvolumeclaims":
		var obj corev1.PersistentVolumeClaim
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding pvc manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().PersistentVolumeClaims(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating pvc: %w", err)
		}
		return toMap(created, "v1", "PersistentVolumeClaim")

	case "persistentvolumes":
		var obj corev1.PersistentVolume
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding pv manifest: %w", err)
		}
		created, err := client.CoreV1().PersistentVolumes().Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating persistentvolume: %w", err)
		}
		return toMap(created, "v1", "PersistentVolume")

	case "networkpolicies":
		var obj networkingv1.NetworkPolicy
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding networkpolicy manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.NetworkingV1().NetworkPolicies(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating networkpolicy: %w", err)
		}
		return toMap(created, "networking.k8s.io/v1", "NetworkPolicy")

	case "serviceaccounts":
		var obj corev1.ServiceAccount
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding serviceaccount manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().ServiceAccounts(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating serviceaccount: %w", err)
		}
		return toMap(created, "v1", "ServiceAccount")

	case "horizontalpodautoscalers":
		var obj autoscalingv2.HorizontalPodAutoscaler
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding hpa manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AutoscalingV2().HorizontalPodAutoscalers(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating horizontalpodautoscaler: %w", err)
		}
		return toMap(created, "autoscaling/v2", "HorizontalPodAutoscaler")

	case "storageclasses":
		var obj storagev1.StorageClass
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding storageclass manifest: %w", err)
		}
		created, err := client.StorageV1().StorageClasses().Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating storageclass: %w", err)
		}
		return toMap(created, "storage.k8s.io/v1", "StorageClass")

	case "nodes":
		var obj corev1.Node
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding node manifest: %w", err)
		}
		created, err := client.CoreV1().Nodes().Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("creating node: %w", err)
		}
		return toMap(created, "v1", "Node")

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

// UpdateResource updates an existing Kubernetes resource.
func (r *ResourceRepo) UpdateResource(ctx context.Context, clusterID, kind, namespace, name string, manifest map[string]interface{}) (map[string]interface{}, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	k := normalizeKind(kind)

	switch k {
	case "configmaps":
		var obj corev1.ConfigMap
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding configmap manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().ConfigMaps(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating configmap %s: %w", name, err)
		}
		return toMap(updated, "v1", "ConfigMap")

	case "secrets":
		var obj corev1.Secret
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding secret manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().Secrets(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating secret %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "Secret")
		if err != nil {
			return nil, err
		}
		maskSecretMap(m)
		return m, nil

	case "deployments":
		var obj appsv1.Deployment
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding deployment manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AppsV1().Deployments(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating deployment %s: %w", name, err)
		}
		return toMap(updated, "apps/v1", "Deployment")

	case "services":
		var obj corev1.Service
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding service manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().Services(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating service %s: %w", name, err)
		}
		return toMap(updated, "v1", "Service")

	case "ingresses":
		var obj networkingv1.Ingress
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding ingress manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.NetworkingV1().Ingresses(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating ingress %s: %w", name, err)
		}
		return toMap(updated, "networking.k8s.io/v1", "Ingress")

	case "statefulsets":
		var obj appsv1.StatefulSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding statefulset manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AppsV1().StatefulSets(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating statefulset %s: %w", name, err)
		}
		return toMap(updated, "apps/v1", "StatefulSet")

	case "daemonsets":
		var obj appsv1.DaemonSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding daemonset manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AppsV1().DaemonSets(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating daemonset %s: %w", name, err)
		}
		return toMap(updated, "apps/v1", "DaemonSet")

	case "jobs":
		var obj batchv1.Job
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding job manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.BatchV1().Jobs(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating job %s: %w", name, err)
		}
		return toMap(updated, "batch/v1", "Job")

	case "cronjobs":
		var obj batchv1.CronJob
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding cronjob manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.BatchV1().CronJobs(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating cronjob %s: %w", name, err)
		}
		return toMap(updated, "batch/v1", "CronJob")

	case "persistentvolumeclaims":
		var obj corev1.PersistentVolumeClaim
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding pvc manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().PersistentVolumeClaims(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating pvc %s: %w", name, err)
		}
		return toMap(updated, "v1", "PersistentVolumeClaim")

	case "persistentvolumes":
		var obj corev1.PersistentVolume
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding pv manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		updated, err := client.CoreV1().PersistentVolumes().Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating persistentvolume %s: %w", name, err)
		}
		return toMap(updated, "v1", "PersistentVolume")

	case "networkpolicies":
		var obj networkingv1.NetworkPolicy
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding networkpolicy manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.NetworkingV1().NetworkPolicies(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating networkpolicy %s: %w", name, err)
		}
		return toMap(updated, "networking.k8s.io/v1", "NetworkPolicy")

	case "serviceaccounts":
		var obj corev1.ServiceAccount
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding serviceaccount manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().ServiceAccounts(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating serviceaccount %s: %w", name, err)
		}
		return toMap(updated, "v1", "ServiceAccount")

	case "horizontalpodautoscalers":
		var obj autoscalingv2.HorizontalPodAutoscaler
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding hpa manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AutoscalingV2().HorizontalPodAutoscalers(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating horizontalpodautoscaler %s: %w", name, err)
		}
		return toMap(updated, "autoscaling/v2", "HorizontalPodAutoscaler")

	case "storageclasses":
		var obj storagev1.StorageClass
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding storageclass manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		updated, err := client.StorageV1().StorageClasses().Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating storageclass %s: %w", name, err)
		}
		return toMap(updated, "storage.k8s.io/v1", "StorageClass")

	case "nodes":
		var obj corev1.Node
		if err := fromMap(manifest, &obj); err != nil {
			return nil, fmt.Errorf("decoding node manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		updated, err := client.CoreV1().Nodes().Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("updating node %s: %w", name, err)
		}
		return toMap(updated, "v1", "Node")

	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

// DeleteResource deletes a Kubernetes resource by kind, namespace, and name.
func (r *ResourceRepo) DeleteResource(ctx context.Context, clusterID, kind, namespace, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	k := normalizeKind(kind)

	switch k {
	case "pods":
		return client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "configmaps":
		return client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "secrets":
		return client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "deployments":
		return client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "services":
		return client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "ingresses":
		return client.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "statefulsets":
		return client.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "daemonsets":
		return client.AppsV1().DaemonSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "jobs":
		return client.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "cronjobs":
		return client.BatchV1().CronJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "persistentvolumeclaims":
		return client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "persistentvolumes":
		return client.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
	case "networkpolicies":
		return client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "serviceaccounts":
		return client.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "horizontalpodautoscalers":
		return client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "storageclasses":
		return client.StorageV1().StorageClasses().Delete(ctx, name, metav1.DeleteOptions{})
	case "nodes":
		return client.CoreV1().Nodes().Delete(ctx, name, metav1.DeleteOptions{})
	case "events":
		return client.CoreV1().Events(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	default:
		return fmt.Errorf("unsupported resource kind: %s", kind)
	}
}

// ApplyYAML parses and applies one or more YAML resource documents.
func (r *ResourceRepo) ApplyYAML(ctx context.Context, clusterID, namespace string, yamlContent []byte) error {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlContent), 4096)

	for {
		var raw map[string]interface{}
		err := decoder.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("decoding yaml: %w", err)
		}
		if len(raw) == 0 {
			continue
		}

		kind, _ := raw["kind"].(string)
		if kind == "" {
			continue
		}

		meta, ok := raw["metadata"].(map[string]interface{})
		if !ok || meta == nil {
			return fmt.Errorf("resource %s missing metadata", kind)
		}

		name, _ := meta["name"].(string)
		if name == "" {
			return fmt.Errorf("resource %s missing metadata.name", kind)
		}

		docNs, _ := meta["namespace"].(string)
		if docNs == "" {
			docNs = namespace
		}
		if docNs != "" {
			meta["namespace"] = docNs
		}

		k := normalizeKind(kind)

		// Check if resource already exists
		existing, err := r.GetResource(ctx, clusterID, k, docNs, name)
		if err == nil && existing != nil {
			// Copy resourceVersion if present to avoid conflict
			if existingMeta, ok := existing["metadata"].(map[string]interface{}); ok {
				if rv, ok := existingMeta["resourceVersion"].(string); ok && rv != "" {
					meta["resourceVersion"] = rv
				}
			}
			if _, err := r.UpdateResource(ctx, clusterID, k, docNs, name, raw); err != nil {
				return fmt.Errorf("updating %s %s: %w", kind, name, err)
			}
		} else {
			if k8serrors.IsNotFound(err) || existing == nil {
				if _, err := r.CreateResource(ctx, clusterID, k, docNs, raw); err != nil {
					return fmt.Errorf("creating %s %s: %w", kind, name, err)
				}
			} else {
				return fmt.Errorf("checking %s %s: %w", kind, name, err)
			}
		}
	}

	return nil
}

// CordonNode sets node.Spec.Unschedulable to true.
func (r *ResourceRepo) CordonNode(ctx context.Context, clusterID, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	node.Spec.Unschedulable = true
	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("cordoning node %s: %w", name, err)
	}
	return nil
}

// UncordonNode sets node.Spec.Unschedulable to false.
func (r *ResourceRepo) UncordonNode(ctx context.Context, clusterID, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	node.Spec.Unschedulable = false
	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("uncordoning node %s: %w", name, err)
	}
	return nil
}

// DrainNode cordons the node and gracefully deletes pods running on it.
func (r *ResourceRepo) DrainNode(ctx context.Context, clusterID, name string, gracePeriodSeconds int64, ignoreDaemonSets bool) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	// 1. Cordon the node
	if err := r.CordonNode(ctx, clusterID, name); err != nil {
		return fmt.Errorf("cordoning node during drain: %w", err)
	}

	// 2. List pods running on the node
	podList, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		return fmt.Errorf("listing pods on node %s: %w", name, err)
	}

	// 3. Delete pods gracefully
	var deleteOpts metav1.DeleteOptions
	if gracePeriodSeconds >= 0 {
		deleteOpts.GracePeriodSeconds = &gracePeriodSeconds
	}

	for _, p := range podList.Items {
		if p.Spec.NodeName != "" && p.Spec.NodeName != name {
			continue
		}
		// Skip mirror pods
		if _, isMirror := p.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
			continue
		}
		// Skip DaemonSet pods when ignoreDaemonSets is true
		isDaemonSet := false
		for _, ref := range p.OwnerReferences {
			if strings.EqualFold(ref.Kind, "DaemonSet") {
				isDaemonSet = true
				break
			}
		}
		if isDaemonSet && ignoreDaemonSets {
			continue
		}

		if err := client.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, deleteOpts); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("deleting pod %s/%s during drain: %w", p.Namespace, p.Name, err)
		}
	}

	return nil
}

// UpdateNodeTaints updates taints on the specified node.
func (r *ResourceRepo) UpdateNodeTaints(ctx context.Context, clusterID, name string, taints []corev1.Taint) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	node.Spec.Taints = taints
	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating taints for node %s: %w", name, err)
	}
	return nil
}

// UpdateNodeLabels updates labels on the specified node.
func (r *ResourceRepo) UpdateNodeLabels(ctx context.Context, clusterID, name string, labels map[string]string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	node, err := client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", name, err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	for k, v := range labels {
		node.Labels[k] = v
	}

	_, err = client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating labels for node %s: %w", name, err)
	}
	return nil
}

// ScaleDeployment scales a Deployment to the specified number of replicas.
func (r *ResourceRepo) ScaleDeployment(ctx context.Context, clusterID, namespace, name string, replicas int32) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s: %w", name, err)
	}

	deploy.Spec.Replicas = &replicas
	if _, err := client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating scale for deployment %s: %w", name, err)
	}
	return nil
}

// RestartDeployment triggers a rolling restart of a Deployment by updating its restartedAt annotation.
func (r *ResourceRepo) RestartDeployment(ctx context.Context, clusterID, namespace, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s: %w", name, err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)

	if _, err := client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("restarting deployment %s: %w", name, err)
	}
	return nil
}

// ScaleStatefulSet scales a StatefulSet to the specified number of replicas.
func (r *ResourceRepo) ScaleStatefulSet(ctx context.Context, clusterID, namespace, name string, replicas int32) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	sts, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting statefulset %s: %w", name, err)
	}

	sts.Spec.Replicas = &replicas
	if _, err := client.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating scale for statefulset %s: %w", name, err)
	}
	return nil
}

// RestartDaemonSet triggers a rolling restart of a DaemonSet by updating its restartedAt annotation.
func (r *ResourceRepo) RestartDaemonSet(ctx context.Context, clusterID, namespace, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting daemonset %s: %w", name, err)
	}

	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)

	if _, err := client.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("restarting ddemonset %s: %w", name, err)
	}
	return nil
}

func randomSuffix(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		num, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return fmt.Sprintf("%d", time.Now().UnixNano()%100000)
		}
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

// TriggerCronJob triggers a manual run of a CronJob by creating a Job from its JobTemplate.
func (r *ResourceRepo) TriggerCronJob(ctx context.Context, clusterID, namespace, name string) (*batchv1.Job, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	cronJob, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting cronjob %s: %w", name, err)
	}

	randSuffix := randomSuffix(5)
	jobName := fmt.Sprintf("%s-manual-%s", name, randSuffix)
	if len(jobName) > 63 {
		maxBase := 63 - len(fmt.Sprintf("-manual-%s", randSuffix))
		if maxBase > 0 && len(name) > maxBase {
			jobName = fmt.Sprintf("%s-manual-%s", name[:maxBase], randSuffix)
		}
	}

	jobMeta := metav1.ObjectMeta{
		Name:        jobName,
		Namespace:   namespace,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
	}
	for k, v := range cronJob.Spec.JobTemplate.ObjectMeta.Labels {
		jobMeta.Labels[k] = v
	}
	for k, v := range cronJob.Spec.JobTemplate.ObjectMeta.Annotations {
		jobMeta.Annotations[k] = v
	}
	jobMeta.Annotations["cronjob.kubernetes.io/instantiate"] = "manual"

	job := &batchv1.Job{
		ObjectMeta: jobMeta,
		Spec:       cronJob.Spec.JobTemplate.Spec,
	}

	createdJob, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating manual job for cronjob %s: %w", name, err)
	}
	return createdJob, nil
}

// SuspendCronJob suspends or resumes a CronJob.
func (r *ResourceRepo) SuspendCronJob(ctx context.Context, clusterID, namespace, name string, suspend bool) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	cronJob, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting cronjob %s: %w", name, err)
	}

	cronJob.Spec.Suspend = &suspend
	if _, err := client.BatchV1().CronJobs(namespace).Update(ctx, cronJob, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating suspend for cronjob %s: %w", name, err)
	}
	return nil
}
