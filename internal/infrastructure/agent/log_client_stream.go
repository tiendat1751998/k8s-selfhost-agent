package agent

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/netguard"
)

// StreamLogs connects to GET /logs/tail on the remote agent and invokes onLine for each streamed line.
// It uses a dedicated HTTP client with Timeout=0 to prevent premature termination of long-lived SSE connections.
// If /logs/tail returns 404, 503, or 400 (or when service is empty), it falls back to polling SearchNodeLogs.
func (c *AgentLogClient) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	baseURL := NormalizeHostEndpoint(agentURL)
	if baseURL == "" {
		return fmt.Errorf("invalid or empty agent url: %q", agentURL)
	}

	if _, err := netguard.ValidateURL(baseURL, "http", "https"); err != nil {
		return fmt.Errorf("validating agent url %q: %w", baseURL, err)
	}

	reqURL, err := url.Parse(baseURL + "/logs/tail")
	if err != nil {
		return fmt.Errorf("parsing tail log url: %w", err)
	}

	query := reqURL.Query()
	if service != "" {
		query.Set("service", service)
	}
	query.Set("tail", "100")
	reqURL.RawQuery = query.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating http request: %w", err)
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	if token := c.getAuthToken(); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	// Ensure streaming client has no timeout
	var transport http.RoundTripper
	if c.httpClient != nil && c.httpClient.Transport != nil {
		transport = c.httpClient.Transport
	}
	streamClient := &http.Client{
		Transport: transport,
		Timeout:   0,
	}

	resp, err := streamClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("executing stream request to %s: %w", baseURL, err)
	}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusBadRequest || (service == "" && resp.StatusCode != http.StatusOK) {
		_ = resp.Body.Close()
		return c.pollLogsFallback(ctx, baseURL, service, onLine)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return fmt.Errorf("agent returned HTTP %d for tail stream", resp.StatusCode)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 64*1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ":") {
			// Skip empty SSE delimiter or comments
			continue
		}

		// Handle SSE payload "data: <line>"
		if strings.HasPrefix(line, "data: ") {
			line = strings.TrimPrefix(line, "data: ")
		} else if strings.HasPrefix(line, "data:") {
			line = strings.TrimPrefix(line, "data:")
		}

		if onLine != nil {
			onLine(line)
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("reading tail stream: %w", err)
	}

	return ctx.Err()
}

func (c *AgentLogClient) pollLogsFallback(ctx context.Context, baseURL, service string, onLine func(line string)) error {
	formatLine := func(r LogSearchResult) string {
		if r.Raw != "" {
			return r.Raw
		}
		lvl := r.Level
		if lvl == "" {
			lvl = "INFO"
		}
		return fmt.Sprintf("%s [%s] [%s] %s", r.Timestamp.UTC().Format(time.RFC3339Nano), r.Service, strings.ToUpper(lvl), r.Message)
	}

	initialResults, err := c.SearchNodeLogs(ctx, baseURL, "", SearchLogsRequest{App: service, Limit: 100})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}

	var lastTimestamp time.Time
	seenKeys := make(map[string]struct{})

	emit := func(r LogSearchResult) {
		key := fmt.Sprintf("%d_%s_%s", r.Timestamp.UnixNano(), r.Service, r.Message)
		if _, seen := seenKeys[key]; seen {
			return
		}
		seenKeys[key] = struct{}{}
		if r.Timestamp.After(lastTimestamp) {
			lastTimestamp = r.Timestamp
		}
		if onLine != nil {
			onLine(formatLine(r))
		}
	}

	for _, r := range initialResults {
		emit(r)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			req := SearchLogsRequest{
				App:   service,
				Limit: 100,
			}
			if !lastTimestamp.IsZero() {
				req.Since = lastTimestamp.Add(1 * time.Nanosecond).UTC().Format(time.RFC3339Nano)
			}
			results, err := c.SearchNodeLogs(ctx, baseURL, "", req)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue
			}
			for _, r := range results {
				emit(r)
			}
			if len(seenKeys) > 10000 {
				seenKeys = make(map[string]struct{})
			}
		}
	}
}
