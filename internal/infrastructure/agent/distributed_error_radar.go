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
		fp := logengine.FingerprintError([]byte(msg))
		key := fmt.Sprintf("%s|%s|%s|%s", node, svc, lvl, fp)
		mu.Lock()
		defer mu.Unlock()
		if eg, exists := groupMap[key]; exists {
			eg.Count++
			if ts.Before(eg.FirstSeen) { eg.FirstSeen = ts }
			if ts.After(eg.LastSeen) { eg.LastSeen = ts }
		} else {
			groupMap[key] = &ErrorRadarGroup{
				Node: node, Service: svc, Level: lvl, Fingerprint: fp,
				SampleMessage: msg, Count: 1, FirstSeen: ts, LastSeen: ts,
			}
		}
	}

	var wg sync.WaitGroup
	client := &http.Client{Timeout: 5 * time.Second}

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
			baseURL := NormalizeHostEndpoint(host.Endpoint)
			if baseURL != "" {
				endpointURL := fmt.Sprintf("%s/logs/errors?service=%s&level=%s&since=%s&until=%s&limit=%s",
					baseURL, url.QueryEscape(service), url.QueryEscape(string(filter.LogLevel)),
					url.QueryEscape(sinceStr), url.QueryEscape(untilStr), strconv.Itoa(limit))
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
				if err == nil {
					if token != "" { req.Header.Set("Authorization", "Bearer "+token) }
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
								key := fmt.Sprintf("%s|%s|%s|%s", host.Name, g.Service, g.Level, g.Fingerprint)
								if eg, exists := groupMap[key]; exists {
									eg.Count += g.Count
									if g.FirstSeen.Before(eg.FirstSeen) { eg.FirstSeen = g.FirstSeen }
									if g.LastSeen.After(eg.LastSeen) { eg.LastSeen = g.LastSeen }
								} else {
									groupMap[key] = &ErrorRadarGroup{
										Node: host.Name, Service: g.Service, Level: g.Level,
										Fingerprint: g.Fingerprint, SampleMessage: g.SampleMessage,
										Count: g.Count, FirstSeen: g.FirstSeen, LastSeen: g.LastSeen,
									}
								}
							}
							mu.Unlock()
							return
						}
					} else if resp != nil {
						_ = resp.Body.Close()
					}
				}
			}

			// Fallback: search node logs directly and group with FingerprintError
			if r.logClient != nil {
				entries, err := r.logClient.SearchNodeLogs(ctx, host.Endpoint, token, SearchLogsRequest{
					App: service, Level: levelOrError, Since: sinceStr, Until: untilStr, Limit: 500,
				})
				if err == nil {
					for _, en := range entries { addEntry(host.Name, en.Service, en.Level, en.Message, en.Timestamp) }
				}
			}
		}(h)
	}
	wg.Wait()

	// In-memory logs scan fallback
	if r.memRepo != nil {
		memRes, _ := r.memRepo.QueryLogs(ctx, domainLogging.LogFilter{
			TenantID: filter.TenantID, ClusterID: filter.ClusterID, Namespace: filter.Namespace,
			PodName: filter.PodName, ContainerName: filter.ContainerName, LogLevel: domainLogging.LogLevel(levelOrError),
			StartTime: filter.StartTime, EndTime: filter.EndTime, Limit: 500,
		})
		if memRes != nil {
			for _, en := range memRes.Entries {
				node := ""
				if en.Attributes != nil {
					node = en.Attributes["node_name"]
					if node == "" { node = en.Attributes["node_id"] }
				}
				addEntry(node, en.ContainerName, string(en.LogLevel), en.Message, en.Timestamp)
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

func isNoiseService(svc string) bool {
	return logengine.IsNoiseService(svc)
}

