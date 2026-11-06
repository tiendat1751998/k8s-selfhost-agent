package agent

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	infraLogging "github.com/datdt/k8sselfhost/internal/infrastructure/logging"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// DistributedAgentLogRepo implements domainLogging.LogRepository by scattering queries
// across edge k8s-agents running embedded logengine instances.
type DistributedAgentLogRepo struct {
	hostRepo     docker.ComputeHostRepository
	logClient    LogClientInterface
	memRepo      *infraLogging.MemoryLogRepo
	radarHTTPCli *http.Client
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
		radarHTTPCli: &http.Client{
			Transport: NewFastTransport(),
			Timeout:   1 * time.Second,
		},
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
		hosts, _ = resolveCandidateHosts(ctx, r.hostRepo, filter.TenantID)
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

	// Prepare remote search request
	searchReq := SearchLogsRequest{
		Query:  filter.SearchText,
		Level:  string(filter.LogLevel),
		Limit:  filter.Limit,
		Stream: filter.Stream,
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

	reqApp := filter.ContainerName
	if reqApp == "" {
		reqApp = filter.PodName
	}
	if reqApp == "" {
		reqApp = filter.ServiceName
	}

	if reqApp == "" || domainLogging.IsJournalctlAlias(reqApp) {
		if searchReq.Limit < 2000 {
			searchReq.Limit = 2000
		}
	}

	if domainLogging.IsJournalctlAlias(reqApp) {
		// When querying journalctl, query edge agent unfiltered so all host units return
		searchReq.App = ""
		searchReq.PodName = ""
		searchReq.ContainerName = ""
		searchReq.Namespace = ""
	} else if filter.Namespace != "" || filter.PodName != "" {
		searchReq.Namespace = filter.Namespace
		searchReq.PodName = filter.PodName
		searchReq.ContainerName = filter.ContainerName
	} else {
		// Non-K8s systemd/docker nodes: preserve App fallback
		searchReq.App = reqApp
	}

	var clusterEntries []domainLogging.LogEntry
	if len(hosts) > 0 && r.logClient != nil {
		clusterResults, err := r.logClient.SearchClusterLogs(ctx, hosts, searchReq)
		if err == nil {
			clusterEntries = mapClusterResultsToLogEntries(clusterResults, filter)
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
	allEntries = filterSecurityLogEntries(ctx, allEntries, reqApp, filter.TenantID)

	// If querying "All Apps" (reqApp == "") and host logs saturated edge query leaving < filter.Limit pure app entries,
	// discover pure app containers on hosts and backfill them concurrently.
	if reqApp == "" && r.logClient != nil && len(hosts) > 0 && len(allEntries) < filter.Limit {
		backfilled := r.backfillPureAppLogs(ctx, hosts, searchReq, filter, len(allEntries))
		if len(backfilled) > 0 {
			allEntries = append(allEntries, backfilled...)
		}
	}

	seen := make(map[string]bool, len(allEntries))
	deduped := make([]domainLogging.LogEntry, 0, len(allEntries))
	for _, e := range allEntries {
		key := fmt.Sprintf("%d|%s|%s|%s|%s", e.Timestamp.UnixNano(), e.Namespace, e.PodName, e.ContainerName, e.Message)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, e)
		}
	}

	totalCount := int64(len(deduped))
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	if targetNode == "" && len(deduped) > (limit+offset) {
		deduped = fairMergeLogEntries(deduped, limit+offset)
	} else {
		// Sort descending (newest first) before applying pagination
		sort.SliceStable(deduped, func(i, j int) bool {
			return deduped[i].Timestamp.After(deduped[j].Timestamp)
		})
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

	hasMore := (offset + len(paged)) < int(totalCount)
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
	targetLevel := filter.LogLevel
	histFilter.LogLevel = ""
	histFilter.Limit = 10000
	histFilter.Offset = 0

	searchResult, err := r.QueryLogs(ctx, histFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs for histogram: %w", err)
	}

	buckets := make(map[int64]*domainLogging.LogAggregationBucket)
	for _, entry := range searchResult.Entries {
		lvl := string(domainLogging.NormalizeLogLevel(string(entry.LogLevel), entry.Message))
		if lvl == "" {
			lvl = "info"
		}
		if targetLevel != "" && !strings.EqualFold(lvl, string(targetLevel)) {
			continue
		}
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

// ListServicesCatalog queries active compute hosts in parallel, returning the merged
// deduplicated service list and a per-node service map. Offline hosts are fast-failed.
func (r *DistributedAgentLogRepo) ListServicesCatalog(ctx context.Context) ([]string, map[string][]string, error) {
	var hosts []docker.ComputeHost
	if r.hostRepo != nil {
		var err error
		hosts, err = resolveCandidateHosts(ctx, r.hostRepo, tenancy.TenantIDFromContext(ctx))
		if err != nil {
			return []string{}, make(map[string][]string), nil
		}
	}

	byNode := make(map[string][]string, len(hosts))
	for _, h := range hosts {
		name := strings.TrimSpace(h.Name)
		if name == "" { name = strings.TrimSpace(h.ID) }
		if name != "" { byNode[name] = []string{} }
	}

	if len(hosts) == 0 || r.logClient == nil {
		return []string{}, byNode, nil
	}

	// Filter candidate hosts: must be online, have an endpoint, and not be in backoff cooldown
	var candidates []docker.ComputeHost
	onlineNodeKeys := make([]string, 0, len(hosts))
	for _, h := range hosts {
		endpoint := strings.TrimSpace(h.Endpoint)
		if endpoint == "" { continue }
		if strings.EqualFold(h.Status, "disconnected") || strings.EqualFold(h.Status, "disabled") ||
			strings.EqualFold(h.Status, "down") || strings.EqualFold(h.Status, "offline") ||
			strings.EqualFold(h.Status, "stopped") || strings.EqualFold(h.Status, "unhealthy") {
			continue
		}
		if r.logClient != nil && r.logClient.IsHostUnreachable(endpoint) { continue }
		candidates = append(candidates, h)
		hostKey := strings.TrimSpace(h.Name)
		if hostKey == "" { hostKey = strings.TrimSpace(h.ID) }
		if hostKey == "" { hostKey = strings.TrimSpace(h.Endpoint) }
		onlineNodeKeys = append(onlineNodeKeys, hostKey)
	}

	if len(candidates) == 0 {
		services, nodeMap := sanitizeCatalogServices(ctx, byNode, nil)
		return services, nodeMap, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var (
		mu sync.Mutex
		wg sync.WaitGroup
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
			if err != nil { return }

			// If edge agent returned empty, supplement from recent logs
			if len(services) == 0 && r.logClient != nil {
				if recentLogs, sErr := r.logClient.SearchNodeLogs(timeoutCtx, host.Endpoint, token, SearchLogsRequest{Limit: 20}); sErr == nil {
					for _, l := range recentLogs {
						if s := strings.TrimSpace(l.Service); s != "" { services = append(services, s) }
					}
				}
			}

			hostKey := strings.TrimSpace(host.Name)
			if hostKey == "" { hostKey = strings.TrimSpace(host.ID) }
			if hostKey == "" { hostKey = strings.TrimSpace(host.Endpoint) }

			hostSet := make(map[string]struct{}, len(services))
			cleaned := make([]string, 0, len(services))
			for _, s := range services {
				s = strings.TrimSpace(s)
				if s != "" {
					if _, seen := hostSet[s]; !seen {
						hostSet[s] = struct{}{}
						cleaned = append(cleaned, s)
					}
				}
			}
			sort.Strings(cleaned)

			mu.Lock()
			byNode[hostKey] = cleaned
			mu.Unlock()
		}()
	}

	wg.Wait()

	services, nodeMap := sanitizeCatalogServices(ctx, byNode, onlineNodeKeys)
	return services, nodeMap, nil
}

// ListServices queries active compute hosts via ListServicesCatalog,
// returning a merged, deduplicated, sorted list of unique service names.
func (r *DistributedAgentLogRepo) ListServices(ctx context.Context) ([]string, error) {
	allServices, _, err := r.ListServicesCatalog(ctx)
	return allServices, err
}
// QuerySurroundingContext delegates surrounding context queries to the underlying in-memory fallback.
func (r *DistributedAgentLogRepo) QuerySurroundingContext(
	ctx context.Context,
	tenantID, service string,
	timestamp time.Time,
	window int,
) ([]domainLogging.LogEntry, error) {
	return r.memRepo.QuerySurroundingContext(ctx, tenantID, service, timestamp, window)
}
