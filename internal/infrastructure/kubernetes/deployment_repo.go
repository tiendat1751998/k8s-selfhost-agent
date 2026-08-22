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
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/domain/deployment"
	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
)

type deploymentRepo struct {
	defaultK8sClient  *kubernetes.Clientset
	defaultDockerRepo domainDocker.Repository
	fleetRepo         fleet.Repository
	clientManager     *infraCluster.ClientManager
}

// NewDeploymentRepo creates a new live deployment repository.
func NewDeploymentRepo(
	defaultK8sClient *kubernetes.Clientset,
	defaultDockerRepo domainDocker.Repository,
	fleetRepo fleet.Repository,
	clientManager *infraCluster.ClientManager,
) deployment.Repository {
	return &deploymentRepo{
		defaultK8sClient:  defaultK8sClient,
		defaultDockerRepo: defaultDockerRepo,
		fleetRepo:         fleetRepo,
		clientManager:     clientManager,
	}
}

func (r *deploymentRepo) getK8sClient(ctx context.Context, clusterName string) (*kubernetes.Clientset, error) {
	if r.clientManager != nil && clusterName != "" {
		clusters, err := r.fleetRepo.ListClusters(ctx)
		if err == nil {
			for _, c := range clusters {
				if c.Name == clusterName {
					cli, err := r.clientManager.GetK8sClient(ctx, c.ID)
					if err == nil && cli != nil {
						return cli, nil
					}
				}
			}
		}
	}
	if r.defaultK8sClient != nil {
		return r.defaultK8sClient, nil
	}
	return nil, fmt.Errorf("kubernetes client not found for cluster %s", clusterName)
}

func (r *deploymentRepo) getDockerClient(ctx context.Context, clusterName string) (domainDocker.Repository, error) {
	if r.clientManager != nil && clusterName != "" {
		clusters, err := r.fleetRepo.ListClusters(ctx)
		if err == nil {
			for _, c := range clusters {
				if c.Name == clusterName {
					cli, err := r.clientManager.GetDockerClient(ctx, c.ID)
					if err == nil && cli != nil {
						return infraDocker.NewDockerRepoWithClient(cli), nil
					}
				}
			}
		}
	}
	if r.defaultDockerRepo != nil {
		return r.defaultDockerRepo, nil
	}
	return nil, fmt.Errorf("docker repository not found for cluster %s", clusterName)
}

func (r *deploymentRepo) List(ctx context.Context) ([]deployment.Application, error) {
	var apps []deployment.Application

	var clusters []fleet.Cluster
	var err error
	if r.fleetRepo != nil {
		clusters, err = r.fleetRepo.ListClusters(ctx)
	}

	if err == nil && len(clusters) > 0 {
		for _, c := range clusters {
			if c.Provider == "docker" {
				dCli, err := r.clientManager.GetDockerClient(ctx, c.ID)
				if err == nil && dCli != nil {
					repo := infraDocker.NewDockerRepoWithClient(dCli)
					svcs, err := repo.ListServices(ctx)
					if err == nil {
						for _, s := range svcs {
							apps = append(apps, r.mapSwarmService(s, c.Name))
						}
					}
				}
			} else {
				kCli, err := r.clientManager.GetK8sClient(ctx, c.ID)
				if err == nil && kCli != nil {
					deps, err := kCli.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
					if err == nil {
						for _, d := range deps.Items {
							if isSystemNamespace(d.Namespace) {
								continue
							}
							apps = append(apps, r.mapK8sDeployment(d, c.Name))
						}
					}
				}
			}
		}
	}

	if len(apps) == 0 {
		// Fallback to local default clients if available
		if r.defaultK8sClient != nil {
			deps, err := r.defaultK8sClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
			if err == nil {
				for _, d := range deps.Items {
					if isSystemNamespace(d.Namespace) {
						continue
					}
					apps = append(apps, r.mapK8sDeployment(d, "prod-us-east"))
				}
			}
		}
		if r.defaultDockerRepo != nil {
			svcs, err := r.defaultDockerRepo.ListServices(ctx)
			if err == nil {
				for _, s := range svcs {
					apps = append(apps, r.mapSwarmService(s, "swarm-cluster"))
				}
			}
		}
	}

	return apps, nil
}

