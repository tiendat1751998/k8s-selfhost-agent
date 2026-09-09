package alert_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainAlert "github.com/datdt/k8sselfhost/internal/domain/alert"
	usecaseAlert "github.com/datdt/k8sselfhost/internal/usecase/alert"
	"go.uber.org/zap"
)

type mockRepo struct {
	rules    []*domainAlert.AlertRule
	channels []*domainAlert.NotificationChannel
	history  []*domainAlert.AlertHistory
	mu       sync.Mutex
}

func (m *mockRepo) CreateChannel(ctx context.Context, channel *domainAlert.NotificationChannel) error { return nil }
func (m *mockRepo) ListChannels(ctx context.Context, tenantID string) ([]*domainAlert.NotificationChannel, error) {
	return m.channels, nil
}
func (m *mockRepo) CreateRule(ctx context.Context, rule *domainAlert.AlertRule) error { return nil }
func (m *mockRepo) ListRules(ctx context.Context, tenantID string) ([]*domainAlert.AlertRule, error) {
	return m.rules, nil
}
func (m *mockRepo) UpdateRule(ctx context.Context, rule *domainAlert.AlertRule) error { return nil }
func (m *mockRepo) DeleteRule(ctx context.Context, id, tenantID string) error        { return nil }
func (m *mockRepo) CreateHistory(ctx context.Context, h *domainAlert.AlertHistory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = append(m.history, h)
	return nil
}
func (m *mockRepo) ListHistory(ctx context.Context, tenantID string) ([]*domainAlert.AlertHistory, error) {
	return m.history, nil
}
func (m *mockRepo) AcknowledgeAlert(ctx context.Context, id, tenantID, userID string) error { return nil }

type mockNotifier struct {
	sendErr error
	called  chan struct{}
}

func (n *mockNotifier) Send(ctx context.Context, channel *domainAlert.NotificationChannel, message string) error {
	if n.called != nil {
		select {
		case n.called <- struct{}{}:
		default:
		}
	}
	return n.sendErr
}

func TestNewRuleEngine_NilRepo(t *testing.T) {
	engine, err := usecaseAlert.NewRuleEngine(nil, nil)
	if err == nil {
		t.Fatalf("expected error when repo is nil, got nil")
	}
	if engine != nil {
		t.Errorf("expected engine to be nil on error")
	}
}

func TestNewRuleEngine_Success(t *testing.T) {
	repo := &mockRepo{}
	engine, err := usecaseAlert.NewRuleEngine(repo, nil, usecaseAlert.WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine == nil {
		t.Fatalf("expected non-nil engine")
	}
}

func TestRuleEngine_EvaluateRule_NotifierErrorHandled(t *testing.T) {
	notifierCalled := make(chan struct{}, 1)
	repo := &mockRepo{
		rules: []*domainAlert.AlertRule{
			{
				ID:         "rule-1",
				TenantID:   "tenant-1",
				Name:       "High CPU",
				MetricName: "cpu_usage",
				Condition:  "gt",
				Threshold:  80.0,
				ChannelIDs: []string{"ch-1"},
				Enabled:    true,
			},
		},
		channels: []*domainAlert.NotificationChannel{
			{
				ID:      "ch-1",
				Type:    "slack",
				Enabled: true,
			},
		},
	}
	notifiers := map[string]domainAlert.Notifier{
		"slack": &mockNotifier{
			sendErr: errors.New("slack webhook failed"),
			called:  notifierCalled,
		},
	}

	engine, err := usecaseAlert.NewRuleEngine(repo, notifiers, usecaseAlert.WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatalf("unexpected error creating engine: %v", err)
	}

	err = engine.EvaluateRule(context.Background(), "cpu_usage", 95.0, "tenant-1")
	if err != nil {
		t.Fatalf("EvaluateRule returned error: %v", err)
	}

	select {
	case <-notifierCalled:
		// success: notifier was invoked and error was handled/logged
	case <-time.After(2 * time.Second):
		t.Fatalf("notifier was not called within timeout")
	}
}
