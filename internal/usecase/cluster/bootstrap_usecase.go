package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

	clusterDomain "github.com/datdt/k8sselfhost/internal/domain/cluster"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

const (
	defaultMetricsServerYAML = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: metrics-server
  namespace: kube-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
spec:
  selector:
    matchLabels:
      k8s-app: metrics-server
  template:
    metadata:
      labels:
        k8s-app: metrics-server
    spec:
      serviceAccountName: metrics-server
      containers:
      - name: metrics-server
        image: registry.k8s.io/metrics-server/metrics-server:v0.7.2
        imagePullPolicy: IfNotPresent
        args:
        - --cert-dir=/tmp
        - --secure-port=4443
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        - --kubelet-insecure-tls
        ports:
        - name: https
          containerPort: 4443
          protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
spec:
  selector:
    k8s-app: metrics-server
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: 4443
`

	defaultLocalStorageYAML = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: local-path-provisioner-service-account
  namespace: kube-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: local-path-provisioner
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: local-path-provisioner
  template:
    metadata:
      labels:
        app: local-path-provisioner
    spec:
      serviceAccountName: local-path-provisioner-service-account
      containers:
      - name: local-path-provisioner
        image: rancher/local-path-provisioner:v0.0.28
        imagePullPolicy: IfNotPresent
        command:
        - local-path-provisioner
        - --debug
        - start
        - --config
        - /etc/config/config.json
        volumeMounts:
        - name: config-volume
          mountPath: /etc/config/
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
      volumes:
      - name: config-volume
        configMap:
          name: local-path-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-path-config
  namespace: kube-system
data:
  config.json: |-
    {
      "nodePathMap":[
        {
          "node":"DEFAULT_PATH_FOR_NON_LISTED_NODES",
          "paths":["/opt/local-path-provisioner"]
        }
      ]
    }
  setup: |-
    #!/bin/sh
    set -eu
    mkdir -m 0777 -p "$VOL_DIR"
  teardown: |-
    #!/bin/sh
    set -eu
    rm -rf "$VOL_DIR"
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-path
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: rancher.io/local-path
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
`

	defaultControlAgentYAML = `apiVersion: v1
kind: ServiceAccount
metadata:
  name: k8s-control-agent
  namespace: kube-system
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: k8s-control-agent
  namespace: kube-system
  labels:
    app: k8s-control-agent
spec:
  selector:
    matchLabels:
      app: k8s-control-agent
  template:
    metadata:
      labels:
        app: k8s-control-agent
    spec:
      serviceAccountName: k8s-control-agent
      tolerations:
      - operator: Exists
      containers:
      - name: agent
        image: ghcr.io/datdt/k8s-control-agent:latest
        imagePullPolicy: IfNotPresent
`
)

// MetricsFetcher fetches raw metrics for pods.
type MetricsFetcher interface {
	FetchPodMetrics(ctx context.Context, clusterID string) ([]byte, error)
}

// BootstrapUsecase manages cluster essentials bootstrap operations and status checks.
type BootstrapUsecase struct {
	repo           *infraK8s.ResourceRepo
	clientManager  *infraCluster.ClientManager
	logger         *zap.Logger
	metricsFetcher MetricsFetcher
}

// NewBootstrapUsecase creates a new BootstrapUsecase.
func NewBootstrapUsecase(repo *infraK8s.ResourceRepo, clientManager *infraCluster.ClientManager, logger *zap.Logger) *BootstrapUsecase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BootstrapUsecase{
		repo:          repo,
		clientManager: clientManager,
		logger:        logger,
	}
}

// SetMetricsFetcher allows overriding the metrics retrieval implementation (useful for tests).
func (u *BootstrapUsecase) SetMetricsFetcher(fetcher MetricsFetcher) {
	u.metricsFetcher = fetcher
}

