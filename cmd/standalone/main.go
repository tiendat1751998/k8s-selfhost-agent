// Package main is a standalone development server for the K8S Self-Healing dashboard.
// It runs WITHOUT PostgreSQL, Redis, or NATS — perfect for frontend development.
// Usage: go run ./cmd/standalone
package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/infrastructure/llm"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraCluster "github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	usecaseAgent "github.com/datdt/k8sselfhost/internal/usecase/agent"
	"github.com/datdt/k8sselfhost/pkg/health"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

var mockProviders = []struct {
	Name     string
	Type     string
	Model    string
	Endpoint string
	Default  bool
}{
	{"OpenAI GPT-4o", "openai", "gpt-4o", "https://api.openai.com/v1", true},
	{"OpenAI GPT-4 Turbo", "openai", "gpt-4-turbo", "https://api.openai.com/v1", false},
	{"OpenAI GPT-3.5 Turbo", "openai", "gpt-3.5-turbo", "https://api.openai.com/v1", false},
	{"Azure OpenAI GPT-4", "openai", "azure/gpt-4", "https://corp.openai.azure.com", false},
	{"Anthropic Claude 3.5 Sonnet", "anthropic", "claude-3-5-sonnet", "https://api.anthropic.com/v1", false},
	{"Anthropic Claude 3 Opus", "anthropic", "claude-3-opus", "https://api.anthropic.com/v1", false},
	{"Anthropic Claude 3 Haiku", "anthropic", "claude-3-haiku", "https://api.anthropic.com/v1", false},
	{"Google Gemini 1.5 Pro", "gemini", "gemini-1.5-pro", "https://generativelanguage.googleapis.com", false},
	{"Google Gemini 1.5 Flash", "gemini", "gemini-1.5-flash", "https://generativelanguage.googleapis.com", false},
	{"Google Vertex AI Gemini", "gemini", "vertex/gemini-pro", "https://us-central1-aiplatform.googleapis.com", false},
	{"AWS Bedrock Claude v3", "bedrock", "bedrock/claude-v3", "https://bedrock-runtime.us-east-1.amazonaws.com", false},
	{"AWS Bedrock Llama 3 70B", "bedrock", "bedrock/llama3-70b", "https://bedrock-runtime.us-east-1.amazonaws.com", false},
	{"Mistral Large 2", "mistral", "mistral-large-latest", "https://api.mistral.ai/v1", false},
	{"Mistral Codestral", "mistral", "codestral-latest", "https://api.mistral.ai/v1", false},
	{"Groq Llama 3.1 70B", "groq", "llama3-70b-8192", "https://api.groq.com/openai/v1", false},
	{"Groq Mixtral 8x7B", "groq", "mixtral-8x7b-32768", "https://api.groq.com/openai/v1", false},
	{"DeepSeek Coder V2", "deepseek", "deepseek-coder", "https://api.deepseek.com", false},
	{"Cohere Command R+", "cohere", "command-r-plus", "https://api.cohere.ai/v1", false},
	{"Ollama Local Llama 3", "ollama", "llama3:8b", "http://localhost:11434", false},
	{"vLLM Cluster Codellama", "vllm", "codellama:34b", "http://vllm.internal:8000", false},
}

