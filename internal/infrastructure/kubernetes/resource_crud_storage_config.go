package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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

func (r *ResourceRepo) listStorageConfig(ctx context.Context, client kubernetes.Interface, kind, namespace string) ([]map[string]interface{}, bool, error) {
	var results []map[string]interface{}
	switch kind {
	case "configmaps":
		list, err := client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing configmaps: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "ConfigMap")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "secrets":
		list, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing secrets: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Secret")
			if err != nil {
				return nil, true, err
			}
			maskSecretMap(m)
			results = append(results, m)
		}
		return results, true, nil

	case "persistentvolumeclaims":
		list, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing persistentvolumeclaims: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "PersistentVolumeClaim")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "persistentvolumes":
		list, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing persistentvolumes: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "PersistentVolume")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "storageclasses":
		list, err := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing storageclasses: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "storage.k8s.io/v1", "StorageClass")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil
	}

	return nil, false, nil
}

func (r *ResourceRepo) getStorageConfig(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (map[string]interface{}, bool, error) {
	switch kind {
	case "configmaps":
		obj, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "ConfigMap")
		return m, true, err

	case "secrets":
		obj, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "Secret")
		if err != nil {
			return nil, true, err
		}
		maskSecretMap(m)
		return m, true, nil

	case "persistentvolumeclaims":
		obj, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "PersistentVolumeClaim")
		return m, true, err

	case "persistentvolumes":
		obj, err := client.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "PersistentVolume")
		return m, true, err

	case "storageclasses":
		obj, err := client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "storage.k8s.io/v1", "StorageClass")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) createStorageConfig(ctx context.Context, client kubernetes.Interface, kind, namespace string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "configmaps":
		var obj corev1.ConfigMap
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding configmap manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().ConfigMaps(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating configmap: %w", err)
		}
		m, err := toMap(created, "v1", "ConfigMap")
		return m, true, err

	case "secrets":
		var obj corev1.Secret
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding secret manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().Secrets(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating secret: %w", err)
		}
		m, err := toMap(created, "v1", "Secret")
		if err != nil {
			return nil, true, err
		}
		maskSecretMap(m)
		return m, true, nil

	case "persistentvolumeclaims":
		var obj corev1.PersistentVolumeClaim
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding pvc manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().PersistentVolumeClaims(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating pvc: %w", err)
		}
		m, err := toMap(created, "v1", "PersistentVolumeClaim")
		return m, true, err

	case "persistentvolumes":
		var obj corev1.PersistentVolume
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding pv manifest: %w", err)
		}
		created, err := client.CoreV1().PersistentVolumes().Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating persistentvolume: %w", err)
		}
		m, err := toMap(created, "v1", "PersistentVolume")
		return m, true, err

	case "storageclasses":
		var obj storagev1.StorageClass
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding storageclass manifest: %w", err)
		}
		created, err := client.StorageV1().StorageClasses().Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating storageclass: %w", err)
		}
		m, err := toMap(created, "storage.k8s.io/v1", "StorageClass")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) updateStorageConfig(ctx context.Context, client kubernetes.Interface, kind, namespace, name string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "configmaps":
		var obj corev1.ConfigMap
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding configmap manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().ConfigMaps(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating configmap %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "ConfigMap")
		return m, true, err

	case "secrets":
		var obj corev1.Secret
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding secret manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().Secrets(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating secret %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "Secret")
		if err != nil {
			return nil, true, err
		}
		maskSecretMap(m)
		return m, true, nil

	case "persistentvolumeclaims":
		var obj corev1.PersistentVolumeClaim
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding pvc manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().PersistentVolumeClaims(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating pvc %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "PersistentVolumeClaim")
		return m, true, err

	case "persistentvolumes":
		var obj corev1.PersistentVolume
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding pv manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		updated, err := client.CoreV1().PersistentVolumes().Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating persistentvolume %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "PersistentVolume")
		return m, true, err

	case "storageclasses":
		var obj storagev1.StorageClass
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding storageclass manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		updated, err := client.StorageV1().StorageClasses().Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating storageclass %s: %w", name, err)
		}
		m, err := toMap(updated, "storage.k8s.io/v1", "StorageClass")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) deleteStorageConfig(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (bool, error) {
	switch kind {
	case "configmaps":
		return true, client.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "secrets":
		return true, client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "persistentvolumeclaims":
		return true, client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "persistentvolumes":
		return true, client.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
	case "storageclasses":
		return true, client.StorageV1().StorageClasses().Delete(ctx, name, metav1.DeleteOptions{})
	}

	return false, nil
}
