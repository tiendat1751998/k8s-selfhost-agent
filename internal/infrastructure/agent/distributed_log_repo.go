package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraLogging "github.com/datdt/k8sselfhost/internal/infrastructure/logging"
)

// DistributedAgentLogRepo implements domainLogging.LogRepository by scattering queries
// across edge k8s-agents running embedded logengine instances.
type DistributedAgentLogRepo struct {
	hostRepo  docker.ComputeHostRepository
	logClient LogClientInterface
	memRepo   *infraLogging.MemoryLogRepo
}

// NewDistributedAgentLogRepo initializes a distributed log repository.
func NewDistributedAgentLogRepo(
	hostRepo docker.ComputeHostRepository,
	logClient LogClientInterface,
	memRepos ...*infraLogging.MemoryLogRepo,
) *DistributedAgentLogRepo {
	if logClient == nil {
		logClient = NewAgentLogClient()
	}
	var mem *infraLogging.MemoryLogRepo
	if len(memRepos) > 0 && memRepos[0] != nil {
		mem = memRepos[0]
	} else {
		mem = infraLogging.NewMemoryLogRepo(10000)
	}

	return &DistributedAgentLogRepo{
		hostRepo:  hostRepo,
		logClient: logClient,
		memRepo:   mem,
	}
}

// IngestBatch writes incoming log entries to the local fallback in-memory ring buffer.
func (r *DistributedAgentLogRepo) IngestBatch(ctx context.Context, entries []domainLogging.LogEntry) error {
	return r.memRepo.IngestBatch(ctx, entries)
}

// TailLogs delegates live streaming subscriptions to the local in-memory log broker.
func (r *DistributedAgentLogRepo) TailLogs(ctx context.Context, filter domainLogging.LogFilter) (<-chan domainLogging.LogEntry, error) {
	return r.memRepo.TailLogs(ctx, filter)
}

// QueryLogs executes scatter-gather distributed search across all edge k8s-agents,
// converts results into domain log entries, merges with in-memory logs, deduplicates,
// sorts chronologically, and applies limit/offset pagination.
func (r *DistributedAgentLogRepo) QueryLogs(ctx context.Context, filter domainLogging.LogFilter) (*domainLogging.LogSearchResult, error) {
	var hosts []docker.ComputeHost
	if r.hostRepo != nil {
		var err error
		if filter.TenantID != "" {
			hosts, err = r.hostRepo.List(ctx, filter.TenantID)
		}
		if len(hosts) == 0 || err != nil {
			hosts, _ = r.hostRepo.ListAll(ctx)
		}
	}

	// When filter.Attributes["node"] or filter.Attributes["node_name"] is specified,
	// filter candidate hosts to ONLY query that specific node instead of all hosts.
	targetNode := ""
	if filter.Attributes != nil {
		if n := strings.TrimSpace(filter.Attributes["node"]); n != "" {
			targetNode = n
		} else if n := strings.TrimSpace(filter.Attributes["node_name"]); n != "" {
			targetNode = n
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
	}

	// Prepare remote search request
	searchReq := SearchLogsRequest{
		Query: filter.SearchText,
		App:   filter.ContainerName,
		Level: string(filter.LogLevel),
		Limit: filter.Limit,
	}
	if searchReq.App == "" {
		searchReq.App = filter.PodName
	}
	if !filter.StartTime.IsZero() {
		searchReq.Since = filter.StartTime.Format(time.RFC3339Nano)
	}
	if !filter.EndTime.IsZero() {
		searchReq.Until = filter.EndTime.Format(time.RFC3339Nano)
	}
	if searchReq.Limit <= 0 {
		searchReq.Limit = 100
	}
	if filter.Offset > 0 {
		searchReq.Limit += filter.Offset
	}

	var clusterEntries []domainLogging.LogEntry
	if len(hosts) > 0 && r.logClient != nil {
		clusterResults, err := r.logClient.SearchClusterLogs(ctx, hosts, searchReq)
		if err == nil {
			for _, res := range clusterResults {
				lvl, _ := domainLogging.ParseLogLevel(res.Level)
				if lvl == "" {
					lvl = domainLogging.LogLevelInfo
				}
				clusterEntries = append(clusterEntries, domainLogging.LogEntry{
					Timestamp:     res.Timestamp,
					TenantID:      filter.TenantID,
					ClusterID:     filter.ClusterID,
					Namespace:     filter.Namespace,
					PodName:       res.Service,
					ContainerName: res.Service,
					Stream:        "stdout",
					LogLevel:      lvl,
					Message:       res.Message,
					Attributes: map[string]string{
						"node_id":   res.NodeID,
						"node_name": res.NodeName,
						"raw":       res.Raw,
					},
				})
			}
		}
	}

	// Query local in-memory fallback
	var memEntries []domainLogging.LogEntry
	if r.memRepo != nil {
		memRes, _ := r.memRepo.QueryLogs(ctx, domainLogging.LogFilter{
			TenantID:      filter.TenantID,
			ClusterID:     filter.ClusterID,
			Namespace:     filter.Namespace,
			PodName:       filter.PodName,
			ContainerName: filter.ContainerName,
			Stream:        filter.Stream,
			LogLevel:      filter.LogLevel,
			SearchText:    filter.SearchText,
			StartTime:     filter.StartTime,
			EndTime:       filter.EndTime,
			Limit:         searchReq.Limit,
		})
		if memRes != nil {
			memEntries = memRes.Entries
		}
	}

	// Merge & deduplicate
	allEntries := make([]domainLogging.LogEntry, 0, len(clusterEntries)+len(memEntries))
	allEntries = append(allEntries, clusterEntries...)
	allEntries = append(allEntries, memEntries...)

	seen := make(map[string]bool, len(allEntries))
	deduped := make([]domainLogging.LogEntry, 0, len(allEntries))
	for _, e := range allEntries {
		key := fmt.Sprintf("%d|%s|%s", e.Timestamp.UnixNano(), e.ContainerName, e.Message)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, e)
		}
	}

	// Sort descending (newest first) before applying pagination
	sort.SliceStable(deduped, func(i, j int) bool {
		return deduped[i].Timestamp.After(deduped[j].Timestamp)
	})

	totalCount := int64(len(deduped))
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	var paged []domainLogging.LogEntry
	if offset >= len(deduped) {
		paged = []domainLogging.LogEntry{}
	} else {
		end := offset + limit
		if end > len(deduped) {
			end = len(deduped)
		}
		paged = deduped[offset:end]
	}

	// Re-sort the extracted page ascending so terminal displays in chronological order
	sort.SliceStable(paged, func(i, j int) bool {
		return paged[i].Timestamp.Before(paged[j].Timestamp)
	})

	hasMore := (offset + len(paged)) < len(deduped)
	return &domainLogging.LogSearchResult{
		Entries:    paged,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}, nil
}

