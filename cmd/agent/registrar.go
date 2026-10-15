package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// AgentRegistrationPayload is sent via POST /api/v1/agents/register.
type AgentRegistrationPayload struct {
	NodeIP       string            `json:"node_ip"`
	Hostname     string            `json:"hostname"`
	Port         int               `json:"port"`
	AuthToken    string            `json:"auth_token,omitempty"`
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	Status       string            `json:"status"`
	RegisteredAt time.Time         `json:"registered_at"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RegistrarConfig configures Agent Registrar.
type RegistrarConfig struct {
	ControlPlaneURL string
	NodeIP          string
	Port            int
	AuthToken       string
	Interval        time.Duration
	Logger          *slog.Logger
	HTTPClient      *http.Client
}

// Registrar periodically registers the agent with the Control Plane.
type Registrar struct {
	cfg    RegistrarConfig
	logger *slog.Logger
	client *http.Client
}

// NewRegistrar creates a new Registrar.
func NewRegistrar(cfg RegistrarConfig) *Registrar {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}
	return &Registrar{
		cfg:    cfg,
		logger: cfg.Logger,
		client: cfg.HTTPClient,
	}
}

// Start begins periodic registration and heartbeats until ctx is done.
func (r *Registrar) Start(ctx context.Context) {
	if strings.TrimSpace(r.cfg.ControlPlaneURL) == "" {
		return
	}

	r.logger.Info("Starting agent auto-registration and heartbeat loop",
		slog.String("control_plane_url", r.cfg.ControlPlaneURL),
		slog.String("node_ip", r.cfg.NodeIP),
		slog.Int("port", r.cfg.Port),
	)

	// Immediate registration attempt on start
	_ = r.registerOnce(ctx)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Agent registrar stopped")
			return
		case <-ticker.C:
			_ = r.registerOnce(ctx)
		}
	}
}

func (r *Registrar) registerOnce(ctx context.Context) error {
	hostname, _ := os.Hostname()
	payload := AgentRegistrationPayload{
		NodeIP:       r.cfg.NodeIP,
		Hostname:     hostname,
		Port:         r.cfg.Port,
		AuthToken:    r.cfg.AuthToken,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Status:       "online",
		RegisteredAt: time.Now().UTC(),
		Capabilities: []string{"metrics", "logs", "k8s", "psi"},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling register payload: %w", err)
	}

	url := strings.TrimRight(r.cfg.ControlPlaneURL, "/") + "/api/v1/agents/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if r.cfg.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.AuthToken)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Warn("Agent registration heartbeat failed", slog.String("error", err.Error()))
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.logger.Warn("Agent registration received non-2xx status", slog.Int("status", resp.StatusCode))
		return fmt.Errorf("non-2xx response: %d", resp.StatusCode)
	}

	r.logger.Debug("Agent registration heartbeat succeeded", slog.Int("status", resp.StatusCode))
	return nil
}
