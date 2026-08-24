package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// SearchLogsRequest represents parameters for distributed log search.
type SearchLogsRequest struct {
	Query string `json:"query"`
	App   string `json:"app,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	Level string `json:"level,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// LogSearchResult represents a single search match across nodes.
type LogSearchResult struct {
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"node_id,omitempty"`
	NodeName  string    `json:"node_name,omitempty"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Level     string    `json:"level,omitempty"`
	Raw       string    `json:"raw,omitempty"`
}

// LogClientInterface defines the interface for interacting with node agent log endpoints.
type LogClientInterface interface {
	GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string) (string, error)
	SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req SearchLogsRequest) ([]LogSearchResult, error)
	SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req SearchLogsRequest) ([]LogSearchResult, error)
}

// AgentLogClient implements communication with distributed k8s-agents (:9100).
type AgentLogClient struct {
	httpClient     *http.Client
	clusterTimeout time.Duration
	nodeTimeout    time.Duration
}

// AgentLogClientOption configures AgentLogClient.
type AgentLogClientOption func(*AgentLogClient)

// WithHTTPClient overrides the HTTP client.
func WithHTTPClient(client *http.Client) AgentLogClientOption {
	return func(c *AgentLogClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithClusterTimeout sets timeout for parallel scatter-gather cluster queries.
func WithClusterTimeout(d time.Duration) AgentLogClientOption {
	return func(c *AgentLogClient) {
		if d > 0 {
			c.clusterTimeout = d
		}
	}
}

// WithNodeTimeout sets timeout for single node queries.
func WithNodeTimeout(d time.Duration) AgentLogClientOption {
	return func(c *AgentLogClient) {
		if d > 0 {
			c.nodeTimeout = d
		}
	}
}

// NewAgentLogClient creates a new AgentLogClient.
func NewAgentLogClient(opts ...AgentLogClientOption) *AgentLogClient {
	c := &AgentLogClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		clusterTimeout: 4 * time.Second,
		nodeTimeout:    5 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NormalizeHostEndpoint standardizes endpoint URLs (adds http:// scheme if omitted).
func NormalizeHostEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return ""
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if strings.HasPrefix(endpoint, "tcp://") {
			endpoint = "http://" + strings.TrimPrefix(endpoint, "tcp://")
		} else {
			endpoint = "http://" + endpoint
		}
	}
	return endpoint
}

// GetNodeLogs queries logs directly from a single node agent.
func (c *AgentLogClient) GetNodeLogs(ctx context.Context, hostEndpoint, authToken, app, tail, since, until, q, level string) (string, error) {
	baseURL := NormalizeHostEndpoint(hostEndpoint)
	if baseURL == "" {
		return "", fmt.Errorf("invalid or empty host endpoint: %q", hostEndpoint)
	}

	reqURL, err := url.Parse(baseURL + "/logs")
	if err != nil {
		return "", fmt.Errorf("parsing log url: %w", err)
	}

	query := reqURL.Query()
	if app != "" {
		query.Set("app", app)
	}
	if tail != "" {
		query.Set("tail", tail)
	}
	if since != "" {
		query.Set("since", since)
	}
	if until != "" {
		query.Set("until", until)
	}
	if q != "" {
		query.Set("q", q)
	}
	if level != "" {
		query.Set("level", level)
	}
	reqURL.RawQuery = query.Encode()

	reqCtx := ctx
	if c.nodeTimeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, c.nodeTimeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("creating http request: %w", err)
	}

	if authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("executing request to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("agent returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// If response is JSON structured logs, extract raw logs string if present
	var jsonResp struct {
		Logs  string   `json:"logs"`
		Lines []string `json:"lines"`
	}
	if err := json.Unmarshal(bodyBytes, &jsonResp); err == nil {
		if jsonResp.Logs != "" {
			return jsonResp.Logs, nil
		}
		if len(jsonResp.Lines) > 0 {
			return strings.Join(jsonResp.Lines, "\n"), nil
		}
	}

	return string(bodyBytes), nil
}

// SearchNodeLogs sends search query payload to a single node agent.
func (c *AgentLogClient) SearchNodeLogs(ctx context.Context, hostEndpoint, authToken string, req SearchLogsRequest) ([]LogSearchResult, error) {
	baseURL := NormalizeHostEndpoint(hostEndpoint)
	if baseURL == "" {
		return nil, fmt.Errorf("invalid or empty host endpoint: %q", hostEndpoint)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling search request: %w", err)
	}

	reqCtx := ctx
	if c.nodeTimeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, c.nodeTimeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/logs/search", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing search request to %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent search returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// Support multiple envelope formats: {"results": [...]}, {"data": [...]}, or direct [...]
	var envelope struct {
		Results []LogSearchResult `json:"results"`
		Data    []LogSearchResult `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &envelope); err == nil && (envelope.Results != nil || envelope.Data != nil) {
		if envelope.Results != nil {
			return envelope.Results, nil
		}
		return envelope.Data, nil
	}

	var direct []LogSearchResult
	if err := json.Unmarshal(bodyBytes, &direct); err == nil {
		return direct, nil
	}

	return []LogSearchResult{}, nil
}

// SearchClusterLogs fans out search to all active compute hosts in parallel,
// merges results chronologically, and caps to the requested limit.
func (c *AgentLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req SearchLogsRequest) ([]LogSearchResult, error) {
	if len(hosts) == 0 {
		return []LogSearchResult{}, nil
	}

	// Filter candidate hosts that are active and have valid endpoints
	var candidates []docker.ComputeHost
	for _, h := range hosts {
		endpoint := strings.TrimSpace(h.Endpoint)
		if endpoint == "" {
			continue
		}
		if strings.EqualFold(h.Status, "disconnected") || strings.EqualFold(h.Status, "disabled") || strings.EqualFold(h.Status, "down") {
			continue
		}
		candidates = append(candidates, h)
	}

	if len(candidates) == 0 {
		return []LogSearchResult{}, nil
	}

	timeout := c.clusterTimeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		mu         sync.Mutex
		allResults []LogSearchResult
		wg         sync.WaitGroup
	)

	for _, h := range candidates {
		host := h
		wg.Add(1)
		go func() {
			defer wg.Done()

			token := ""
			if host.Labels != nil {
				if t, ok := host.Labels["auth_token"]; ok && t != "" {
					token = t
				} else if t, ok := host.Labels["token"]; ok && t != "" {
					token = t
				}
			}

			results, err := c.SearchNodeLogs(timeoutCtx, host.Endpoint, token, req)
			if err != nil {
				return
			}

			for i := range results {
				if results[i].NodeID == "" {
					results[i].NodeID = host.ID
				}
				if results[i].NodeName == "" {
					results[i].NodeName = host.Name
				}
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Sort chronologically by timestamp (ascending)
	sort.SliceStable(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	// Cap to requested limit (default 100)
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	return allResults, nil
}