func (r *deploymentRepo) Scale(ctx context.Context, targetType, targetCluster, namespace, name string, replicas int) error {
	if targetType == "kubernetes" {
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
	} else if targetType == "swarm" || targetType == "docker" {
		repo, err := r.getDockerClient(ctx, targetCluster)
		if err != nil {
			return err
		}
		return repo.ScaleService(ctx, name, replicas)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}

func (r *deploymentRepo) Restart(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if targetType == "kubernetes" {
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
	} else if targetType == "swarm" || targetType == "docker" {
		repo, err := r.getDockerClient(ctx, targetCluster)
		if err != nil {
			return err
		}
		return repo.RestartService(ctx, name)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}

func (r *deploymentRepo) Delete(ctx context.Context, targetType, targetCluster, namespace, name string) error {
	if targetType == "kubernetes" {
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
	} else if targetType == "swarm" || targetType == "docker" {
		repo, err := r.getDockerClient(ctx, targetCluster)
		if err != nil {
			return err
		}
		return repo.DeleteService(ctx, name)
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
}

func (r *deploymentRepo) Create(ctx context.Context, app deployment.Application) error {
	if app.Type == "kubernetes" {
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
	} else if app.Type == "swarm" || app.Type == "docker" {
		repo, err := r.getDockerClient(ctx, app.Target)
		if err != nil {
			return err
		}
		return repo.CreateService(ctx, app.Name, app.Image, app.Replicas, app.Port)
	}
	return fmt.Errorf("unsupported target type: %s", app.Type)
}

func (r *deploymentRepo) UpdateResources(ctx context.Context, targetType, targetCluster, namespace, name string, memoryLimitBytes, memoryReservBytes, nanoCPUs int64, replicas int) error {
	if targetType == "kubernetes" || targetType == "" {
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
	} else if targetType == "swarm" || targetType == "docker" {
		repo, err := r.getDockerClient(ctx, targetCluster)
		if err != nil {
			return err
		}
		err = repo.UpdateServiceResources(ctx, name, memoryLimitBytes, memoryReservBytes, nanoCPUs)
		if err != nil {
			return err
		}
		if replicas >= 0 {
			err = repo.ScaleService(ctx, name, replicas)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unsupported target type: %s", targetType)
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
	if d.Status.ReadyReplicas == int32(replicas) && replicas > 0 {
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
	memReq := "128Mi"
	if len(d.Spec.Template.Spec.Containers) > 0 {
		res := d.Spec.Template.Spec.Containers[0].Resources
		if !res.Requests.Cpu().IsZero() {
			cpuReq = res.Requests.Cpu().String()
		}
		if !res.Requests.Memory().IsZero() {
			memReq = res.Requests.Memory().String()
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
		Name:      d.Name,
		Team:      team,
		Env:       env,
		Image:     image,
		Target:    clusterName,
		Namespace: d.Namespace,
		Type:      "kubernetes",
		Replicas:  replicas,
		Status:    status,
		CPU:       cpuReq,
		Memory:    memReq,
		Port:      port,
		NetType:   netType,
		Volume:    volume,
	}
}

func (r *deploymentRepo) mapSwarmService(s domainDocker.Service, clusterName string) deployment.Application {
	status := "healthy"
	if s.Replicas == 0 {
		status = "down"
	}

	team := "SRE"
	env := "production"
	if strings.Contains(s.Name, "prod") {
		env = "production"
	} else if strings.Contains(s.Name, "stg") || strings.Contains(s.Name, "staging") {
		env = "staging"
		team = "Dev-Auth"
	} else if strings.Contains(s.Name, "worker") || strings.Contains(s.Name, "payment") {
		team = "Finance"
	}

	port := 80
	if len(s.Ports) > 0 {
		parts := strings.Split(s.Ports[0], ":")
		if len(parts) >= 2 {
			_, _ = fmt.Sscanf(parts[0], "%d", &port)
		}
	}

	return deployment.Application{
		Name:      s.Name,
		Team:      team,
		Env:       env,
		Image:     s.Image,
		Target:    clusterName,
		Namespace: "",
		Type:      "swarm",
		Replicas:  s.Replicas,
		Status:    status,
		CPU:       "500m",
		Memory:    "1Gi",
		Port:      port,
		NetType:   "NodePort",
		Volume:    "none",
	}
}

func isSystemNamespace(ns string) bool {
	return ns == "kube-system" || ns == "kube-public" || ns == "kube-node-lease" || ns == "local-path-storage"
}
