// Package main is a standalone development server for the K8S Self-Healing dashboard.
// It runs WITHOUT PostgreSQL, Redis, or NATS — perfect for frontend development.
// Usage: go run ./cmd/standalone
package main

import (
	"context"
	"fmt"
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
	usecaseAuth "github.com/datdt/k8sselfhost/internal/usecase/auth"
	usecaseDeployment "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseSearch "github.com/datdt/k8sselfhost/internal/usecase/search"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
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

// Unused mock simulation variables removed


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
	incRepo := postgres.NewIncidentRepo(pgClient)
	reportRepo := postgres.NewReportRepo(pgClient)
	prRepo := postgres.NewPRRepo(pgClient)
	costRepo := postgres.NewCostRepo(pgClient)
	backupRepo := postgres.NewBackupRepo(pgClient)

		defaultLLM, _ := registry.Default()
	bridge := adapthttp.NewWSBridge(wsHub)
	orchestrator := usecaseAgent.NewOrchestrator(agentRepo, defaultLLM, bridge)

	userRepo := postgres.NewUserRepo(pgClient)
	authUsecase := usecaseAuth.NewUsecase(userRepo)

	searchRepo := postgres.NewSearchRepo(pgClient)
	searchUsecase := usecaseSearch.NewUsecase(searchRepo)

	gitopsController := usecaseGitops.NewController(prRepo, incRepo)
	tenancyRepo := postgres.NewTenancyRepo(pgClient)

	platformHandlers := &adapthttp.PlatformHandlers{
		Dashboard:     adapthttp.NewHandler(incRepo, reportRepo, prRepo, nil, gitopsController),
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
		Auth:          adapthttp.NewAuthHandler(authUsecase),
		Search:        adapthttp.NewSearchHandler(searchUsecase),
		Cost:          adapthttp.NewCostHandler(costRepo),
		Backup:        adapthttp.NewBackupHandler(backupRepo, wsHub),
		Agents:        adapthttp.NewAgentHandler(agentRepo, orchestrator),
		Deployments:   adapthttp.NewDeploymentHandler(usecaseDeployment.NewUsecase(infraK8s.NewDeploymentRepo(k8sClient, dockerRepo, fleetRepo, clientManager))),
		Tenancy:       adapthttp.NewTenancyHandler(tenancyRepo),
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

// Unused simulation data loops generateOperationalData, generateEnterpriseData, and randomHex removed


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

// local wsBridge removed, using adapthttp.WSBridge

