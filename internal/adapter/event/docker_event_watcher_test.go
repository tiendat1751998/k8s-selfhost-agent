package event

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

type mockDockerEventAPI struct {
	eventsCh       chan events.Message
	errCh          chan error
	containers     []container.Summary
	logsMap        map[string]string
	listErr        error
	logsErr        error
	mu             sync.Mutex
}

func newMockDockerEventAPI() *mockDockerEventAPI {
	return &mockDockerEventAPI{
		eventsCh:   make(chan events.Message, 10),
		errCh:      make(chan error, 10),
		containers: make([]container.Summary, 0),
		logsMap:    make(map[string]string),
	}
}

func (m *mockDockerEventAPI) Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error) {
	return m.eventsCh, m.errCh
}

func (m *mockDockerEventAPI) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logsErr != nil {
		return nil, m.logsErr
	}
	logContent := m.logsMap[containerID]
	return io.NopCloser(bytes.NewBufferString(logContent)), nil
}

func (m *mockDockerEventAPI) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	res := make([]container.Summary, len(m.containers))
	copy(res, m.containers)
	return res, nil
}

type mockIncidentRepo struct {
	mu        sync.Mutex
	incidents map[string]*incident.Incident
	created   []*incident.Incident
	updated   []*incident.Incident
}

func newMockIncidentRepo() *mockIncidentRepo {
	return &mockIncidentRepo{
		incidents: make(map[string]*incident.Incident),
		created:   make([]*incident.Incident, 0),
		updated:   make([]*incident.Incident, 0),
	}
}

func (r *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inc.ID == "" {
		inc.ID = fmt.Sprintf("inc-%d", len(r.incidents)+1)
	}
	r.incidents[inc.ID] = inc
	r.created = append(r.created, inc)
	return nil
}

func (r *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	inc, ok := r.incidents[id]
	if !ok {
		return nil, fmt.Errorf("incident not found: %s", id)
	}
	return inc, nil
}

func (r *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incidents[inc.ID] = inc
	r.updated = append(r.updated, inc)
	return nil
}

func (r *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var res []*incident.Incident
	for _, inc := range r.incidents {
		res = append(res, inc)
	}
	return res, int64(len(res)), nil
}

func (r *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incidentType incident.Type) (*incident.Incident, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, inc := range r.incidents {
		if inc.Namespace == namespace && inc.PodName == podName && inc.Type == incidentType {
			if inc.Status != incident.StatusResolved && inc.Status != incident.StatusFailed {
				return inc, nil
			}
		}
	}
	return nil, nil
}

type mockBroadcaster struct {
	mu       sync.Mutex
	messages []struct {
		MsgType string
		Data    interface{}
	}
}

func (b *mockBroadcaster) Broadcast(msgType string, data interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, struct {
		MsgType string
		Data    interface{}
	}{MsgType: msgType, Data: data})
}

func (b *mockBroadcaster) getMessages() []struct {
	MsgType string
	Data    interface{}
} {
	b.mu.Lock()
	defer b.mu.Unlock()
	res := make([]struct {
		MsgType string
		Data    interface{}
	}, len(b.messages))
	copy(res, b.messages)
	return res
}

func TestCrashTracker_SlidingWindow(t *testing.T) {
	tracker := NewCrashTracker(5 * time.Minute)
	now := time.Now().UTC()
	service := "prod/auth-service"

	// 1st crash at T-10m (outside 5-min window of subsequent crashes)
	c1 := tracker.RecordCrash(service, now.Add(-10*time.Minute))
	if c1 != 1 {
		t.Fatalf("expected 1 crash in window, got %d", c1)
	}
	// 2nd crash at T-4m (T-10m is 6m older than T-4m, so pruned)
	c2 := tracker.RecordCrash(service, now.Add(-4*time.Minute))
	if c2 != 1 {
		t.Fatalf("expected 1 crash in window, got %d", c2)
	}

	// 3rd crash at T-2m
	c3 := tracker.RecordCrash(service, now.Add(-2*time.Minute))
	if c3 != 2 {
		t.Fatalf("expected 2 crashes in window, got %d", c3)
	}

	// 4th crash at T-0m
	c4 := tracker.RecordCrash(service, now)
	if c4 != 3 {
		t.Fatalf("expected 3 crashes in window, got %d", c4)
	}

	// Check count
	if count := tracker.GetCount(service, now); count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}

	// Reset
	tracker.Reset(service)
	if count := tracker.GetCount(service, now); count != 0 {
		t.Errorf("expected count 0 after reset, got %d", count)
	}
}

