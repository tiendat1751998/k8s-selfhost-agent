package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
)

func (r *deploymentRepo) scaleK8sDeployment(ctx context.Context, targetCluster, namespace, name string, replicas int) error {
	client, err := r.getK8sClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = "default"
	}
	scale, err := client.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting scale for deployment %s: %w", name, err)
	}
	scale.Spec.Replicas = int32(replicas)
	_, err = client.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating scale for deployment %s: %w", name, err)
	}
	return nil
}

func (r *deploymentRepo) restartK8sDeployment(ctx context.Context, targetCluster, namespace, name string) error {
	client, err := r.getK8sClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = "default"
	}
	deploymentObj, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s for restart: %w", name, err)
	}
	if deploymentObj.Spec.Template.Annotations == nil {
		deploymentObj.Spec.Template.Annotations = make(map[string]string)
	}
	deploymentObj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deploymentObj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating deployment %s for restart: %w", name, err)
	}
	return nil
}

func (r *deploymentRepo) deleteK8sDeployment(ctx context.Context, targetCluster, namespace, name string) error {
	client, err := r.getK8sClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = "default"
	}
	err = client.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("deleting deployment %s: %w", name, err)
	}
	return nil
}

func (r *deploymentRepo) createK8sDeployment(ctx context.Context, app deployment.Application) error {
	client, err := r.getK8sClient(ctx, app.Target)
	if err != nil {
		return err
	}
	ns := app.Namespace
	if ns == "" {
		ns = "default"
	}

	replicasVal := int32(app.Replicas)
	deploymentObj := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: ns,
			Labels: map[string]string{
				"app":  app.Name,
				"team": app.Team,
				"env":  app.Env,
			},
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicasVal,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": app.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": app.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  app.Name,
							Image: app.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(app.Port),
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(app.CPU),
									corev1.ResourceMemory: resource.MustParse(app.Memory),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = client.AppsV1().Deployments(ns).Create(ctx, deploymentObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating kubernetes deployment: %w", err)
	}

	serviceObj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": app.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       int32(app.Port),
					TargetPort: intstr.FromInt(app.Port),
				},
			},
			Type: corev1.ServiceType(app.NetType),
		},
	}
	_, err = client.CoreV1().Services(ns).Create(ctx, serviceObj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("creating service for app %s: %w", app.Name, err)
	}

	return nil
}

func (r *deploymentRepo) updateK8sResources(ctx context.Context, targetCluster, namespace, name string, memoryLimitBytes, memoryReservBytes, nanoCPUs int64, replicas int) error {
	client, err := r.getK8sClient(ctx, targetCluster)
	if err != nil {
		return err
	}
	if namespace == "" {
		namespace = "default"
	}
	dep, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s: %w", name, err)
	}
	if replicas >= 0 {
		rep := int32(replicas)
		dep.Spec.Replicas = &rep
	}
	if len(dep.Spec.Template.Spec.Containers) > 0 {
		c := &dep.Spec.Template.Spec.Containers[0]
		if c.Resources.Limits == nil {
			c.Resources.Limits = make(corev1.ResourceList)
		}
		if c.Resources.Requests == nil {
			c.Resources.Requests = make(corev1.ResourceList)
		}
		if memoryLimitBytes > 0 {
			c.Resources.Limits[corev1.ResourceMemory] = *resource.NewQuantity(memoryLimitBytes, resource.BinarySI)
		}
		if memoryReservBytes > 0 {
			c.Resources.Requests[corev1.ResourceMemory] = *resource.NewQuantity(memoryReservBytes, resource.BinarySI)
		}
		if nanoCPUs > 0 {
			c.Resources.Limits[corev1.ResourceCPU] = *resource.NewScaledQuantity(nanoCPUs, resource.Nano)
		}
	}
	_, err = client.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating deployment %s resources: %w", name, err)
	}
	return nil
}

func (r *deploymentRepo) mapK8sDeployment(d v1.Deployment, clusterName string) deployment.Application {
	replicas := 0
	if d.Spec.Replicas != nil {
		replicas = int(*d.Spec.Replicas)
	}

	image := ""
	if len(d.Spec.Template.Spec.Containers) > 0 {
		image = d.Spec.Template.Spec.Containers[0].Image
	}

	status := "down"
	if d.Status.ReadyReplicas >= int32(replicas) && replicas > 0 {
		status = "healthy"
	} else if d.Status.ReadyReplicas > 0 {
		status = "degraded"
	}

	team := d.Labels["team"]
	if team == "" {
		team = "SRE"
	}

	env := d.Labels["env"]
	if env == "" {
		if strings.Contains(d.Namespace, "prod") || strings.Contains(d.Name, "prod") {
			env = "production"
		} else if strings.Contains(d.Namespace, "stage") || strings.Contains(d.Name, "stage") {
			env = "staging"
		} else {
			env = "default"
		}
	}

	cpuReq := "100m"
	memReq := "128MiB"
	cpuLimit := "1 Core"
	memLimit := "512MiB"
	if len(d.Spec.Template.Spec.Containers) > 0 {
		res := d.Spec.Template.Spec.Containers[0].Resources
		if !res.Requests.Cpu().IsZero() {
			cpuReq = res.Requests.Cpu().String()
		}
		if !res.Requests.Memory().IsZero() {
			memReq = res.Requests.Memory().String()
		}
		if !res.Limits.Cpu().IsZero() {
			cpuLimit = res.Limits.Cpu().String()
		}
		if !res.Limits.Memory().IsZero() {
			memLimit = res.Limits.Memory().String()
		}
	}

	port := 80
	netType := "ClusterIP"
	if len(d.Spec.Template.Spec.Containers) > 0 && len(d.Spec.Template.Spec.Containers[0].Ports) > 0 {
		port = int(d.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
	}

	volume := "none"
	if len(d.Spec.Template.Spec.Volumes) > 0 {
		volume = "mounted"
	}

	return deployment.Application{
		Name:              d.Name,
		Team:              team,
		Env:               env,
		Image:             image,
		Target:            clusterName,
		Namespace:         d.Namespace,
		Type:              "kubernetes",
		Replicas:          replicas,
		Status:            status,
		CPU:               cpuLimit,
		Memory:            memLimit,
		CPULimit:          cpuLimit,
		CPUReservation:    cpuReq,
		MemoryLimit:       memLimit,
		MemoryReservation: memReq,
		Port:              port,
		NetType:           netType,
		Volume:            volume,
	}
}