var (
	incidentTypes = []string{"CrashLoopBackOff", "OOMKilled", "ImagePullBackOff", "ProbeFailed", "FailedScheduling", "NodeNotReady"}
	severities    = []string{"critical", "warning", "info"}
	namespaces    = []string{"default", "production", "staging", "monitoring", "kube-system"}
	clusters      = []string{"prod-us-east", "prod-eu-west", "staging-1", "dev-cluster"}
	podPrefixes   = []string{"api-server", "payment-svc", "auth-proxy", "data-pipeline", "worker", "scheduler", "gateway"}
	steps         = []string{"TASK", "LLM", "CODE", "TEST", "FIX", "COMMIT", "PR"}

	logMessages = []string{
		"kubectl: pod api-server-7b8f9 restarted due to OOMKilled",
		"watcher: detected CrashLoopBackOff on payment-svc-4d2e1",
		"rca: analyzing incident inc-a8f3b2 with ollama/llama3",
		"rca: root cause identified — memory limit exceeded (confidence: 0.87)",
		"gitops: creating branch fix/inc-a8f3b2",
		"gitops: PR #142 created — increase memory limit to 512Mi",
		"watcher: node ip-10-0-1-42 reporting NotReady condition",
		"collector: gathering pod logs for auth-proxy-9c1d3",
		"collector: fetching node metrics — CPU 78%, Memory 92%",
		"rca: generating remediation YAML patch",
		"health: all pods in monitoring namespace healthy",
		"metrics: cluster CPU utilization at 67%, memory at 84%",
	}
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Validate encryption key
	if err := crypto.ValidateKey(); err != nil {
		return fmt.Errorf("cryptography validation: %w", err)
	}

	logger.Init("debug")
	defer logger.Sync()
	log := logger.Get()

	log.Info("🚀 Starting K8S Control Plane (standalone mode)",
		zap.String("version", "0.1.0"),
		zap.String("mode", "standalone-enterprise"),
	)

	healthHandler := health.NewHandler(5 * time.Second)
	wsHub := adapthttp.NewWSHub()
	go wsHub.Run()

	// Initialize LLM Provider Registry
	registry := llm.NewProviderRegistry()

	for _, mp := range mockProviders {
		registry.Register(mp.Name, &mockLLMClient{model: mp.Model}, llm.ProviderInfo{
			Type:     mp.Type,
			Model:    mp.Model,
			Endpoint: mp.Endpoint,
			Default:  mp.Default,
		})
	}

	dsn := "postgres://myuser:mysecretpassword@10.10.10.133:5432/mydatabase?sslmode=disable"
	pgClient, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pgClient.Close()

	dockerRepo, err := infraDocker.NewRealDockerRepo("tcp://10.10.10.133:2375", "1.41")
	if err != nil {
		log.Fatal("failed to initialize docker repo", zap.Error(err))
	}

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			kubeconfigPath = home + "\\.kube\\config"
		} else if home = os.Getenv("HOME"); home != "" {
			kubeconfigPath = home + "/.kube/config"
		}
	}
	k8sClient, err := infraK8s.NewClient(kubeconfigPath)
	if err != nil {
		log.Warn("failed to initialize Kubernetes client, some features will use mocks", zap.Error(err))
	}
	auditRepo := postgres.NewAuditRepo(pgClient)

	fleetRepo := postgres.NewFleetRepo(pgClient)
	clientManager := infraCluster.NewClientManager(fleetRepo)

	agentRepo := postgres.NewAgentRepo(pgClient)
	defaultLLM, _ := registry.Default()
	bridge := &wsBridge{hub: wsHub}
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge)

	platformHandlers := &adapthttp.PlatformHandlers{
		Docker:        adapthttp.NewDockerHandler(dockerRepo),
		Drift:         adapthttp.NewDriftHandler(postgres.NewDriftRepo(pgClient)),
		Correlation:   adapthttp.NewCorrelationHandler(postgres.NewCorrelationRepo(pgClient)),
		Compliance:    adapthttp.NewComplianceHandler(postgres.NewComplianceRepo(pgClient)),
		Tagging:       adapthttp.NewTaggingHandler(postgres.NewTaggingRepo(pgClient)),
		Runbook:       adapthttp.NewRunbookHandler(postgres.NewRunbookRepo(pgClient), auditRepo),
		Observability: adapthttp.NewObservabilityHandler(postgres.NewObservabilityRepo(pgClient)),
		Capacity:      adapthttp.NewCapacityHandler(infraK8s.NewCapacityRepo(k8sClient, clientManager)),
		Changes:       adapthttp.NewChangeHandler(postgres.NewChangesRepo(pgClient)),
		Promotion:     adapthttp.NewPromotionHandler(postgres.NewPromotionRepo(pgClient)),
		Explorer:      adapthttp.NewExplorerHandler(infraK8s.NewExplorerRepo(k8sClient, dockerRepo, clientManager)),
		Reporting:     adapthttp.NewReportingHandler(postgres.NewReportingRepo(pgClient)),
		HealthCenter:  adapthttp.NewHealthCenterHandler(infraK8s.NewHealthCenterRepo(k8sClient, clientManager)),
		Fleet:         adapthttp.NewFleetHandler(fleetRepo, auditRepo),
		Audit:         adapthttp.NewAuditHandler(auditRepo),
		Notification:  adapthttp.NewNotificationHandler(postgres.NewNotificationRepo(pgClient)),
		Automation:    adapthttp.NewAutomationHandler(postgres.NewAutomationRepo(pgClient)),
		Timeline:      adapthttp.NewTimelineHandler(postgres.NewTimelineRepo(pgClient)),
		AI:            adapthttp.NewAIHandler(registry),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
	}

	router := adapthttp.NewRouterWithWS(healthHandler, wsHub, platformHandlers)


	addr := ":8080"
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("✅ Dashboard available at http://localhost" + addr)
		log.Info("📡 WebSocket endpoint: ws://localhost" + addr + "/ws")
		if listenErr := srv.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", listenErr)
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down HTTP server: %w", err)
	}
	log.Info("server stopped gracefully")
	return nil
}