func TestDockerEventWatcher_OOMKilled(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}
	logger := zap.NewNop()

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, logger,
		WithDockerClusterName("test-cluster"),
		WithDockerDefaultNamespace("production"),
	)

	mockAPI.logsMap["c123"] = "FATAL: Out of memory\nKilled"

	msg := events.Message{
		Type:   "container",
		Action: "oom",
		Actor: events.Actor{
			ID: "c123",
			Attributes: map[string]string{
				"name":                         "/payment-api-1",
				"com.docker.swarm.service.name": "payment-api",
				"com.docker.stack.namespace":    "production",
				"image":                        "payment:v1.2",
				"exitCode":                     "137",
			},
		},
		Time: time.Now().Unix(),
	}

	watcher.ProcessEvent(context.Background(), msg)

	mockRepo.mu.Lock()
	created := mockRepo.created
	mockRepo.mu.Unlock()

	if len(created) != 1 {
		t.Fatalf("expected 1 incident created, got %d", len(created))
	}

	inc := created[0]
	if inc.Type != incident.TypeOOMKilled {
		t.Errorf("expected TypeOOMKilled, got %s", inc.Type)
	}
	if inc.Severity != incident.SeverityCritical {
		t.Errorf("expected SeverityCritical, got %s", inc.Severity)
	}
	if inc.PodName != "payment-api" {
		t.Errorf("expected service name 'payment-api', got %s", inc.PodName)
	}
	if inc.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %s", inc.Namespace)
	}
	if inc.RawData["logs"] != "FATAL: Out of memory\nKilled" {
		t.Errorf("expected enriched logs, got %s", inc.RawData["logs"])
	}
	if inc.RawData["exit_code"] != "137" {
		t.Errorf("expected exit_code 137, got %s", inc.RawData["exit_code"])
	}

	msgs := mockBC.getMessages()
	if len(msgs) != 1 || msgs[0].MsgType != "incident" {
		t.Errorf("expected 1 'incident' broadcast, got %v", msgs)
	}
}

func TestDockerEventWatcher_CrashLoopBackOff_Streak(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, zap.NewNop(),
		WithDockerCrashThreshold(3),
	)

	mockAPI.logsMap["c-die"] = "panic: runtime error: invalid memory address"

	createDieEvent := func(id string) events.Message {
		return events.Message{
			Type:   "container",
			Action: "die",
			Actor: events.Actor{
				ID: id,
				Attributes: map[string]string{
					"name":                      "/auth-srv-1",
					"com.docker.compose.service": "auth-srv",
					"com.docker.compose.project": "auth",
					"image":                     "auth:v2",
					"exitCode":                  "1",
				},
			},
			Time: time.Now().Unix(),
		}
	}

	ctx := context.Background()

	// Crash 1: no incident yet
	watcher.ProcessEvent(ctx, createDieEvent("c-die"))
	if len(mockRepo.created) != 0 {
		t.Fatalf("expected 0 incidents at 1 crash, got %d", len(mockRepo.created))
	}

	// Crash 2: no incident yet
	watcher.ProcessEvent(ctx, createDieEvent("c-die"))
	if len(mockRepo.created) != 0 {
		t.Fatalf("expected 0 incidents at 2 crashes, got %d", len(mockRepo.created))
	}

	// Crash 3: threshold reached -> triggers CrashLoopBackOff!
	watcher.ProcessEvent(ctx, createDieEvent("c-die"))
	if len(mockRepo.created) != 1 {
		t.Fatalf("expected 1 incident at 3 crashes, got %d", len(mockRepo.created))
	}

	inc := mockRepo.created[0]
	if inc.Type != incident.TypeCrashLoopBackOff {
		t.Errorf("expected TypeCrashLoopBackOff, got %s", inc.Type)
	}
	if inc.Severity != incident.SeverityHigh {
		t.Errorf("expected SeverityHigh, got %s", inc.Severity)
	}
	if inc.PodName != "auth-srv" {
		t.Errorf("expected PodName auth-srv, got %s", inc.PodName)
	}
	if inc.RawData["crash_count"] != "3" {
		t.Errorf("expected crash_count 3, got %s", inc.RawData["crash_count"])
	}
}

