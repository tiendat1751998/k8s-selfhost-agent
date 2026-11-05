package agent

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// SearchClusterLogs fans out search to all active compute hosts in parallel,
// applies unreachable host circuit cooldown, merges results fairly,
// and caps to the requested limit.
func (c *AgentLogClient) SearchClusterLogs(ctx context.Context, hosts []docker.ComputeHost, req SearchLogsRequest) ([]LogSearchResult, error) {
	if len(hosts) == 0 {
		return []LogSearchResult{}, nil
	}

	// Filter candidate hosts that are active, have valid endpoints, and are not in dead-node backoff
	var candidates []docker.ComputeHost
	for _, h := range hosts {
		endpoint := strings.TrimSpace(h.Endpoint)
		if endpoint == "" {
			continue
		}
		if strings.EqualFold(h.Status, "disconnected") || strings.EqualFold(h.Status, "disabled") || strings.EqualFold(h.Status, "down") {
			continue
		}
		if c != nil && c.backoff != nil && c.backoff.IsUnreachable(endpoint) {
			continue
		}
		candidates = append(candidates, h)
	}

	if len(candidates) == 0 {
		return []LogSearchResult{}, nil
	}

	timeout := c.clusterTimeout
	if timeout <= 0 || timeout > 1*time.Second {
		timeout = 1 * time.Second
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
				if results[i].NodeID == "" {
					results[i].NodeID = host.Endpoint
				}
				if results[i].NodeName == "" {
					results[i].NodeName = host.Name
				}
				if results[i].NodeName == "" {
					results[i].NodeName = host.Endpoint
				}
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()
		}()
	}

	wg.Wait()

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	return fairMergeSearchResults(allResults, limit), nil
}

// fairMergeSearchResults merges results across responding nodes fairly so that
// a single noisy node or service cannot monopolize the result buffer.
func fairMergeSearchResults(all []LogSearchResult, limit int) []LogSearchResult {
	if len(all) <= limit {
		sort.SliceStable(all, func(i, j int) bool {
			return all[i].Timestamp.Before(all[j].Timestamp)
		})
		return all
	}

	// Group by node+service
	groups := make(map[string][]LogSearchResult)
	var groupKeys []string
	for _, res := range all {
		k := res.NodeID + "/" + res.Service
		if _, exists := groups[k]; !exists {
			groupKeys = append(groupKeys, k)
		}
		groups[k] = append(groups[k], res)
	}

	// Sort each group descending (newest first)
	for _, k := range groupKeys {
		sort.SliceStable(groups[k], func(i, j int) bool {
			return groups[k][i].Timestamp.After(groups[k][j].Timestamp)
		})
	}

	// Sort group keys by latest timestamp in each group descending
	sort.SliceStable(groupKeys, func(i, j int) bool {
		return groups[groupKeys[i]][0].Timestamp.After(groups[groupKeys[j]][0].Timestamp)
	})

	var selected []LogSearchResult
	indices := make(map[string]int)

	for len(selected) < limit {
		addedThisRound := false
		for _, k := range groupKeys {
			idx := indices[k]
			if idx < len(groups[k]) {
				selected = append(selected, groups[k][idx])
				indices[k] = idx + 1
				addedThisRound = true
				if len(selected) == limit {
					break
				}
			}
		}
		if !addedThisRound {
			break
		}
	}

	// Sort ascending for chronological consistency
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Timestamp.Before(selected[j].Timestamp)
	})

	return selected
}

func getLogEntryNodeKey(e domainLogging.LogEntry) string {
	if e.Attributes != nil {
		if n := e.Attributes["node_id"]; n != "" {
			return n
		}
		if n := e.Attributes["node_name"]; n != "" {
			return n
		}
		if n := e.Attributes["node"]; n != "" {
			return n
		}
	}
	if e.PodName != "" {
		return e.PodName
	}
	if e.ContainerName != "" {
		return e.ContainerName
	}
	return "default"
}

// fairMergeLogEntries merges log entries across nodes fairly so that a single noisy node
// or service cannot monopolize the search result window when querying All Nodes.
func fairMergeLogEntries(entries []domainLogging.LogEntry, limit int) []domainLogging.LogEntry {
	if len(entries) <= limit || limit <= 0 {
		return entries
	}

	groups := make(map[string][]domainLogging.LogEntry)
	var groupKeys []string
	for _, e := range entries {
		k := getLogEntryNodeKey(e)
		if _, exists := groups[k]; !exists {
			groupKeys = append(groupKeys, k)
		}
		groups[k] = append(groups[k], e)
	}

	// Sort each group descending (newest first)
	for _, k := range groupKeys {
		sort.SliceStable(groups[k], func(i, j int) bool {
			return groups[k][i].Timestamp.After(groups[k][j].Timestamp)
		})
	}

	// Sort group keys by latest timestamp in each group descending
	sort.SliceStable(groupKeys, func(i, j int) bool {
		return groups[groupKeys[i]][0].Timestamp.After(groups[groupKeys[j]][0].Timestamp)
	})

	selected := make([]domainLogging.LogEntry, 0, limit)
	indices := make(map[string]int)

	for len(selected) < limit {
		addedThisRound := false
		for _, k := range groupKeys {
			idx := indices[k]
			if idx < len(groups[k]) {
				selected = append(selected, groups[k][idx])
				indices[k] = idx + 1
				addedThisRound = true
				if len(selected) == limit {
					break
				}
			}
		}
		if !addedThisRound {
			break
		}
	}

	// Sort descending (newest first)
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Timestamp.After(selected[j].Timestamp)
	})

	return selected
}
