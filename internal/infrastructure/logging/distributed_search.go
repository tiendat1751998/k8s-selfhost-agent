package logging

import (
	"bytes"
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
)

// ComputeHostLister abstracts compute host retrieval for distributed log querying.
type ComputeHostLister interface {
	ListAll(ctx context.Context) ([]docker.ComputeHost, error)
}

// DistributedSearchRequest defines query parameters for scatter-gather search.
type DistributedSearchRequest struct {
	Query string `json:"query"`
	App   string `json:"app,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	Level string `json:"level,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// DistributedSearchResult aggregates logs gathered across k8s-agent nodes.
type DistributedSearchResult struct {
	Entries      []LogEntry `json:"entries"`
	Total        int        `json:"total"`
	NodesQueried int        `json:"nodes_queried"`
	NodesFailed  int        `json:"nodes_failed"`
}

type DistributedCoordinatorOption func(*DistributedCoordinator)

func WithMaxConcurrency(n int) DistributedCoordinatorOption {
	return func(c *DistributedCoordinator) {
		if n > 0 {
			c.maxConcurrency = n
		}
	}
}

func WithHTTPClient(client *http.Client) DistributedCoordinatorOption {
	return func(c *DistributedCoordinator) {
		if client != nil {
			c.client = client
		}
	}
}

// DistributedCoordinator orchestrates parallel scatter-gather searches across 1,000 nodes.
type DistributedCoordinator struct {
	client         *http.Client
	hostLister     ComputeHostLister
	maxConcurrency int
}

// NewDistributedCoordinator creates a new 1,000-node scatter-gather log coordinator.
func NewDistributedCoordinator(hostLister ComputeHostLister, opts ...DistributedCoordinatorOption) *DistributedCoordinator {
	tr := &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   1000 * time.Millisecond,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	c := &DistributedCoordinator{
		client:         &http.Client{Transport: tr, Timeout: 1500 * time.Millisecond},
		hostLister:     hostLister,
		maxConcurrency: 64,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type agentLogItem struct {
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
	Message   string    `json:"message"`
	Level     string    `json:"level,omitempty"`
	Raw       string    `json:"raw,omitempty"`
}

type agentSearchResp struct {
	Results []agentLogItem `json:"results"`
	Total   int            `json:"total"`
}

// Search executes scatter-gather query across all registered online k8s-agent hosts.
func (c *DistributedCoordinator) Search(ctx context.Context, req DistributedSearchRequest) (*DistributedSearchResult, error) {
	if c.hostLister == nil {
		return &DistributedSearchResult{Entries: []LogEntry{}}, nil
	}
	allHosts, err := c.hostLister.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing compute hosts: %w", err)
	}
	var targetHosts []docker.ComputeHost
	for _, h := range allHosts {
		if isAgentHost(h) && isOnlineHost(h) {
			targetHosts = append(targetHosts, h)
		}
	}
	if len(targetHosts) == 0 {
		return &DistributedSearchResult{Entries: []LogEntry{}}, nil
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}

	sem := make(chan struct{}, c.maxConcurrency)
	var wg sync.WaitGroup
	hostResults := make([][]LogEntry, len(targetHosts))
	var nodesSuccess, nodesFailed, totalEntries atomic.Int64

	for i, h := range targetHosts {
		wg.Add(1)
		go func(idx int, target docker.ComputeHost) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				nodesFailed.Add(1)
				return
			}
			entries, total, err := c.queryHost(ctx, target, req)
			if err != nil {
				nodesFailed.Add(1)
				return
			}
			nodesSuccess.Add(1)
			totalEntries.Add(int64(total))
			hostResults[idx] = entries
		}(i, h)
	}
	wg.Wait()

	return &DistributedSearchResult{
		Entries:      mergeResults(hostResults, limit),
		Total:        int(totalEntries.Load()),
		NodesQueried: int(nodesSuccess.Load() + nodesFailed.Load()),
		NodesFailed:  int(nodesFailed.Load()),
	}, nil
}

func (c *DistributedCoordinator) queryHost(ctx context.Context, host docker.ComputeHost, req DistributedSearchRequest) ([]LogEntry, int, error) {
	endpoint := host.Endpoint
	if endpoint == "" {
		return nil, 0, fmt.Errorf("empty endpoint for host %s", host.ID)
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	endpoint = strings.TrimRight(endpoint, "/") + "/logs/search"

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := extractHostToken(host); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, 0, fmt.Errorf("host %s returned status %d: %s", host.ID, resp.StatusCode, string(body))
	}

	var agentResp agentSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
		return nil, 0, err
	}

	entries := make([]LogEntry, 0, len(agentResp.Results))
	for _, r := range agentResp.Results {
		msg := r.Message
		if msg == "" && r.Raw != "" {
			msg = r.Raw
		}
		ts := r.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		lvl := r.Level
		if lvl == "" {
			lvl = "INFO"
		}
		entries = append(entries, LogEntry{
			Timestamp: ts,
			Service:   r.Service,
			Node:      host.Name,
			Level:     strings.ToUpper(lvl),
			Message:   msg,
			Stream:    "stdout",
		})
	}
	return entries, agentResp.Total, nil
}

// Streaming Min-Heap for K-way merge capped at limit
type heapItem struct {
	entry   LogEntry
	hostIdx int
	nextIdx int
}

type minLogHeap []heapItem

func (h minLogHeap) Len() int           { return len(h) }
func (h minLogHeap) Less(i, j int) bool { return h[i].entry.Timestamp.Before(h[j].entry.Timestamp) }
func (h minLogHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *minLogHeap) Push(x any)        { *h = append(*h, x.(heapItem)) }
func (h *minLogHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[:len(old)-1]
	return item
}

func mergeResults(hostResults [][]LogEntry, limit int) []LogEntry {
	if limit <= 0 {
		limit = 100
	}
	h := &minLogHeap{}
	heap.Init(h)

	total := 0
	for i, list := range hostResults {
		if len(list) > 0 {
			total += len(list)
			heap.Push(h, heapItem{entry: list[0], hostIdx: i, nextIdx: 0})
		}
	}
	capSize := limit
	if total < capSize {
		capSize = total
	}
	merged := make([]LogEntry, 0, capSize)
	for h.Len() > 0 && len(merged) < limit {
		item := heap.Pop(h).(heapItem)
		merged = append(merged, item.entry)
		nextIdx := item.nextIdx + 1
		if nextIdx < len(hostResults[item.hostIdx]) {
			heap.Push(h, heapItem{entry: hostResults[item.hostIdx][nextIdx], hostIdx: item.hostIdx, nextIdx: nextIdx})
		}
	}
	return merged
}

func isAgentHost(h docker.ComputeHost) bool {
	return strings.EqualFold(h.HostType, "agent") || h.HostType == ""
}

func isOnlineHost(h docker.ComputeHost) bool {
	st := strings.ToLower(strings.TrimSpace(h.Status))
	return st != "disconnected" && st != "offline" && st != "error" && st != "down"
}

func extractHostToken(h docker.ComputeHost) string {
	if h.Labels == nil {
		return ""
	}
	for k, v := range h.Labels {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "auth_token" || lk == "bearer_token" || lk == "token" || lk == "agent_token" || lk == "access_token" {
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}