func TestDockerEventWatcher_CleanExit_Ignored(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, zap.NewNop())

	cleanExitMsg := events.Message{
		Type:   "container",
		Action: "die",
		Actor: events.Actor{
			ID: "c-clean",
			Attributes: map[string]string{
				"name":                      "/job-worker-1",
				"com.docker.compose.service": "job-worker",
				"exitCode":                  "0",
			},
		},
		Time: time.Now().Unix(),
	}

	for i := 0; i < 5; i++ {
		watcher.ProcessEvent(context.Background(), cleanExitMsg)
	}

	if len(mockRepo.created) != 0 {
		t.Errorf("expected 0 incidents on clean exit 0, got %d", len(mockRepo.created))
	}
}

func TestDockerEventWatcher_ServiceUnhealthy(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, zap.NewNop())

	mockAPI.logsMap["c-health"] = "Health probe /healthz returned 503 Service Unavailable"

	healthMsg := events.Message{
		Type:   "container",
		Action: "health_status: unhealthy",
		Actor: events.Actor{
			ID: "c-health",
			Attributes: map[string]string{
				"name":                         "/gateway-1",
				"com.docker.swarm.service.name": "gateway",
				"health_status":                "unhealthy",
				"image":                        "gateway:latest",
			},
		},
		Time: time.Now().Unix(),
	}

	watcher.ProcessEvent(context.Background(), healthMsg)

	if len(mockRepo.created) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(mockRepo.created))
	}

	inc := mockRepo.created[0]
	if inc.Type != incident.TypeServiceUnhealthy {
		t.Errorf("expected TypeServiceUnhealthy, got %s", inc.Type)
	}
	if inc.Severity != incident.SeverityHigh {
		t.Errorf("expected SeverityHigh, got %s", inc.Severity)
	}
	if inc.PodName != "gateway" {
		t.Errorf("expected PodName gateway, got %s", inc.PodName)
	}
}

func TestDockerEventWatcher_AutoResolve_OnStart(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, zap.NewNop())

	ctx := context.Background()

	// 1. Create an existing active incident for order-service
	inc, _ := incident.New("fleet-primary", "default", "order-service", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "CrashLoopBackOff detected")
	_ = mockRepo.Create(ctx, inc)

	// 2. Mock all containers for order-service as running
	mockAPI.containers = []container.Summary{
		{
			ID:     "c-order-1",
			Names:  []string{"/order-service-1"},
			State:  "running",
			Status: "Up 10 seconds (healthy)",
			Labels: map[string]string{
				"com.docker.compose.service": "order-service",
			},
		},
	}

	// 3. Send "start" event for container
	startMsg := events.Message{
		Type:   "container",
		Action: "start",
		Actor: events.Actor{
			ID: "c-order-1",
			Attributes: map[string]string{
				"name":                      "/order-service-1",
				"com.docker.compose.service": "order-service",
			},
		},
		Time: time.Now().Unix(),
	}

	watcher.ProcessEvent(ctx, startMsg)

	// 4. Verify incident was marked resolved and updated in repo
	if len(mockRepo.updated) != 1 {
		t.Fatalf("expected 1 updated incident on auto-resolve, got %d", len(mockRepo.updated))
	}

	resolvedInc := mockRepo.updated[0]
	if resolvedInc.Status != incident.StatusResolved {
		t.Errorf("expected StatusResolved, got %s", resolvedInc.Status)
	}
	if resolvedInc.ResolvedAt == nil {
		t.Errorf("expected ResolvedAt timestamp to be set")
	}

	// Verify broadcast
	msgs := mockBC.getMessages()
	foundResolved := false
	for _, m := range msgs {
		if m.MsgType == "incident_resolved" {
			foundResolved = true
			break
		}
	}
	if !foundResolved {
		t.Errorf("expected 'incident_resolved' broadcast message, got %v", msgs)
	}
}