// GetHistogram aggregates log counts into discrete time intervals grouped by severity level.
func (r *DistributedAgentLogRepo) GetHistogram(
	ctx context.Context,
	filter domainLogging.LogFilter,
	intervalSeconds int,
) ([]domainLogging.LogAggregationBucket, error) {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	interval := time.Duration(intervalSeconds) * time.Second

	histFilter := filter
	histFilter.Limit = 10000
	histFilter.Offset = 0

	searchResult, err := r.QueryLogs(ctx, histFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs for histogram: %w", err)
	}

	buckets := make(map[int64]*domainLogging.LogAggregationBucket)
	for _, entry := range searchResult.Entries {
		bucketTime := entry.Timestamp.Truncate(interval)
		key := bucketTime.Unix()
		b, exists := buckets[key]
		if !exists {
			b = &domainLogging.LogAggregationBucket{
				TimeBucket: bucketTime,
				LevelCount: make(map[string]uint64),
			}
			buckets[key] = b
		}
		b.TotalCount++
		lvl := strings.ToLower(string(entry.LogLevel))
		if lvl == "" {
			lvl = "info"
		}
		b.LevelCount[lvl]++
	}

	resultBuckets := make([]domainLogging.LogAggregationBucket, 0, len(buckets))
	for _, b := range buckets {
		resultBuckets = append(resultBuckets, *b)
	}
	sort.Slice(resultBuckets, func(i, j int) bool {
		return resultBuckets[i].TimeBucket.Before(resultBuckets[j].TimeBucket)
	})

	return resultBuckets, nil
}

// GetStatus checks connectivity and reports edge logengine health status.
func (r *DistributedAgentLogRepo) GetStatus(ctx context.Context) (*domainLogging.LogEngineStatus, error) {
	start := time.Now()
	status := "connected"
	var totalRecords int64

	if r.hostRepo != nil {
		hosts, err := r.hostRepo.ListAll(ctx)
		if err != nil || len(hosts) == 0 {
			status = "degraded"
		}
	}

	latency := float64(time.Since(start).Microseconds()) / 1000.0
	if r.memRepo != nil {
		if res, err := r.memRepo.QueryLogs(ctx, domainLogging.LogFilter{Limit: 1}); err == nil && res != nil {
			totalRecords = res.TotalCount
		}
	}

	return &domainLogging.LogEngineStatus{
		Engine:        "Distributed Edge LogEngine (k8s-agent)",
		Status:        status,
		LatencyMS:     latency,
		TotalRecords:  totalRecords,
		RetentionDays: 7,
	}, nil
}

// ListServices queries active compute hosts via logClient.GetNodeServices in parallel,
// merges the results, deduplicates them, and returns a sorted list of unique service names.
func (r *DistributedAgentLogRepo) ListServices(ctx context.Context) ([]string, error) {
	var hosts []docker.ComputeHost
	if r.hostRepo != nil {
		var err error
		hosts, err = r.hostRepo.ListAll(ctx)
		if err != nil {
			return []string{}, nil
		}
	}

	if len(hosts) == 0 || r.logClient == nil {
		return []string{}, nil
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
		return []string{}, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	var (
		mu         sync.Mutex
		serviceSet = make(map[string]struct{})
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

			services, err := r.logClient.GetNodeServices(timeoutCtx, host.Endpoint, token)
			if err != nil {
				return
			}

			mu.Lock()
			for _, s := range services {
				s = strings.TrimSpace(s)
				if s != "" {
					serviceSet[s] = struct{}{}
				}
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	result := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		result = append(result, s)
	}
	sort.Strings(result)

	return result, nil
}
