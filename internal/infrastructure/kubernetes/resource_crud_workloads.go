package kubernetes

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func (r *ResourceRepo) listWorkloads(ctx context.Context, client kubernetes.Interface, kind, namespace string) ([]map[string]interface{}, bool, error) {
	var results []map[string]interface{}
	switch kind {
	case "pods":
		list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing pods: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "v1", "Pod")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "deployments":
		list, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing deployments: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "apps/v1", "Deployment")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "statefulsets":
		list, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing statefulsets: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "apps/v1", "StatefulSet")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "daemonsets":
		list, err := client.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing daemonsets: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "apps/v1", "DaemonSet")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "jobs":
		list, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing jobs: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "batch/v1", "Job")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "cronjobs":
		list, err := client.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing cronjobs: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "batch/v1", "CronJob")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil

	case "horizontalpodautoscalers":
		list, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("listing horizontalpodautoscalers: %w", err)
		}
		for _, item := range list.Items {
			m, err := toMap(item, "autoscaling/v2", "HorizontalPodAutoscaler")
			if err != nil {
				return nil, true, err
			}
			results = append(results, m)
		}
		return results, true, nil
	}

	return nil, false, nil
}

func (r *ResourceRepo) getWorkload(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (map[string]interface{}, bool, error) {
	switch kind {
	case "pods":
		obj, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "v1", "Pod")
		return m, true, err

	case "deployments":
		obj, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "apps/v1", "Deployment")
		return m, true, err

	case "statefulsets":
		obj, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "apps/v1", "StatefulSet")
		return m, true, err

	case "daemonsets":
		obj, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "apps/v1", "DaemonSet")
		return m, true, err

	case "jobs":
		obj, err := client.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "batch/v1", "Job")
		return m, true, err

	case "cronjobs":
		obj, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "batch/v1", "CronJob")
		return m, true, err

	case "horizontalpodautoscalers":
		obj, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, true, err
		}
		m, err := toMap(obj, "autoscaling/v2", "HorizontalPodAutoscaler")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) createWorkload(ctx context.Context, client kubernetes.Interface, kind, namespace string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "deployments":
		var obj appsv1.Deployment
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding deployment manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AppsV1().Deployments(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating deployment: %w", err)
		}
		m, err := toMap(created, "apps/v1", "Deployment")
		return m, true, err

	case "statefulsets":
		var obj appsv1.StatefulSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding statefulset manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AppsV1().StatefulSets(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating statefulset: %w", err)
		}
		m, err := toMap(created, "apps/v1", "StatefulSet")
		return m, true, err

	case "daemonsets":
		var obj appsv1.DaemonSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding daemonset manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AppsV1().DaemonSets(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating daemonset: %w", err)
		}
		m, err := toMap(created, "apps/v1", "DaemonSet")
		return m, true, err

	case "jobs":
		var obj batchv1.Job
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding job manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.BatchV1().Jobs(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating job: %w", err)
		}
		m, err := toMap(created, "batch/v1", "Job")
		return m, true, err

	case "cronjobs":
		var obj batchv1.CronJob
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding cronjob manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.BatchV1().CronJobs(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating cronjob: %w", err)
		}
		m, err := toMap(created, "batch/v1", "CronJob")
		return m, true, err

	case "horizontalpodautoscalers":
		var obj autoscalingv2.HorizontalPodAutoscaler
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding hpa manifest: %w", err)
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		created, err := client.AutoscalingV2().HorizontalPodAutoscalers(obj.Namespace).Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("creating horizontalpodautoscaler: %w", err)
		}
		m, err := toMap(created, "autoscaling/v2", "HorizontalPodAutoscaler")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) updateWorkload(ctx context.Context, client kubernetes.Interface, kind, namespace, name string, manifest map[string]interface{}) (map[string]interface{}, bool, error) {
	switch kind {
	case "deployments":
		var obj appsv1.Deployment
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding deployment manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AppsV1().Deployments(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating deployment %s: %w", name, err)
		}
		m, err := toMap(updated, "apps/v1", "Deployment")
		return m, true, err

	case "statefulsets":
		var obj appsv1.StatefulSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding statefulset manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AppsV1().StatefulSets(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating statefulset %s: %w", name, err)
		}
		m, err := toMap(updated, "apps/v1", "StatefulSet")
		return m, true, err

	case "daemonsets":
		var obj appsv1.DaemonSet
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding daemonset manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AppsV1().DaemonSets(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating daemonset %s: %w", name, err)
		}
		m, err := toMap(updated, "apps/v1", "DaemonSet")
		return m, true, err

	case "jobs":
		var obj batchv1.Job
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding job manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.BatchV1().Jobs(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating job %s: %w", name, err)
		}
		m, err := toMap(updated, "batch/v1", "Job")
		return m, true, err

	case "cronjobs":
		var obj batchv1.CronJob
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding cronjob manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.BatchV1().CronJobs(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating cronjob %s: %w", name, err)
		}
		m, err := toMap(updated, "batch/v1", "CronJob")
		return m, true, err

	case "horizontalpodautoscalers":
		var obj autoscalingv2.HorizontalPodAutoscaler
		if err := fromMap(manifest, &obj); err != nil {
			return nil, true, fmt.Errorf("decoding hpa manifest: %w", err)
		}
		if obj.Name == "" {
			obj.Name = name
		}
		if namespace != "" && obj.Namespace == "" {
			obj.Namespace = namespace
		}
		updated, err := client.AutoscalingV2().HorizontalPodAutoscalers(obj.Namespace).Update(ctx, &obj, metav1.UpdateOptions{})
		if err != nil {
			return nil, true, fmt.Errorf("updating horizontalpodautoscaler %s: %w", name, err)
		}
		m, err := toMap(updated, "autoscaling/v2", "HorizontalPodAutoscaler")
		return m, true, err
	}

	return nil, false, nil
}

func (r *ResourceRepo) deleteWorkload(ctx context.Context, client kubernetes.Interface, kind, namespace, name string) (bool, error) {
	switch kind {
	case "pods":
		return true, client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "deployments":
		return true, client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "statefulsets":
		return true, client.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "daemonsets":
		return true, client.AppsV1().DaemonSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "jobs":
		return true, client.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "cronjobs":
		return true, client.BatchV1().CronJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	case "horizontalpodautoscalers":
		return true, client.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}

	return false, nil
}