// generateOperationalData sends incidents, agent events, and logs.
func generateOperationalData(ctx context.Context, hub *adapthttp.WSHub) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
	}

	// Initial burst
	for i := 0; i < 3; i++ {
		hub.Broadcast(adapthttp.WSMessage{Type: "log", Data: logMessages[rng.Intn(len(logMessages))]})
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}

	incidentTicker := time.NewTicker(5 * time.Second)
	logTicker := time.NewTicker(2 * time.Second)
	agentTicker := time.NewTicker(10 * time.Second)
	defer incidentTicker.Stop()
	defer logTicker.Stop()
	defer agentTicker.Stop()

	agentStep := 0
	agentRunning := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-incidentTicker.C:
			if hub.ClientCount() == 0 {
				continue
			}
			pod := podPrefixes[rng.Intn(len(podPrefixes))] + "-" + randomHex(5, rng)
			hub.Broadcast(adapthttp.WSMessage{
				Type: "incident",
				Data: map[string]interface{}{
					"type": incidentTypes[rng.Intn(len(incidentTypes))], "severity": severities[rng.Intn(len(severities))],
					"cluster": clusters[rng.Intn(len(clusters))], "namespace": namespaces[rng.Intn(len(namespaces))],
					"podName": pod, "timestamp": time.Now().UTC().Format(time.RFC3339),
				},
			})
		case <-logTicker.C:
			if hub.ClientCount() == 0 {
				continue
			}
			hub.Broadcast(adapthttp.WSMessage{Type: "log", Data: logMessages[rng.Intn(len(logMessages))]})
		case <-agentTicker.C:
			if hub.ClientCount() == 0 {
				continue
			}
			if !agentRunning {
				agentRunning = true
				agentStep = 0
			}
			if agentStep < len(steps) {
				step := steps[agentStep]
				dur := rng.Intn(3000) + 500
				hub.Broadcast(adapthttp.WSMessage{Type: "agent", Data: map[string]interface{}{"step": step, "status": "running", "duration": dur}})
				go func(s string, d int) {
					// Use a separate select with ctx to prevent hanging after shutdown
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Duration(d) * time.Millisecond):
					}
					
					// Local rng for concurrent safety
					localRng := rand.New(rand.NewSource(time.Now().UnixNano()))
					st := "success"
					if localRng.Float64() < 0.08 {
						st = "failed"
					}
					// Verify context isn't cancelled before broadcasting
					select {
					case <-ctx.Done():
						return
					default:
						hub.Broadcast(adapthttp.WSMessage{Type: "agent", Data: map[string]interface{}{"step": s, "status": st, "duration": d}})
					}
				}(step, dur)
				agentStep++
			} else {
				agentRunning = false
			}
		}
	}
}

