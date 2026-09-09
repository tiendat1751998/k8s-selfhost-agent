package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (r *ResourceRepo) listNetworking(ctx context.Context, client kubernetes.Interface, kind, namespace string) ([]map[string]interface{}, bool, error) {
	var results []map[string]interface{}
	switch kind {
	case "services":
		list, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing services: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Service")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "ingresses":
		list, err := client.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing ingresses: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "networking.k8s.io/v1", "Ingress")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "networkpolicies":
		list, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing networkpolicies: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "networking.k8s.io/v1", "NetworkPolicy")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil
	}

	return nil, false, nil
}

func (r *ResourceRepo) getNetworking(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (map[string]interface{}, bool, error) {
	switch kind {
	case "services":
		obj, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "Service")
		return m, true, err

	case "ingresses":
		obj, err := client.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "networking.k8s.io/v1", "Ingress")
		return m, true, err

	case "networkpolicies":
		obj, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "networking.k8s.io/v1", "NetworkPolicy")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) createNetworking(ctx context.Context, client kubernetes.Interface, kind, namespace string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "services":
		var obj corev1.Service
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding service manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.CoreV1().Services(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating service: %w", err)
		}
		m, err := toMap(created, "v1", "Service")
		return m, true, err

	case "ingresses":
		var obj networkingv1.Ingress
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding ingress manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.NetworkingV1().Ingresses(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating ingress: %w", err)
		}
		m, err := toMap(created, "networking.k8s.io/v1", "Ingress")
		return m, true, err

	case "networkpolicies":
		var obj networkingv1.NetworkPolicy
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding networkpolicy manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.NetworkingV1().NetworkPolicies(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating networkpolicy: %w", err)
		}
		m, err := toMap(created, "networking.k8s.io/v1", "NetworkPolicy")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) updateNetworking(ctx context.Context, client kubernetes.Interface, kind, namespace, name string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "services":
		var obj corev1.Service
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding service manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.CoreV1().Services(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating service %s: %w", name, err)
		}
		m, err := toMap(updated, "v1", "Service")
		return m, true, err

	case "ingresses":
		var obj networkingv1.Ingress
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding ingress manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.NetworkingV1().Ingresses(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating ingress %s: %w", name, err)
		}
		m, err := toMap(updated, "networking.k8s.io/v1", "Ingress")
		return m, true, err

	case "networkpolicies":
		var obj networkingv1.NetworkPolicy
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding networkpolicy manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.NetworkingV1().NetworkPolicies(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating networkpolicy %s: %w", name, err)
		}
		m, err := toMap(updated, "networking.k8s.io/v1", "NetworkPolicy")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) deleteNetworking(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (bool, error) {
	switch kind {
	case "services":
		return true, client.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "ingresses":
		return true, client.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "networkpolicies":
		return true, client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}

	return false, nil
}
