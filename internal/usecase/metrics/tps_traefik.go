package metrics

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
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

	if c.traefikAPIURL != "" {
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
						errorCount = int64(ov.HTTP.Routers.Errors + ov.HTTP.Services.Errors)
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
			}

			if totalReqs > 0 && errorCount > 0 {
				httpTPS.ErrorRate = math.Round((float64(errorCount)/float64(totalReqs))*10000) / 100
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

// DeriveTraefikURL parses the Docker host address or returns a default HTTP URL for Traefik API.
func DeriveTraefikURL(dockerHost string) string {
	dockerHost = strings.TrimSpace(dockerHost)
	if dockerHost == "" {
		return "http://localhost:8080"
	}
	if strings.HasPrefix(dockerHost, "http://") || strings.HasPrefix(dockerHost, "https://") {
		return dockerHost
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
	return "http://localhost:8080"
}