// generateEnterpriseData sends config data on connect and periodic health updates.
func generateEnterpriseData(ctx context.Context, hub *adapthttp.WSHub) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + 1)) // Add 1 to ensure different seed

	select {
	case <-ctx.Done():
		return
	case <-time.After(3 * time.Second):
	}

	configData := map[string]interface{}{
		"kubernetes": []map[string]interface{}{
			{"name": "prod-us-east", "provider": "kubernetes", "endpoint": "https://k8s-prod-us.internal:6443", "status": "healthy", "lastCheck": time.Now().UTC().Format(time.RFC3339), "nodes": 12, "pods": 347, "services": 28, "cpu": "67%", "memory": "72%"},
			{"name": "prod-eu-west", "provider": "kubernetes", "endpoint": "https://k8s-prod-eu.internal:6443", "status": "healthy", "lastCheck": time.Now().UTC().Format(time.RFC3339), "nodes": 8, "pods": 215, "services": 19, "cpu": "45%", "memory": "58%"},
			{"name": "staging-1", "provider": "kubernetes", "endpoint": "https://k8s-staging.internal:6443", "status": "degraded", "lastCheck": time.Now().UTC().Format(time.RFC3339), "nodes": 4, "pods": 89, "services": 11, "cpu": "82%", "memory": "91%"},
			{"name": "docker-prod", "provider": "docker", "endpoint": "tcp://docker-prod.internal:2376", "socketPath": "/var/run/docker.sock", "status": "healthy", "lastCheck": time.Now().UTC().Format(time.RFC3339), "nodes": 1, "containers": 24, "services": 0, "cpu": "38%", "memory": "52%", "dockerVersion": "27.1.2"},
			{"name": "swarm-cluster", "provider": "docker_swarm", "endpoint": "tcp://swarm-mgr.internal:2377", "status": "healthy", "lastCheck": time.Now().UTC().Format(time.RFC3339), "nodes": 5, "containers": 67, "services": 12, "cpu": "55%", "memory": "63%", "dockerVersion": "27.1.2", "swarmManager": true},
			{"name": "dev-docker", "provider": "docker", "endpoint": "unix:///var/run/docker.sock", "socketPath": "/var/run/docker.sock", "status": "healthy", "lastCheck": time.Now().UTC().Format(time.RFC3339), "nodes": 1, "containers": 8, "services": 0, "cpu": "12%", "memory": "28%", "dockerVersion": "26.1.4"},
		},
		"gitProviders": []map[string]interface{}{
			{"type": "github", "organization": "datdt-corp", "repoCount": 47, "syncStatus": "synced", "lastWebhook": time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)},
			{"type": "gitlab", "organization": "datdt-infra", "repoCount": 23, "syncStatus": "synced", "lastWebhook": time.Now().Add(-15 * time.Minute).UTC().Format(time.RFC3339)},
			{"type": "gitea", "organization": "internal-tools", "repoCount": 8, "syncStatus": "syncing", "lastWebhook": time.Now().Add(-45 * time.Minute).UTC().Format(time.RFC3339)},
		},
		"cicd": []map[string]interface{}{
			{"name": "deploy-prod", "provider": "GitHub Actions", "status": "active", "successRate": 96, "lastRun": time.Now().Add(-8 * time.Minute).UTC().Format(time.RFC3339), "lastLog": "✓ Build passed\n✓ Tests passed (142/142)\n✓ Image pushed: k8s-agent:v1.2.3\n✓ Deployed to prod-us-east\n✓ Health check OK"},
			{"name": "deploy-staging", "provider": "GitHub Actions", "status": "active", "successRate": 89, "lastRun": time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339), "lastLog": "✓ Build passed\n✓ Tests passed\n✓ Deployed to staging-1"},
			{"name": "security-scan", "provider": "GitLab CI", "status": "active", "successRate": 100, "lastRun": time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339), "lastLog": "✓ SAST scan completed\n✓ No critical vulnerabilities\n⚠ 2 medium severity findings"},
			{"name": "backup-db", "provider": "Jenkins", "status": "failing", "successRate": 45, "lastRun": time.Now().Add(-30 * time.Minute).UTC().Format(time.RFC3339), "lastLog": "✓ Started backup job\n✗ Connection to pg-primary timed out\n✗ Backup failed after 3 retries"},
		},
		"aiProviders": []map[string]interface{}{
			{"id": "openai-gpt4o", "name": "OpenAI GPT-4o", "model": "gpt-4o", "endpoint": "https://api.openai.com/v1", "status": "healthy", "latency": "340ms", "tokenUsage": "1,245,200", "default": true},
			{"id": "openai-gpt4-turbo", "name": "OpenAI GPT-4 Turbo", "model": "gpt-4-turbo", "endpoint": "https://api.openai.com/v1", "status": "healthy", "latency": "380ms", "tokenUsage": "450,120"},
			{"id": "openai-gpt35", "name": "OpenAI GPT-3.5 Turbo", "model": "gpt-3.5-turbo", "endpoint": "https://api.openai.com/v1", "status": "healthy", "latency": "190ms", "tokenUsage": "1,890,430"},
			{"id": "azure-openai", "name": "Azure OpenAI GPT-4", "model": "azure/gpt-4", "endpoint": "https://corp.openai.azure.com", "status": "healthy", "latency": "220ms", "tokenUsage": "890,210"},
			{"id": "anthropic-sonnet", "name": "Anthropic Claude 3.5 Sonnet", "model": "claude-3-5-sonnet", "endpoint": "https://api.anthropic.com/v1", "status": "healthy", "latency": "410ms", "tokenUsage": "2,130,450"},
			{"id": "anthropic-opus", "name": "Anthropic Claude 3 Opus", "model": "claude-3-opus", "endpoint": "https://api.anthropic.com/v1", "status": "healthy", "latency": "780ms", "tokenUsage": "120,450"},
			{"id": "anthropic-haiku", "name": "Anthropic Claude 3 Haiku", "model": "claude-3-haiku", "endpoint": "https://api.anthropic.com/v1", "status": "healthy", "latency": "150ms", "tokenUsage": "3,450,120"},
			{"id": "google-gemini-pro", "name": "Google Gemini 1.5 Pro", "model": "gemini-1.5-pro", "endpoint": "https://generativelanguage.googleapis.com", "status": "healthy", "latency": "290ms", "tokenUsage": "670,120"},
			{"id": "google-gemini-flash", "name": "Google Gemini 1.5 Flash", "model": "gemini-1.5-flash", "endpoint": "https://generativelanguage.googleapis.com", "status": "healthy", "latency": "110ms", "tokenUsage": "12,450,210"},
			{"id": "google-vertex", "name": "Google Vertex AI Gemini", "model": "vertex/gemini-pro", "endpoint": "https://us-central1-aiplatform.googleapis.com", "status": "healthy", "latency": "310ms", "tokenUsage": "450,110"},
			{"id": "aws-bedrock-claude", "name": "AWS Bedrock Claude v3", "model": "bedrock/claude-v3", "endpoint": "https://bedrock-runtime.us-east-1.amazonaws.com", "status": "healthy", "latency": "450ms", "tokenUsage": "320,400"},
			{"id": "aws-bedrock-llama", "name": "AWS Bedrock Llama 3 70B", "model": "bedrock/llama3-70b", "endpoint": "https://bedrock-runtime.us-east-1.amazonaws.com", "status": "healthy", "latency": "390ms", "tokenUsage": "140,250"},
			{"id": "mistral-large", "name": "Mistral Large 2", "model": "mistral-large-latest", "endpoint": "https://api.mistral.ai/v1", "status": "healthy", "latency": "420ms", "tokenUsage": "90,450"},
			{"id": "mistral-codestral", "name": "Mistral Codestral", "model": "codestral-latest", "endpoint": "https://api.mistral.ai/v1", "status": "healthy", "latency": "230ms", "tokenUsage": "890,110"},
			{"id": "groq-llama", "name": "Groq Llama 3.1 70B", "model": "llama3-70b-8192", "endpoint": "https://api.groq.com/openai/v1", "status": "healthy", "latency": "80ms", "tokenUsage": "4,120,450"},
			{"id": "groq-mixtral", "name": "Groq Mixtral 8x7B", "model": "mixtral-8x7b-32768", "endpoint": "https://api.groq.com/openai/v1", "status": "healthy", "latency": "70ms", "tokenUsage": "2,450,120"},
			{"id": "deepseek-coder", "name": "DeepSeek Coder V2", "model": "deepseek-coder", "endpoint": "https://api.deepseek.com", "status": "healthy", "latency": "350ms", "tokenUsage": "8,120,450"},
			{"id": "cohere-command", "name": "Cohere Command R+", "model": "command-r-plus", "endpoint": "https://api.cohere.ai/v1", "status": "healthy", "latency": "310ms", "tokenUsage": "210,450"},
			{"id": "ollama-local", "name": "Ollama Local Llama 3", "model": "llama3:8b", "endpoint": "http://localhost:11434", "status": "healthy", "latency": "120ms", "tokenUsage": "45,230"},
			{"id": "vllm-cluster", "name": "vLLM Cluster Codellama", "model": "codellama:34b", "endpoint": "http://vllm.internal:8000", "status": "degraded", "latency": "890ms", "tokenUsage": "67,120"},
		},
		"connectionHealth": map[string]interface{}{
			"k8s":  map[string]interface{}{"status": "healthy", "latency": "12ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"git":  map[string]interface{}{"status": "healthy", "latency": "45ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"cicd": map[string]interface{}{"status": "degraded", "latency": "230ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"ai":   map[string]interface{}{"status": "healthy", "latency": "120ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
		},
	}

	// Register config as welcome message — every new client gets this immediately on connect
	hub.SetWelcomeMessage(adapthttp.WSMessage{Type: "config", Data: configData})

	healthTicker := time.NewTicker(10 * time.Second)
	defer healthTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-healthTicker.C:
			if hub.ClientCount() == 0 {
				continue
			}
			now := time.Now().UTC().Format(time.RFC3339)
			statuses := []string{"healthy", "healthy", "healthy", "degraded"}
			latencies := []string{"8ms", "12ms", "18ms", "45ms", "120ms", "230ms"}
			hub.Broadcast(adapthttp.WSMessage{
				Type: "connection.status",
				Data: map[string]interface{}{
					"k8s":  map[string]interface{}{"status": statuses[rng.Intn(len(statuses))], "latency": latencies[rng.Intn(len(latencies))], "lastCheck": now},
					"git":  map[string]interface{}{"status": "healthy", "latency": latencies[rng.Intn(len(latencies))], "lastCheck": now},
					"cicd": map[string]interface{}{"status": statuses[rng.Intn(len(statuses))], "latency": latencies[rng.Intn(len(latencies))], "lastCheck": now},
					"ai":   map[string]interface{}{"status": statuses[rng.Intn(len(statuses))], "latency": latencies[rng.Intn(len(latencies))], "lastCheck": now},
				},
			})
		}
	}
}

func randomHex(n int, rng *rand.Rand) string {
	const chars = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[rng.Intn(len(chars))]
	}
	return string(b)
}

type mockLLMClient struct {
	model string
}

func (m *mockLLMClient) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return &llm.CompletionResponse{
		Content:        fmt.Sprintf("[AI Response from %s] You asked: %s", m.model, req.Prompt),
		Model:          m.model,
		PromptTokens:   15,
		ResponseTokens: 25,
		Duration:       100 * time.Millisecond,
	}, nil
}

func (m *mockLLMClient) HealthCheck(ctx context.Context) error {
	return nil
}

type wsBridge struct {
	hub *adapthttp.WSHub
}

func (b *wsBridge) Broadcast(msgType string, data interface{}) {
	b.hub.Broadcast(adapthttp.WSMessage{Type: msgType, Data: data})
}
