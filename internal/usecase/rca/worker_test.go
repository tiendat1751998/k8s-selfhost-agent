package rca

import (
	"context"
	"testing"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
)

type mockWSBroadcaster struct {
	msgType string
	data    interface{}
}

func (m *mockWSBroadcaster) Broadcast(msgType string, data interface{}) {
	m.msgType = msgType
	m.data = data
}

type mockIncidentRepo struct {
	incidents map[string]*incident.Incident
}

func (m *mockIncidentRepo) Create(ctx context.Context, inc *incident.Incident) error {
	m.incidents[inc.ID] = inc
	return nil
}

func (m *mockIncidentRepo) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	return m.incidents[id], nil
}

func (m *mockIncidentRepo) GetByPodAndType(ctx context.Context, namespace, podName string, incType incident.Type) (*incident.Incident, error) {
	return nil, nil
}

func (m *mockIncidentRepo) Update(ctx context.Context, inc *incident.Incident) error {
	m.incidents[inc.ID] = inc
	return nil
}

func (m *mockIncidentRepo) List(ctx context.Context, filter incident.Filter) ([]*incident.Incident, int64, error) {
	return nil, 0, nil
}

func TestNewWorker(t *testing.T) {
	ws := &mockWSBroadcaster{}
	repo := &mockIncidentRepo{incidents: make(map[string]*incident.Incident)}
	
	worker := NewWorker(nil, nil, repo, ws)
	if worker == nil {
		t.Fatal("expected non-nil worker")
	}
	if worker.incRepo != repo {
		t.Error("worker repo not set correctly")
	}
	if worker.wsHub != ws {
		t.Error("worker wsHub not set correctly")
	}
}
