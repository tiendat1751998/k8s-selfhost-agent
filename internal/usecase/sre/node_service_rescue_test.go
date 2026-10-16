package sre_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/usecase/sre"
)

type testHostProber struct {
	pingOk       bool
	agentOk      bool
	kubeletOk    bool
	containerdOk bool
}

func (p *testHostProber) CheckHostConnectivity(ctx context.Context, node *corev1.Node) (bool, bool, error) {
	return p.pingOk, p.agentOk, nil
}

func (p *testHostProber) CheckKubeletHealth(ctx context.Context, node *corev1.Node) (bool, bool, error) {
	return p.kubeletOk, p.containerdOk, nil
}

type testServiceExecutor struct {
	restartErr error
	restarted  []string
	onRestart  func()
}

func (e *testServiceExecutor) RestartServices(ctx context.Context, node *corev1.Node, services []string) error {
	e.restarted = append(e.restarted, services...)
	if e.onRestart != nil {
		e.onRestart()
	}
	return e.restartErr
}

func TestProbeNodeHealth_SoftDeathServiceFault(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-soft-dead"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown, Reason: "NodeStatusUnknown"},
			},
		},
	}
	client := fake.NewSimpleClientset(node)

	prober := &testHostProber{
		pingOk:       true,
		agentOk:      true,
		kubeletOk:    false,
		containerdOk: false,
	}

	rescueManager := sre.NewNodeServiceRescueManager(prober, nil)
	report, err := rescueManager.ProbeNodeHealth(context.Background(), client, "worker-soft-dead")
	if err != nil {
		t.Fatalf("unexpected error probing node: %v", err)
	}

	if report.Classification != sre.ClassificationSoftDeathServiceFault {
		t.Errorf("expected classification %s, got %s", sre.ClassificationSoftDeathServiceFault, report.Classification)
	}
	if !report.HostAlive {
		t.Errorf("expected host to be alive")
	}
	if report.KubeletResponsive || report.ContainerdResponsive {
		t.Errorf("expected kubelet and containerd to be unresponsive")
	}
}

func TestProbeNodeHealth_HardDeathFencingRequired(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-hard-dead"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown, Reason: "NodeStatusUnknown"},
			},
		},
	}
	client := fake.NewSimpleClientset(node)

	prober := &testHostProber{
		pingOk:       false,
		agentOk:      false,
		kubeletOk:    false,
		containerdOk: false,
	}

	rescueManager := sre.NewNodeServiceRescueManager(prober, nil)
	report, err := rescueManager.ProbeNodeHealth(context.Background(), client, "worker-hard-dead")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Classification != sre.ClassificationHardDeathFencingRequired {
		t.Errorf("expected classification %s, got %s", sre.ClassificationHardDeathFencingRequired, report.Classification)
	}
	if report.HostAlive {
		t.Errorf("expected host to be dead")
	}
}

func TestExecuteServiceRescue_SuccessRestoresLeaseCancelsFencing(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-soft-dead"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
			},
		},
	}
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-soft-dead",
			Namespace: "kube-node-lease",
		},
		Spec: coordinationv1.LeaseSpec{
			RenewTime: &metav1.MicroTime{Time: time.Now().Add(-5 * time.Minute)},
		},
	}
	client := fake.NewSimpleClientset(node, lease)

	executor := &testServiceExecutor{
		onRestart: func() {
			// Simulate node recovery upon service restart
			node.Status.Conditions = []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			}
			_, _ = client.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
			lease.Spec.RenewTime = &metav1.MicroTime{Time: time.Now()}
			_, _ = client.CoordinationV1().Leases("kube-node-lease").Update(context.Background(), lease, metav1.UpdateOptions{})
		},
	}

	rescueManager := sre.NewNodeServiceRescueManager(nil, executor)
	res, err := rescueManager.ExecuteServiceRescue(context.Background(), client, "worker-soft-dead", []string{"containerd", "kubelet"})
	if err != nil {
		t.Fatalf("unexpected rescue error: %v", err)
	}

	if !res.Success {
		t.Errorf("expected rescue success")
	}
	if !res.CancelFencing {
		t.Errorf("expected CancelFencing to be true")
	}
	if res.ActionTaken != sre.ActionRescueRestoredWorkloadsPreserved {
		t.Errorf("expected action %s, got %s", sre.ActionRescueRestoredWorkloadsPreserved, res.ActionTaken)
	}
}