func TestDockerEventWatcher_AutoResolve_NotAllRunning(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, zap.NewNop())
	ctx := context.Background()

	// Existing incident
	inc, _ := incident.New("fleet-primary", "default", "order-service", incident.TypeCrashLoopBackOff, incident.SeverityHigh, "CrashLoopBackOff")
	_ = mockRepo.Create(ctx, inc)

	// 1 container running, 1 exited
	mockAPI.containers = []container.Summary{
		{
			ID:     "c1",
			Names:  []string{"/order-service.1.task1"},
			State:  "running",
			Labels: map[string]string{"com.docker.swarm.service.name": "order-service"},
		},
		{
			ID:     "c2",
			Names:  []string{"/order-service.2.task2"},
			State:  "exited",
			Labels: map[string]string{"com.docker.swarm.service.name": "order-service"},
		},
	}

	startMsg := events.Message{
		Type:   "container",
		Action: "start",
		Actor: events.Actor{
			ID: "c1",
			Attributes: map[string]string{
				"name":                         "/order-service.1.task1",
				"com.docker.swarm.service.name": "order-service",
			},
		},
		Time: time.Now().Unix(),
	}

	watcher.ProcessEvent(ctx, startMsg)

	// Should NOT be resolved because c2 is exited
	if len(mockRepo.updated) != 0 {
		t.Errorf("expected 0 resolved incidents when not all containers are running, got %d", len(mockRepo.updated))
	}
}

func TestDockerEventWatcher_Deduplication(t *testing.T) {
	mockAPI := newMockDockerEventAPI()
	mockRepo := newMockIncidentRepo()
	mockBC := &mockBroadcaster{}

	watcher := NewDockerEventWatcher(mockAPI, mockRepo, mockBC, zap.NewNop())

	oomMsg := events.Message{
		Type:   "container",
		Action: "oom",
		Actor: events.Actor{
			ID: "c-oom",
			Attributes: map[string]string{
				"name":                         "/cache-1",
				"com.docker.swarm.service.name": "cache",
			},
		},
		Time: time.Now().Unix(),
	}

	ctx := context.Background()
	// Trigger first OOM
	watcher.ProcessEvent(ctx, oomMsg)
	// Trigger second OOM while first is still active
	watcher.ProcessEvent(ctx, oomMsg)

	if len(mockRepo.created) != 1 {
		t.Errorf("expected exactly 1 incident created due to deduplication, got %d", len(mockRepo.created))
	}
}

func TestExtractServiceNameAndNamespace(t *testing.T) {
	// Swarm label
	labels1 := map[string]string{
		"com.docker.swarm.service.name": "web-fe",
		"com.docker.stack.namespace":    "prod",
	}
	if name := ExtractServiceName(labels1, "container-1"); name != "web-fe" {
		t.Errorf("expected web-fe, got %s", name)
	}
	if ns := ExtractNamespace(labels1, "default"); ns != "prod" {
		t.Errorf("expected prod, got %s", ns)
	}

	// Compose label
	labels2 := map[string]string{
		"com.docker.compose.service": "db",
		"com.docker.compose.project": "myproject",
	}
	if name := ExtractServiceName(labels2, "container-2"); name != "db" {
		t.Errorf("expected db, got %s", name)
	}
	if ns := ExtractNamespace(labels2, "default"); ns != "myproject" {
		t.Errorf("expected myproject, got %s", ns)
	}

	// Fallback to container name
	if name := ExtractServiceName(nil, "/standalone-nginx"); name != "standalone-nginx" {
		t.Errorf("expected standalone-nginx, got %s", name)
	}
	if ns := ExtractNamespace(nil, "custom-ns"); ns != "custom-ns" {
		t.Errorf("expected custom-ns, got %s", ns)
	}
}
