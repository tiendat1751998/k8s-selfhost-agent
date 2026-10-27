package http

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/alert"
	"github.com/datdt/k8sselfhost/internal/pkg/netguard"
)

// ChannelTestResult contains honest diagnostic output from testing an alert channel.
type ChannelTestResult struct {
	Status     string `json:"status"`
	LatencyMS  int64  `json:"latency_ms"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Message    string `json:"message"`
	Target     string `json:"target,omitempty"`
}

func testNotificationChannel(ctx context.Context, ch *alert.NotificationChannel) (*ChannelTestResult, error) {
	if ch == nil {
		return nil, fmt.Errorf("channel is nil")
	}

	chType := strings.ToLower(strings.TrimSpace(ch.Type))
	cfg := ch.Config
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	targetURL := ""
	for _, key := range []string{"webhook_url", "url", "endpoint"} {
		if v, ok := cfg[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				targetURL = strings.TrimSpace(s)
				break
			}
		}
	}

	guard := netguard.New(netguard.WithAllowPrivate(false))

	switch chType {
	case "slack", "webhook", "discord", "teams", "pagerduty", "generic":
		if targetURL == "" {
			return nil, fmt.Errorf("channel has no webhook_url configured")
		}

		parsed, err := guard.ValidateURL(targetURL, "http", "https")
		if err != nil {
			return nil, fmt.Errorf("invalid or forbidden webhook URL: %w", err)
		}
		host := parsed.Hostname()
		if strings.EqualFold(host, "localhost") {
			return nil, fmt.Errorf("webhook host is not permitted: %s", host)
		}
		if ip := net.ParseIP(host); ip != nil {
			if err := guard.ValidateIP(ip); err != nil {
				return nil, fmt.Errorf("webhook IP is not permitted: %w", err)
			}
		}

		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		reqBody := []byte(`{"text":"K8s Self-Host connectivity test probe","event":"test"}`)
		req, err := http.NewRequestWithContext(testCtx, http.MethodPost, targetURL, bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create probe request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "k8s-selfhost-alertmanager/1.0")

		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					h, p, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}
					if ip := net.ParseIP(h); ip != nil {
						if err := guard.ValidateIP(ip); err != nil {
							return nil, err
						}
					}
					dialer := &net.Dialer{Timeout: 3 * time.Second}
					return dialer.DialContext(ctx, network, net.JoinHostPort(h, p))
				},
			},
		}

		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return &ChannelTestResult{
				Status:    "failed",
				LatencyMS: latency,
				Message:   fmt.Sprintf("connection error: %v", err),
				Target:    targetURL,
			}, nil
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return &ChannelTestResult{
				Status:     "success",
				LatencyMS:  latency,
				HTTPStatus: resp.StatusCode,
				Message:    fmt.Sprintf("test alert delivered successfully (HTTP %d)", resp.StatusCode),
				Target:     targetURL,
			}, nil
		}

		return &ChannelTestResult{
			Status:     "failed",
			LatencyMS:  latency,
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("remote endpoint returned HTTP %d", resp.StatusCode),
			Target:     targetURL,
		}, nil

	case "email", "smtp":
		hostVal, _ := cfg["smtp_host"].(string)
		if hostVal == "" {
			hostVal, _ = cfg["host"].(string)
		}
		portVal := 25
		if p, ok := cfg["smtp_port"].(float64); ok && p > 0 {
			portVal = int(p)
		} else if pStr, ok := cfg["port"].(string); ok {
			if parsedP, err := strconv.Atoi(pStr); err == nil && parsedP > 0 {
				portVal = parsedP
			}
		}
		if hostVal == "" {
			return nil, fmt.Errorf("smtp host is not configured")
		}
		if ip := net.ParseIP(hostVal); ip != nil {
			if err := guard.ValidateIP(ip); err != nil {
				return nil, fmt.Errorf("smtp host is not permitted: %w", err)
			}
		}

		start := time.Now()
		dialer := &net.Dialer{Timeout: 3 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(hostVal, strconv.Itoa(portVal)))
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return &ChannelTestResult{
				Status:    "failed",
				LatencyMS: latency,
				Message:   fmt.Sprintf("SMTP dial error: %v", err),
				Target:    net.JoinHostPort(hostVal, strconv.Itoa(portVal)),
			}, nil
		}
		conn.Close()

		return &ChannelTestResult{
			Status:    "success",
			LatencyMS: latency,
			Message:   "SMTP server connection established",
			Target:    net.JoinHostPort(hostVal, strconv.Itoa(portVal)),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported channel type %q for connectivity testing", ch.Type)
	}
}