func TestExecuteServiceRescue_RestartFailureRequiresFencing(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-stuck"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
			},
		},
	}
	client := fake.NewSimpleClientset(node)

	executor := &testServiceExecutor{
		restartErr: errors.New("systemctl restart containerd timed out: runtime socket deadlocked"),
	}

	rescueManager := sre.NewNodeServiceRescueManager(nil, executor)
	res, err := rescueManager.ExecuteServiceRescue(context.Background(), client, "worker-stuck", []string{"containerd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Success {
		t.Errorf("expected rescue failure")
	}
	if res.CancelFencing {
		t.Errorf("expected CancelFencing to be false")
	}
	if res.ActionTaken != sre.ActionRescueFailedProceedToFencing {
		t.Errorf("expected action %s, got %s", sre.ActionRescueFailedProceedToFencing, res.ActionTaken)
	}
}

type rescueMockHostRepo struct {
	hosts []docker.ComputeHost
}

func (m *rescueMockHostRepo) Create(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *rescueMockHostRepo) GetByID(ctx context.Context, id string) (*docker.ComputeHost, error) {
	for _, h := range m.hosts {
		if h.ID == id {
			return &h, nil
		}
	}
	return nil, nil
}
func (m *rescueMockHostRepo) List(ctx context.Context, tenantID string) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}
func (m *rescueMockHostRepo) ListAll(ctx context.Context) ([]docker.ComputeHost, error) {
	return m.hosts, nil
}
func (m *rescueMockHostRepo) Update(ctx context.Context, host *docker.ComputeHost) error { return nil }
func (m *rescueMockHostRepo) Delete(ctx context.Context, id string) error               { return nil }
func (m *rescueMockHostRepo) UpdateStatus(ctx context.Context, id string, status string, lastHealthCheck time.Time) error {
	return nil
}

func TestDefaultServiceExecutor_RestartServices_RealAgentRPC(t *testing.T) {
	restartedServices := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/remediate" && r.Method == http.MethodPost {
			var body struct {
				Action string `json:"action"`
				Target string `json:"target"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				restartedServices = append(restartedServices, body.Target)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"action":"restart_service","details":"service active"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	hostRepo := &rescueMockHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-worker-1",
				Name:     "worker-agent-1",
				Endpoint: server.URL,
				Labels:   map[string]string{"auth_token": "secret-123"},
			},
		},
	}

	mgr := sre.NewNodeServiceRescueManager(nil, nil)
	mgr.SetComputeHostRepo(hostRepo)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-agent-1"},
	}

	executor, ok := mgr.Executor().(sre.ServiceExecutor)
	if !ok || executor == nil {
		t.Fatalf("expected non-nil ServiceExecutor from manager")
	}

	err := executor.RestartServices(context.Background(), node, []string{"containerd", "kubelet"})
	if err != nil {
		t.Fatalf("unexpected error restarting services via agent: %v", err)
	}

	if len(restartedServices) != 2 || restartedServices[0] != "containerd" || restartedServices[1] != "kubelet" {
		t.Errorf("expected restarted services containerd, kubelet; got %+v", restartedServices)
	}
}

func TestDefaultHostProber_CheckKubeletHealth_ViaAgentK8sHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/k8s/health" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"kubelet_healthy": true,
				"containerd_healthy": true,
				"pleg_healthy": true
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	hostRepo := &rescueMockHostRepo{
		hosts: []docker.ComputeHost{
			{
				ID:       "host-worker-2",
				Name:     "worker-agent-2",
				Endpoint: server.URL,
			},
		},
	}

	mgr := sre.NewNodeServiceRescueManager(nil, nil)
	mgr.SetComputeHostRepo(hostRepo)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-agent-2"},
	}

	prober, ok := mgr.Prober().(sre.HostProber)
	if !ok || prober == nil {
		t.Fatalf("expected non-nil HostProber from manager")
	}

	kubeletOk, containerdOk, err := prober.CheckKubeletHealth(context.Background(), node)
	if err != nil {
		t.Fatalf("unexpected error checking kubelet health: %v", err)
	}

	if !kubeletOk || !containerdOk {
		t.Errorf("expected kubeletOk and containerdOk to be true, got %t, %t", kubeletOk, containerdOk)
	}
}

