package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// ErrorRadarGroup aggregates recurring error signatures discovered across the cluster.
type ErrorRadarGroup struct {
	Node          string    `json:"node,omitempty"`
	Service       string    `json:"service"`
	Level         string    `json:"level"`
	Fingerprint   string    `json:"fingerprint"`
	SampleMessage string    `json:"sample_message"`
	Count         int       `json:"count"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}

// SummarizeErrors executes scatter-gather blind error discovery across edge k8s-agents.
func (r *DistributedAgentLogRepo) SummarizeErrors(ctx context.Context, filter domainLogging.LogFilter) ([]ErrorRadarGroup, error) {
	var hosts []docker.ComputeHost
	if r.hostRepo != nil {
		if filter.TenantID != "" { hosts, _ = r.hostRepo.List(ctx, filter.TenantID) }
		if len(hosts) == 0 { hosts, _ = r.hostRepo.ListAll(ctx) }
	}

	targetNode := ""
	if filter.Attributes != nil {
		if n := strings.TrimSpace(filter.Attributes["node"]); n != "" {
			targetNode = n
		} else {
			targetNode = strings.TrimSpace(filter.Attributes["node_name"])
		}
	}
	if targetNode != "" {
		var filtered []docker.ComputeHost
		for _, h := range hosts {
			if h.ID == targetNode || strings.EqualFold(h.Name, targetNode) || h.Endpoint == targetNode {
				filtered = append(filtered, h)
			}
		}
		hosts = filtered
	} else if len(hosts) > 0 {
		var online []docker.ComputeHost
		for _, h := range hosts {
			if strings.EqualFold(h.Status, "connected") || strings.EqualFold(h.Status, "online") || strings.EqualFold(h.Status, "healthy") {
				online = append(online, h)
			}
		}
		if len(online) > 0 {
			hosts = online
		}
	}

	service := filter.ContainerName
	if service == "" { service = filter.PodName }
	levelOrError := string(filter.LogLevel)
	if levelOrError == "" { levelOrError = "error" }

	var sinceStr, untilStr string
	if !filter.StartTime.IsZero() { sinceStr = filter.StartTime.Format(time.RFC3339Nano) }
	if !filter.EndTime.IsZero() { untilStr = filter.EndTime.Format(time.RFC3339Nano) }
	limit := filter.Limit
	if limit <= 0 { limit = 100 }

	var mu sync.Mutex
	groupMap := make(map[string]*ErrorRadarGroup)

	addEntry := func(node, svc, lvl, msg string, ts time.Time) {
		if isNoiseService(svc) { return }
		normLvl := string(domainLogging.NormalizeLogLevel(lvl, msg))
		if !isErrorSeverity(normLvl) && !isErrorSeverity(lvl) {
			return
		}
		if normLvl == "" {
			normLvl = "error"
		}
		fp := fingerprintLogMessage(msg)
		key := fmt.Sprintf("%s|%s|%s|%s", node, svc, normLvl, fp)
		mu.Lock()
		defer mu.Unlock()
		if eg, exists := groupMap[key]; exists {
			eg.Count++
			if ts.Before(eg.FirstSeen) { eg.FirstSeen = ts }
			if ts.After(eg.LastSeen) { eg.LastSeen = ts }
		} else {
			groupMap[key] = &ErrorRadarGroup{
				Node: node, Service: svc, Level: normLvl, Fingerprint: fp,
				SampleMessage: msg, Count: 1, FirstSeen: ts, LastSeen: ts,
			}
		}
	}

	var candidates []docker.ComputeHost
	for _, h := range hosts {
		endpoint := strings.TrimSpace(h.Endpoint)
		if endpoint == "" {
			continue
		}
		if strings.EqualFold(h.Status, "disconnected") || strings.EqualFold(h.Status, "disabled") || strings.EqualFold(h.Status, "down") {
			continue
		}
		if r.logClient != nil && r.logClient.IsHostUnreachable(endpoint) {
			continue
		}
		candidates = append(candidates, h)
	}
	hosts = candidates

	var wg sync.WaitGroup
	client := r.radarHTTPCli
	if client == nil {
		client = &http.Client{
			Transport: NewFastTransport(),
			Timeout:   1 * time.Second,
		}
	}

	for _, h := range hosts {
		wg.Add(1)
		go func(host docker.ComputeHost) {
			defer wg.Done()
			token := ""
			if host.Labels != nil {
				if t, ok := host.Labels["auth_token"]; ok && t != "" {
					token = t
				} else if t, ok := host.Labels["token"]; ok && t != "" {
					token = t
				}
			}

			// Fetch native level=error entries AND an unfiltered recent sample
			// (Level: "", Limit up to edge search cap such as 2000, respecting filters)
			// in parallel per online node.
			fetchLimit := 2000
			if limit > fetchLimit {
				fetchLimit = limit
			}

			reqErr := SearchLogsRequest{
				App:           service,
				Level:         levelOrError,
				Since:         sinceStr,
				Until:         untilStr,
				Limit:         fetchLimit,
				Namespace:     filter.Namespace,
				PodName:       filter.PodName,
				ContainerName: filter.ContainerName,
				Query:         filter.SearchText,
			}
			reqAll := SearchLogsRequest{
				App:           service,
				Level:         "",
				Since:         sinceStr,
				Until:         untilStr,
				Limit:         fetchLimit,
				Namespace:     filter.Namespace,
				PodName:       filter.PodName,
				ContainerName: filter.ContainerName,
				Query:         filter.SearchText,
			}

			var (
				errEntries  []LogSearchResult
				allEntries  []LogSearchResult
				nodeQueryWg sync.WaitGroup
			)

			if r.logClient != nil {
				nodeQueryWg.Add(2)
				go func() {
					defer nodeQueryWg.Done()
					entries, err := r.logClient.SearchNodeLogs(ctx, host.Endpoint, token, reqErr)
					if err == nil {
						errEntries = entries
					}
				}()
				go func() {
					defer nodeQueryWg.Done()
					entries, err := r.logClient.SearchNodeLogs(ctx, host.Endpoint, token, reqAll)
					if err == nil {
						allEntries = entries
					}
				}()
				nodeQueryWg.Wait()
			}

			// Deduplicate entries across queries by timestamp + node + service + message
			seenEntries := make(map[string]struct{}, len(errEntries)+len(allEntries))
			hasEntries := false

			processEntry := func(en LogSearchResult) {
				key := fmt.Sprintf("%d|%s|%s|%s", en.Timestamp.UnixNano(), host.Name, en.Service, en.Message)
				if _, seen := seenEntries[key]; seen {
					return
				}
				seenEntries[key] = struct{}{}
				hasEntries = true
				addEntry(host.Name, en.Service, en.Level, en.Message, en.Timestamp)
			}

			for _, en := range errEntries {
				processEntry(en)
			}
			for _, en := range allEntries {
				processEntry(en)
			}

			// Fallback: if no entries returned from SearchNodeLogs, check GET /logs/errors endpoint
			// (supporting edge agents or mock test servers that only implement /logs/errors)
			if !hasEntries {
				baseURL := NormalizeHostEndpoint(host.Endpoint)
				if baseURL != "" {
					endpointURL := fmt.Sprintf("%s/logs/errors?service=%s&level=%s&since=%s&until=%s&limit=%s",
						baseURL, url.QueryEscape(service), url.QueryEscape(string(filter.LogLevel)),
						url.QueryEscape(sinceStr), url.QueryEscape(untilStr), strconv.Itoa(limit))
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
					if err == nil {
						if token != "" {
							req.Header.Set("Authorization", "Bearer "+token)
						}
						resp, err := client.Do(req)
						if err == nil && resp.StatusCode == http.StatusOK {
							var parsed struct {
								Groups []struct {
									Service, Level, Fingerprint, SampleMessage string
									Count                                      int
									FirstSeen, LastSeen                        time.Time
								} `json:"groups"`
							}
							decErr := json.NewDecoder(resp.Body).Decode(&parsed)
							_ = resp.Body.Close()
							if decErr == nil {
								mu.Lock()
								for _, g := range parsed.Groups {
									if isNoiseService(g.Service) {
										continue
									}
									gLevel := string(domainLogging.NormalizeLogLevel(g.Level, g.SampleMessage))
									if !isErrorSeverity(gLevel) && !isErrorSeverity(g.Level) {
										continue
									}
									fp := g.Fingerprint
									if fp == "" {
										fp = fingerprintLogMessage(g.SampleMessage)
									}
									key := fmt.Sprintf("%s|%s|%s|%s", host.Name, g.Service, gLevel, fp)
									if eg, exists := groupMap[key]; exists {
										eg.Count += g.Count
										if g.FirstSeen.Before(eg.FirstSeen) { eg.FirstSeen = g.FirstSeen }
										if g.LastSeen.After(eg.LastSeen) { eg.LastSeen = g.LastSeen }
									} else {
										groupMap[key] = &ErrorRadarGroup{
											Node: host.Name, Service: g.Service, Level: gLevel,
											Fingerprint: fp, SampleMessage: g.SampleMessage,
											Count: g.Count, FirstSeen: g.FirstSeen, LastSeen: g.LastSeen,
										}
									}
								}
								mu.Unlock()
							}
						} else if resp != nil {
							_ = resp.Body.Close()
						}
					}
				}
			}
		}(h)
	}
	wg.Wait()

	// In-memory logs scan fallback
	if r.memRepo != nil {
		memRes, _ := r.memRepo.QueryLogs(ctx, domainLogging.LogFilter{
			TenantID: filter.TenantID, ClusterID: filter.ClusterID, Namespace: filter.Namespace,
			PodName: filter.PodName, ContainerName: filter.ContainerName, LogLevel: "",
			StartTime: filter.StartTime, EndTime: filter.EndTime, Limit: 1000,
		})
		if memRes != nil {
			for _, en := range memRes.Entries {
				node := ""
				if en.Attributes != nil {
					node = en.Attributes["node_name"]
					if node == "" { node = en.Attributes["node_id"] }
				}
				svc := en.ContainerName
				if svc == "" { svc = en.PodName }
				addEntry(node, svc, string(en.LogLevel), en.Message, en.Timestamp)
			}
		}
	}

	result := make([]ErrorRadarGroup, 0, len(groupMap))
	for _, g := range groupMap { result = append(result, *g) }
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count { return result[i].Count > result[j].Count }
		return result[i].LastSeen.After(result[j].LastSeen)
	})
	if filter.Limit > 0 && len(result) > filter.Limit { result = result[:filter.Limit] }
	return result, nil
}

func isErrorSeverity(lvl string) bool {
	l := strings.ToLower(strings.TrimSpace(lvl))
	return l == "error" || l == "fatal" || l == "critical" || l == "panic" || l == "err"
}

func fingerprintLogMessage(msg string) string {
	return logengine.FingerprintError([]byte(msg))
}

func isNoiseService(svc string) bool {
	return logengine.IsNoiseService(svc)
}

