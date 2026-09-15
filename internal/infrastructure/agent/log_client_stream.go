package agent

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// StreamLogs connects to GET /logs/tail on the remote agent and invokes onLine for each streamed line.
// It uses a dedicated HTTP client with Timeout=0 to prevent premature termination of long-lived SSE connections.
func (c *AgentLogClient) StreamLogs(ctx context.Context, agentURL, service string, onLine func(line string)) error {
	baseURL := NormalizeHostEndpoint(agentURL)
	if baseURL == "" {
		return fmt.Errorf("invalid or empty agent url: %q", agentURL)
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent returned HTTP %d for tail stream", resp.StatusCode)
	}

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
