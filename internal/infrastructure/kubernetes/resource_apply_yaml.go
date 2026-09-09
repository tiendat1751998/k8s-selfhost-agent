package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

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

func isNamespacedKind(kind string) bool {
	switch normalizeKind(kind) {
	case "nodes", "persistentvolumes", "storageclasses", "namespaces":
		return false
	default:
		return true
	}
}

func (r *ResourceRepo) discoverNamespace(ctx context.Context, client kubernetes.Interface, kind, name string) string {
	if client == nil || strings.TrimSpace(name) == "" {
		return "default"
	}

	k := normalizeKind(kind)
	switch k {
	case "pods":
		if list, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "configmaps":
		if list, err := client.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "secrets":
		if list, err := client.CoreV1().Secrets("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "deployments":
		if list, err := client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "services":
		if list, err := client.CoreV1().Services("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "ingresses":
		if list, err := client.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "statefulsets":
		if list, err := client.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "daemonsets":
		if list, err := client.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "jobs":
		if list, err := client.BatchV1().Jobs("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "cronjobs":
		if list, err := client.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "persistentvolumeclaims":
		if list, err := client.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "networkpolicies":
		if list, err := client.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "serviceaccounts":
		if list, err := client.CoreV1().ServiceAccounts("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "horizontalpodautoscalers":
		if list, err := client.AutoscalingV2().HorizontalPodAutoscalers("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	case "events":
		if list, err := client.CoreV1().Events("").List(ctx, metav1.ListOptions{}); err == nil {
			for _, item := range list.Items {
				if item.Name == name {
					return item.Namespace
				}
			}
		}
	}

	return "default"
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
			return fmt.Errorf("decoding YAML document: %w", err)
		}
		if len(raw) == 0 {
			continue
		}

		kind, _ := raw["kind"].(string)
		if kind == "" {
			return fmt.Errorf("manifest missing required field 'kind'")
		}

		metadata, _ := raw["metadata"].(map[string]interface{})
		if metadata == nil {
			return fmt.Errorf("manifest missing required field 'metadata'")
		}

		name, _ := metadata["name"].(string)
		if name == "" {
			return fmt.Errorf("manifest missing required field 'metadata.name'")
		}

		// Use namespace from manifest if present, otherwise fall back to parameter
		ns := namespace
		if mNs, ok := metadata["namespace"].(string); ok && mNs != "" {
			ns = mNs
		}
		if ns == "" && isNamespacedKind(kind) {
			ns = "default"
		}

		// Try GET first: if exists -> UPDATE, if 404 -> CREATE
		existing, getErr := r.GetResource(ctx, clusterID, kind, ns, name)
		if getErr == nil && existing != nil {
			// Resource exists: preserve resourceVersion for update
			if existingMeta, ok := existing["metadata"].(map[string]interface{}); ok {
				if rv, ok := existingMeta["resourceVersion"].(string); ok && rv != "" {
					metadata["resourceVersion"] = rv
				}
			}
			if _, err := r.UpdateResource(ctx, clusterID, kind, ns, name, raw); err != nil {
				return fmt.Errorf("updating %s/%s: %w", kind, name, err)
			}
		} else if getErr != nil && (k8serrors.IsNotFound(getErr) || strings.Contains(getErr.Error(), "not found")) {
			// Resource doesn't exist: create it
			if _, err := r.CreateResource(ctx, clusterID, kind, ns, raw); err != nil {
				return fmt.Errorf("creating %s/%s: %w", kind, name, err)
			}
		} else if getErr != nil {
			return fmt.Errorf("checking existence of %s/%s: %w", kind, name, getErr)
		}
	}

	return nil
}

func emptyIfNil(res []map[string]interface{}, err error) ([]map[string]interface{}, error) {
	if err != nil || res != nil {
		return res, err
	}
	return []map[string]interface{}{}, nil
}