// GetEssentialsStatus inspects the cluster for required essentials: metrics-server, default StorageClass,
// control agent DaemonSet, and control-plane master node taints.
func (u *BootstrapUsecase) GetEssentialsStatus(ctx context.Context, clusterID string) (*clusterDomain.ClusterEssentialsStatus, error) {
	if u.repo == nil {
		return nil, fmt.Errorf("kubernetes resource repository not configured")
	}

	// 1. Check metrics-server Deployment in kube-system
	compMetrics := clusterDomain.EssentialComponent{
		Name:      clusterDomain.ComponentNameMetricsServer,
		Installed: false,
		Status:    clusterDomain.ComponentStatusNotFound,
		Namespace: "kube-system",
	}
	if dep, err := u.repo.GetResource(ctx, clusterID, "deployments", "kube-system", "metrics-server"); err == nil && dep != nil {
		compMetrics.Installed = true
		compMetrics.Version = extractImageTag(dep)
		if isDeploymentRunning(dep) {
			compMetrics.Status = clusterDomain.ComponentStatusRunning
		} else {
			compMetrics.Status = clusterDomain.ComponentStatusPending
		}
	}

	// 2. Check default StorageClass
	compStorage := clusterDomain.EssentialComponent{
		Name:      clusterDomain.ComponentNameLocalStorage,
		Installed: false,
		Status:    clusterDomain.ComponentStatusNotFound,
		Namespace: "",
	}
	if scList, err := u.repo.ListResources(ctx, clusterID, "storageclasses", ""); err == nil {
		for _, sc := range scList {
			if meta, ok := sc["metadata"].(map[string]interface{}); ok {
				if annotations, ok := meta["annotations"].(map[string]interface{}); ok {
					if isDef, _ := annotations["storageclass.kubernetes.io/is-default-class"].(string); isDef == "true" {
						compStorage.Installed = true
						compStorage.Status = clusterDomain.ComponentStatusRunning
						if prov, ok := sc["provisioner"].(string); ok {
							compStorage.Version = prov
						}
						break
					}
					if isBetaDef, _ := annotations["beta.kubernetes.io/storage-class"].(string); isBetaDef == "true" {
						compStorage.Installed = true
						compStorage.Status = clusterDomain.ComponentStatusRunning
						if prov, ok := sc["provisioner"].(string); ok {
							compStorage.Version = prov
						}
						break
					}
				}
			}
		}
	}

	// 3. Check k8s-control-agent DaemonSet in kube-system
	compAgent := clusterDomain.EssentialComponent{
		Name:      clusterDomain.ComponentNameControlAgent,
		Installed: false,
		Status:    clusterDomain.ComponentStatusNotFound,
		Namespace: "kube-system",
	}
	if ds, err := u.repo.GetResource(ctx, clusterID, "daemonsets", "kube-system", "k8s-control-agent"); err == nil && ds != nil {
		compAgent.Installed = true
		compAgent.Version = extractImageTag(ds)
		if isDaemonSetRunning(ds) {
			compAgent.Status = clusterDomain.ComponentStatusRunning
		} else {
			compAgent.Status = clusterDomain.ComponentStatusPending
		}
	}

	// 4. Check master node taints
	var taintsInfo []clusterDomain.NodeTaintInfo
	if nodes, err := u.repo.ListResources(ctx, clusterID, "nodes", ""); err == nil {
		for _, node := range nodes {
			var nodeName string
			if meta, ok := node["metadata"].(map[string]interface{}); ok {
				nodeName, _ = meta["name"].(string)
			}
			if nodeName == "" {
				continue
			}

			isTainted := false
			var taintsStr []string
			if spec, ok := node["spec"].(map[string]interface{}); ok {
				if taintsList, ok := spec["taints"].([]interface{}); ok {
					for _, t := range taintsList {
						if tMap, ok := t.(map[string]interface{}); ok {
							key, _ := tMap["key"].(string)
							effect, _ := tMap["effect"].(string)
							if key == "node-role.kubernetes.io/control-plane" || key == "node-role.kubernetes.io/master" {
								isTainted = true
								taintsStr = append(taintsStr, fmt.Sprintf("%s:%s", key, effect))
							}
						}
					}
				}
			}

			taintsInfo = append(taintsInfo, clusterDomain.NodeTaintInfo{
				NodeName: nodeName,
				Tainted:  isTainted,
				Taints:   strings.Join(taintsStr, ", "),
			})
		}
	} else {
		u.logger.Warn("failed to list nodes for taint check", zap.String("cluster", clusterID), zap.Error(err))
	}

	components := []clusterDomain.EssentialComponent{compMetrics, compStorage, compAgent}
	return clusterDomain.NewClusterEssentialsStatus(clusterID, components, taintsInfo), nil
}

