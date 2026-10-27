package metrics

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type httpStatsRaw struct {
	totalRequests int64
	errorRequests int64
}

// collectHTTPTPS gathers HTTP throughput and service stats from Traefik API.
func (c *TPSCollector) collectHTTPTPS(ctx context.Context, now time.Time) HTTPTPS {
	var httpTPS HTTPTPS

	type traefikOverview struct {
		HTTP struct {
			Routers struct {
				Total    int `json:"total"`
				Warnings int `json:"warnings"`
				Errors   int `json:"errors"`
			} `json:"routers"`
			Services struct {
				Total    int `json:"total"`
				Warnings int `json:"warnings"`
				Errors   int `json:"errors"`
			} `json:"services"`
			Middlewares struct {
				Total int `json:"total"`
			} `json:"middlewares"`
		} `json:"http"`
	}

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	var totalReqs int64
	var errorCount int64
	var activeConns int
	var queuedReqs int

	if c.traefikAPIURL != "" && !IsSelfServerURL(c.traefikAPIURL, 8080) {
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.traefikAPIURL+"/api/overview", nil)
		if err == nil {
			resp, doErr := client.Do(req)
			if doErr == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var ov traefikOverview
					if decodeErr := json.NewDecoder(resp.Body).Decode(&ov); decodeErr == nil {
						activeConns = ov.HTTP.Services.Total + ov.HTTP.Routers.Total
						// Note: ov.HTTP.Routers.Errors and ov.HTTP.Services.Errors represent configuration
						// syntax errors in Traefik's dynamic router/service definitions, NOT runtime HTTP 5xx errors.
						// They must not be treated as runtime HTTP request errors.
						queuedReqs = ov.HTTP.Routers.Warnings + ov.HTTP.Services.Warnings
					}
				}
			} else {
				c.logger.Debug("Traefik overview API request failed", zap.Error(doErr))
			}
		}

		// Also check services for active upstream servers
		srvReqCtx, srvCancel := context.WithTimeout(ctx, 2*time.Second)
		defer srvCancel()
		srvReq, srvErr := http.NewRequestWithContext(srvReqCtx, http.MethodGet, c.traefikAPIURL+"/api/http/services", nil)
		if srvErr == nil {
			srvResp, srvDoErr := client.Do(srvReq)
			if srvDoErr == nil {
				defer srvResp.Body.Close()
				if srvResp.StatusCode == http.StatusOK {
					type traefikServiceItem struct {
						Status       string            `json:"status"`
						ServerStatus map[string]string `json:"serverStatus"`
					}
					var services []traefikServiceItem
					if err := json.NewDecoder(srvResp.Body).Decode(&services); err == nil {
						serverCount := 0
						for _, s := range services {
							for _, state := range s.ServerStatus {
								if strings.EqualFold(state, "UP") {
									serverCount++
								}
							}
						}
						if serverCount > 0 {
							activeConns = serverCount
						}
					}
				}
			}
		}
	}

	if c.requestCountFn != nil {
		totalReqs = c.requestCountFn()
	}

	c.mu.Lock()
	if !c.prevHTTPTime.IsZero() {
		elapsed := now.Sub(c.prevHTTPTime).Seconds()
		if elapsed > 0 {
			if totalReqs > 0 {
				reqDelta := safeDeltaInt64(totalReqs, c.prevHTTPStats.totalRequests)
				httpTPS.RequestsPerSec = math.Round((float64(reqDelta)/elapsed)*100) / 100

				// Report ErrorRate only if real runtime HTTP 5xx responses are measured
				if errorCount > 0 {
					errDelta := safeDeltaInt64(errorCount, c.prevHTTPStats.errorRequests)
					if reqDelta > 0 && errDelta > 0 {
						httpTPS.ErrorRate = math.Round((float64(errDelta)/float64(reqDelta))*10000) / 100
					}
				}
			}
		}
	}
	c.prevHTTPStats = httpStatsRaw{
		totalRequests: totalReqs,
		errorRequests: errorCount,
	}
	c.prevHTTPTime = now
	c.mu.Unlock()

	httpTPS.ActiveConnections = activeConns
	httpTPS.QueuedRequests = queuedReqs
	httpTPS.TotalRequests = totalReqs
	return httpTPS
}

// DeriveTraefikURL parses the Docker host address or returns a configured HTTP URL for Traefik API.
// When dockerHost is empty (or a local unix socket) and TRAEFIK_API_URL is unset, it returns "" to prevent self-hammering port 8080.
func DeriveTraefikURL(dockerHost string) string {
	if envURL := strings.TrimSpace(os.Getenv("TRAEFIK_API_URL")); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}
	dockerHost = strings.TrimSpace(dockerHost)
	if dockerHost == "" || strings.HasPrefix(dockerHost, "unix://") {
		return ""
	}
	if strings.HasPrefix(dockerHost, "http://") || strings.HasPrefix(dockerHost, "https://") {
		return strings.TrimRight(dockerHost, "/")
	}
	if strings.HasPrefix(dockerHost, "tcp://") {
		trimmed := strings.TrimPrefix(dockerHost, "tcp://")
		host := trimmed
		if idx := strings.Index(trimmed, ":"); idx != -1 {
			host = trimmed[:idx]
		}
		if host == "" || host == "0.0.0.0" {
			host = "localhost"
		}
		return "http://" + host + ":8080"
	}
	return ""
}

// IsSelfServerURL checks if lbURL points to the server's own host and port (e.g., localhost:8080, 127.0.0.1:8080).
func IsSelfServerURL(lbURL string, serverPort int) bool {
	lbURL = strings.TrimSpace(lbURL)
	if lbURL == "" {
		return false
	}
	if serverPort <= 0 {
		serverPort = 8080
	}
	if !strings.Contains(lbURL, "://") {
		lbURL = "http://" + lbURL
	}
	u, err := url.Parse(lbURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	portStr := u.Port()
	if portStr == "" {
		if strings.EqualFold(u.Scheme, "https") {
			portStr = "443"
		} else {
			portStr = "80"
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	if port != serverPort {
		return false
	}
	h := strings.Trim(strings.ToLower(host), "[]")
	return h == "localhost" || h == "127.0.0.1" || h == "::1" || h == "0.0.0.0"
}
