package telegram_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	"go.uber.org/zap"
)

type mockLogFetcher struct {
	logs string
	err  error
}

func (m *mockLogFetcher) GetPodLogs(ctx context.Context, namespace, pod string, tailLines int64) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.logs, nil
}

func TestTelegramBot_RollbackCallback_ExecutesRollback(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := &MockBotClient{}

	ns := "default"
	depName := "payment-service"

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName,
			Namespace: ns,
			Annotations: map[string]string{
				telegram.RevisionAnnotation: "2",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "payment:v2.0.0"},
					},
				},
			},
		},
	}

	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName + "-rs-1",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: depName},
			},
			Annotations: map[string]string{
				telegram.RevisionAnnotation: "1",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "payment:v1.0.0"},
					},
				},
			},
		},
	}

	rs2 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName + "-rs-2",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: depName},
			},
			Annotations: map[string]string{
				telegram.RevisionAnnotation: "2",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "payment:v2.0.0"},
					},
				},
			},
		},
	}

	fakeK8s := fake.NewSimpleClientset(dep, rs1, rs2)

	cfg := telegram.TelegramConfig{
		BotToken:     "test-token",
		AdminChatIDs: []int64{999},
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)
	bot.SetK8sClient(fakeK8s)

	callbackData := fmt.Sprintf("rollback:kubernetes:prod-us-east:%s:%s", ns, depName)
	res, err := bot.HandleCallbackQuery(context.Background(), "cb-rollback-1", 999, callbackData)
	if err != nil {
		t.Fatalf("unexpected error executing rollback callback: %v", err)
	}

	if !strings.Contains(res, "thành công về Revision 1") {
		t.Errorf("expected success message containing 'Revision 1', got: %s", res)
	}

	// Verify K8s deployment template was updated to Revision 1 (image payment:v1.0.0)
	updatedDep, err := fakeK8s.AppsV1().Deployments(ns).Get(context.Background(), depName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated deployment: %v", err)
	}
	if len(updatedDep.Spec.Template.Spec.Containers) == 0 {
		t.Fatal("expected containers in rolled back deployment")
	}
	if updatedDep.Spec.Template.Spec.Containers[0].Image != "payment:v1.0.0" {
		t.Errorf("expected rolled back image to be payment:v1.0.0, got: %s", updatedDep.Spec.Template.Spec.Containers[0].Image)
	}

	// Verify telegram chat received status message
	mockClient.mu.Lock()
	hasMsg := false
	for _, m := range mockClient.messages {
		if strings.Contains(m, "Rollback deployment default/payment-service thành công") {
			hasMsg = true
			break
		}
	}
	mockClient.mu.Unlock()

	if !hasMsg {
		t.Error("expected chat notification message sent to Telegram")
	}
}

func TestTelegramBot_RollbackCallback_NoPreviousRevision(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := &MockBotClient{}

	ns := "default"
	depName := "single-rev-service"

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName,
			Namespace: ns,
			Annotations: map[string]string{
				telegram.RevisionAnnotation: "1",
			},
		},
	}
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      depName + "-rs-1",
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: depName},
			},
			Annotations: map[string]string{
				telegram.RevisionAnnotation: "1",
			},
		},
	}

	fakeK8s := fake.NewSimpleClientset(dep, rs1)

	cfg := telegram.TelegramConfig{
		BotToken:     "test-token",
		AdminChatIDs: []int64{999},
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)
	bot.SetK8sClient(fakeK8s)

	callbackData := fmt.Sprintf("rollback:%s:%s", ns, depName)
	_, err := bot.HandleCallbackQuery(context.Background(), "cb-rollback-fail", 999, callbackData)
	if err == nil {
		t.Fatal("expected error when no previous revision exists")
	}
	if !strings.Contains(err.Error(), "No previous revision found") {
		t.Errorf("expected 'No previous revision found' in error, got: %v", err)
	}
}

func TestTelegramBot_ViewLogsCallback_ReturnsFormattedLogs(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := &MockBotClient{}

	// Generate 60 lines with ANSI colored codes and content
	var sb strings.Builder
	for i := 1; i <= 60; i++ {
		sb.WriteString(fmt.Sprintf("\x1b[31m[ERROR]\x1b[0m 2026-10-12T10:00:%02dZ Line %d: connection reset by peer\n", i, i))
	}

	cfg := telegram.TelegramConfig{
		BotToken:     "test-token",
		AdminChatIDs: []int64{999},
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)
	bot.SetLogFetcher(&mockLogFetcher{logs: sb.String()})

	res, err := bot.HandleCallbackQuery(context.Background(), "cb-logs-1", 999, "logs:backend:auth-pod-123")
	if err != nil {
		t.Fatalf("unexpected error fetching logs: %v", err)
	}

	// Must be wrapped in codeblock pre formatting
	if !strings.Contains(res, "<pre>") || !strings.Contains(res, "</pre>") {
		t.Errorf("expected HTML/Markdown preformatted code block <pre>...</pre>, got: %s", res)
	}

	// Must strip ANSI
	if strings.Contains(res, "\x1b[31m") || strings.Contains(res, "\x1b[0m") {
		t.Errorf("expected ANSI escape codes to be stripped, got: %s", res)
	}

	// Must contain last line
	if !strings.Contains(res, "Line 60") {
		t.Error("expected tail logs to include line 60")
	}

	// Length must be under Telegram limit (4096 / 4000)
	if len(res) > 4000 {
		t.Errorf("expected log message to be capped under 4000 characters, got %d", len(res))
	}
}

func TestTelegramBot_UnauthorizedDestructiveActions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := &MockBotClient{}

	cfg := telegram.TelegramConfig{
		BotToken:     "test-token",
		AdminChatIDs: []int64{999}, // only 999 is admin
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)

	destructiveActions := []string{
		"restart:kubernetes:prod:default:auth",
		"rollback:kubernetes:prod:default:auth",
		"logs:default:auth-pod-1",
	}

	for _, action := range destructiveActions {
		t.Run(action, func(t *testing.T) {
			_, err := bot.HandleCallbackQuery(context.Background(), "cb-unauth", 111, action)
			if err == nil {
				t.Fatalf("expected unauthorized error for non-whitelisted user 111 on action %s", action)
			}
		})
	}
}

func TestTelegramBot_BuildInlineMarkup_WithLogsButton(t *testing.T) {
	bot := telegram.NewSREBot(telegram.TelegramConfig{}, nil, nil, nil, nil)
	alert := &telegram.AlertPayload{
		Namespace: "prod-payments",
		Service:   "checkout",
		Pod:       "checkout-6b78d-p4x",
	}

	markup := bot.BuildInlineMarkup(alert)
	m, ok := markup.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", markup)
	}

	keyboard, ok := m["inline_keyboard"].([][]map[string]string)
	if !ok {
		t.Fatalf("expected [][]map[string]string, got %T", m["inline_keyboard"])
	}

	foundLogsBtn := false
	for _, row := range keyboard {
		for _, btn := range row {
			if strings.Contains(btn["text"], "Xem 50 Dòng Log") {
				foundLogsBtn = true
				expectedCallback := "logs:prod-payments:checkout-6b78d-p4x"
				if btn["callback_data"] != expectedCallback {
					t.Errorf("expected callback_data %q, got %q", expectedCallback, btn["callback_data"])
				}
			}
		}
	}

	if !foundLogsBtn {
		t.Error("expected 'Xem 50 Dòng Log' inline button to be present in keyboard")
	}
}