// ExecuteBootstrap executes the requested bootstrap actions.
func (u *BootstrapUsecase) ExecuteBootstrap(ctx context.Context, clusterID string, req clusterDomain.BootstrapRequest) (*clusterDomain.BootstrapResult, error) {
	if u.repo == nil {
		return nil, fmt.Errorf("kubernetes resource repository not configured")
	}

	startTime := time.Now()
	var installed []string
	var errs []string

	// 1. Install Metrics Server
	if req.InstallMetricsServer {
		if err := u.repo.ApplyYAML(ctx, clusterID, "kube-system", []byte(defaultMetricsServerYAML)); err != nil {
			errs = append(errs, fmt.Sprintf("metrics-server: %v", err))
		} else {
			installed = append(installed, "metrics-server")
		}
	}

	// 2. Install Local StorageClass
	if req.InstallStorageClass {
		if err := u.repo.ApplyYAML(ctx, clusterID, "", []byte(defaultLocalStorageYAML)); err != nil {
			errs = append(errs, fmt.Sprintf("storage-class: %v", err))
		} else {
			installed = append(installed, "local-path-storageclass")
		}
	}

	// 3. Deploy Agent DaemonSet
	if req.DeployAgentDaemonSet {
		if err := u.repo.ApplyYAML(ctx, clusterID, "kube-system", []byte(defaultControlAgentYAML)); err != nil {
			errs = append(errs, fmt.Sprintf("k8s-control-agent: %v", err))
		} else {
			installed = append(installed, "k8s-control-agent")
		}
	}

	// 4. Untaint Masters
	if req.UntaintMasters {
		nodes, err := u.repo.ListResources(ctx, clusterID, "nodes", "")
		if err != nil {
			errs = append(errs, fmt.Sprintf("untaint: listing nodes failed: %v", err))
		} else {
			for _, node := range nodes {
				meta, _ := node["metadata"].(map[string]interface{})
				nodeName, _ := meta["name"].(string)
				if nodeName == "" {
					continue
				}

				spec, _ := node["spec"].(map[string]interface{})
				taintsList, _ := spec["taints"].([]interface{})
				hasMasterTaint := false
				var cleanTaints []corev1.Taint

				for _, t := range taintsList {
					if tMap, ok := t.(map[string]interface{}); ok {
						key, _ := tMap["key"].(string)
						val, _ := tMap["value"].(string)
						effect, _ := tMap["effect"].(string)
						if key == "node-role.kubernetes.io/control-plane" || key == "node-role.kubernetes.io/master" {
							hasMasterTaint = true
							continue
						}
						cleanTaints = append(cleanTaints, corev1.Taint{
							Key:    key,
							Value:  val,
							Effect: corev1.TaintEffect(effect),
						})
					}
				}

				if hasMasterTaint {
					if err := u.repo.UpdateNodeTaints(ctx, clusterID, nodeName, cleanTaints); err != nil {
						errs = append(errs, fmt.Sprintf("untaint node %s: %v", nodeName, err))
					} else {
						installed = append(installed, fmt.Sprintf("untaint-%s", nodeName))
					}
				}
			}
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	success := len(errs) == 0
	return clusterDomain.NewBootstrapResult(success, installed, errs, durationMs), nil
}

// GetPodMetrics queries metrics from metrics.k8s.io or returns an empty list if metrics-server is not installed.
func (u *BootstrapUsecase) GetPodMetrics(ctx context.Context, clusterID string) ([]byte, error) {
	emptyList := []byte(`{"kind":"PodMetricsList","apiVersion":"metrics.k8s.io/v1beta1","metadata":{},"items":[]}`)

	if u.metricsFetcher != nil {
		data, err := u.metricsFetcher.FetchPodMetrics(ctx, clusterID)
		if err == nil && len(data) > 0 {
			return data, nil
		}
		return emptyList, nil
	}

	if u.clientManager != nil {
		client, err := u.clientManager.GetK8sClient(ctx, clusterID)
		if err == nil && client != nil && client.RESTClient() != nil {
			data, err := client.RESTClient().Get().AbsPath("/apis/metrics.k8s.io/v1beta1/pods").DoRaw(ctx)
			if err == nil && len(data) > 0 {
				return data, nil
			}
			u.logger.Debug("failed to query pod metrics, returning empty list", zap.String("cluster", clusterID), zap.Error(err))
		}
	}

	return emptyList, nil
}

func extractImageTag(obj map[string]interface{}) string {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		return ""
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		return ""
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		return ""
	}
	c0, ok := containers[0].(map[string]interface{})
	if !ok {
		return ""
	}
	img, _ := c0["image"].(string)
	if idx := strings.LastIndex(img, ":"); idx != -1 {
		return img[idx+1:]
	}
	return img
}

func isDeploymentRunning(dep map[string]interface{}) bool {
	status, ok := dep["status"].(map[string]interface{})
	if !ok {
		return false
	}
	ready, ok := status["readyReplicas"]
	if !ok {
		return false
	}
	switch v := ready.(type) {
	case int:
		return v > 0
	case int32:
		return v > 0
	case int64:
		return v > 0
	case float64:
		return v > 0
	default:
		return false
	}
}

func isDaemonSetRunning(ds map[string]interface{}) bool {
	status, ok := ds["status"].(map[string]interface{})
	if !ok {
		return false
	}
	ready, ok := status["numberReady"]
	if !ok {
		ready, ok = status["currentNumberScheduled"]
	}
	if !ok {
		return false
	}
	switch v := ready.(type) {
	case int:
		return v > 0
	case int32:
		return v > 0
	case int64:
		return v > 0
	case float64:
		return v > 0
	default:
		return false
	}
}

