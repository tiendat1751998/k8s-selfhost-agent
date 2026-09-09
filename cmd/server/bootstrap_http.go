package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
)

func newHTTPServer(ctx context.Context, cfg *config.Config, infra *infrastructure, services *appServices, log *zap.Logger) *http.Server {
	if err := initializeWelcomeMessage(ctx, infra.pgClient.Pool(), services.wsHub); err != nil {
		log.Error("failed to initialize WebSocket welcome config message", zap.Error(err))
	}

	router := adapthttp.NewRouterWithWS(infra.health, services.wsHub, services.platformHandlers)

	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}
}

func initializeWelcomeMessage(ctx context.Context, db *pgxpool.Pool, hub *adapthttp.WSHub) error {
	// 1. Seed git_providers if empty
	var count int
	err := db.QueryRow(ctx, "SELECT COUNT(*) FROM git_providers").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting git_providers: %w", err)
	}
	if count == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO git_providers (id, type, organization, repo_count, sync_status) VALUES
			('11111111-1111-1111-1111-111111111111', 'github', 'datdt-corp', 47, 'synced'),
			('22222222-2222-2222-2222-222222222222', 'gitlab', 'datdt-infra', 23, 'synced')
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("seeding git_providers: %w", err)
		}
	}

	// 2. Seed cicd_integrations if empty
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM cicd_integrations").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting cicd_integrations: %w", err)
	}
	if count == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO cicd_integrations (id, name, provider, status, success_rate, last_log) VALUES
			('33333333-3333-3333-3333-333333333333', 'deploy-prod', 'GitHub Actions', 'active', 96.0, '✓ Build passed\n✓ Tests passed (142/142)\n✓ Image pushed: k8s-agent:v1.2.3\n✓ Deployed to prod-us-east\n✓ Health check OK'),
			('44444444-4444-4444-4444-444444444444', 'deploy-staging', 'GitHub Actions', 'active', 89.0, '✓ Build passed\n✓ Tests passed\n✓ Deployed to staging-1')
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("seeding cicd_integrations: %w", err)
		}
	}

	// 3. Seed ai_providers if empty
	err = db.QueryRow(ctx, "SELECT COUNT(*) FROM ai_providers").Scan(&count)
	if err != nil {
		return fmt.Errorf("counting ai_providers: %w", err)
	}
	if count == 0 {
		_, err = db.Exec(ctx, `
			INSERT INTO ai_providers (id, name, provider_type, model, endpoint, status, latency_ms) VALUES
			('55555555-5555-5555-5555-555555555555', 'openai-gpt4o', 'openai', 'gpt-4o', 'https://api.openai.com/v1', 'healthy', 340),
			('66666666-6666-6666-6666-666666666666', 'google-gemini-pro', 'google', 'gemini-1.5-pro', 'https://generativelanguage.googleapis.com', 'healthy', 290),
			('77777777-7777-7777-7777-777777777777', 'ollama-local', 'ollama', 'llama3:8b', 'http://localhost:11434', 'healthy', 120)
			ON CONFLICT DO NOTHING
		`)
		if err != nil {
			return fmt.Errorf("seeding ai_providers: %w", err)
		}
	}

	// 4. Query fleet_clusters
	rows, err := db.Query(ctx, "SELECT name, provider, status, nodes FROM fleet_clusters")
	if err != nil {
		return fmt.Errorf("querying fleet_clusters: %w", err)
	}
	defer rows.Close()

	var k8sData []map[string]interface{}
	for rows.Next() {
		var name, provider, status string
		var nodes int
		if err := rows.Scan(&name, &provider, &status, &nodes); err == nil {
			k8sData = append(k8sData, map[string]interface{}{
				"name":      name,
				"provider":  provider,
				"status":    status,
				"nodes":     nodes,
				"lastCheck": time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// 5. Query git_providers
	rows2, err := db.Query(ctx, "SELECT type, organization, repo_count, sync_status FROM git_providers")
	if err != nil {
		return fmt.Errorf("querying git_providers: %w", err)
	}
	defer rows2.Close()

	var gitData []map[string]interface{}
	for rows2.Next() {
		var providerType, organization, syncStatus string
		var repoCount int
		if err := rows2.Scan(&providerType, &organization, &repoCount, &syncStatus); err == nil {
			gitData = append(gitData, map[string]interface{}{
				"type":         providerType,
				"organization": organization,
				"repoCount":    repoCount,
				"syncStatus":   syncStatus,
				"lastWebhook":  time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// 6. Query cicd_integrations
	rows3, err := db.Query(ctx, "SELECT name, provider, status, success_rate, last_log FROM cicd_integrations")
	if err != nil {
		return fmt.Errorf("querying cicd_integrations: %w", err)
	}
	defer rows3.Close()

	var cicdData []map[string]interface{}
	for rows3.Next() {
		var name, provider, status, lastLog string
		var successRate float64
		if err := rows3.Scan(&name, &provider, &status, &successRate, &lastLog); err == nil {
			cicdData = append(cicdData, map[string]interface{}{
				"name":        name,
				"provider":    provider,
				"status":      status,
				"successRate": int(successRate),
				"lastLog":     lastLog,
				"lastRun":     time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	// 7. Query ai_providers
	rows4, err := db.Query(ctx, "SELECT name, provider_type, model, endpoint, status, latency_ms FROM ai_providers")
	if err != nil {
		return fmt.Errorf("querying ai_providers: %w", err)
	}
	defer rows4.Close()

	var aiData []map[string]interface{}
	for rows4.Next() {
		var name, providerType, model, endpoint, status string
		var latency int
		if err := rows4.Scan(&name, &providerType, &model, &endpoint, &status, &latency); err == nil {
			aiData = append(aiData, map[string]interface{}{
				"id":         name,
				"name":       name,
				"model":      model,
				"endpoint":   endpoint,
				"status":     status,
				"latency":    fmt.Sprintf("%dms", latency),
				"tokenUsage": "0",
			})
		}
	}

	configData := map[string]interface{}{
		"kubernetes":   k8sData,
		"gitProviders": gitData,
		"cicd":         cicdData,
		"aiProviders":  aiData,
		"connectionHealth": map[string]interface{}{
			"k8s":  map[string]interface{}{"status": "healthy", "latency": "12ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"git":  map[string]interface{}{"status": "healthy", "latency": "45ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"cicd": map[string]interface{}{"status": "healthy", "latency": "23ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
			"ai":   map[string]interface{}{"status": "healthy", "latency": "120ms", "lastCheck": time.Now().UTC().Format(time.RFC3339)},
		},
	}

	hub.SetWelcomeMessage(adapthttp.WSMessage{Type: "config", Data: configData})

	// Periodic health broadcasts
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if hub.ClientCount() == 0 {
					continue
				}
				now := time.Now().UTC().Format(time.RFC3339)
				hub.Broadcast(adapthttp.WSMessage{
					Type: "connection.status",
					Data: map[string]interface{}{
						"k8s":  map[string]interface{}{"status": "healthy", "latency": "14ms", "lastCheck": now},
						"git":  map[string]interface{}{"status": "healthy", "latency": "38ms", "lastCheck": now},
						"cicd": map[string]interface{}{"status": "healthy", "latency": "29ms", "lastCheck": now},
						"ai":   map[string]interface{}{"status": "healthy", "latency": "98ms", "lastCheck": now},
					},
				})
			}
		}
	}()

	return nil
}
