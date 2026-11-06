package agent

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	domainLogging "github.com/datdt/k8sselfhost/internal/domain/logging"
	docker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// isPlatformAdminContext checks if the caller holds the platform_admin role.
func isPlatformAdminContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return tenancy.UserRoleFromContext(ctx) == tenancy.PlatformAdminRole
}

// resolveCandidateHosts finds compute hosts for a query while preventing cross-tenant host laundering:
// 1. If caller is platform_admin, ListAll(ctx) is permitted.
// 2. If caller is NOT platform_admin:
//    - Must query by tenantID via hostRepo.List(ctx, tenantID).
//    - If hostRepo returns 0 hosts or an error, return empty slice immediately (NO FALLBACK to ListAll!).
func resolveCandidateHosts(ctx context.Context, hostRepo docker.ComputeHostRepository, tenantID string) ([]docker.ComputeHost, error) {
	if hostRepo == nil {
		return nil, nil
	}

	role := tenancy.UserRoleFromContext(ctx)
	isAdmin := role == tenancy.PlatformAdminRole

	if tenantID == "" {
		tenantID = tenancy.TenantIDFromContext(ctx)
	}

	if tenantID != "" {
		if isAdmin {
			hosts, err := hostRepo.List(ctx, tenantID)
			if err == nil && len(hosts) > 0 {
				return hosts, nil
			}
			return hostRepo.ListAll(ctx)
		}
		// Non-platform admin MUST only see their tenant's hosts.
		// NEVER fall back to ListAll!
		hosts, err := hostRepo.List(ctx, tenantID)
		if err != nil || len(hosts) == 0 {
			return []docker.ComputeHost{}, nil
		}
		return hosts, nil
	}

	// tenantID is empty:
	// Non-platform admin without a tenant sees no hosts.
	if role != "" && !isAdmin {
		return []docker.ComputeHost{}, nil
	}

	// Platform admin or internal/test context without roles:
	return hostRepo.ListAll(ctx)
}

// filterSecurityLogEntries filters log entries based on service request type and security policies:
// 1. Drops sensitive OS log sources unconditionally.
// 2. If journalctl alias is requested, keeps only host system logs (excluding sensitive OS sources).
// 3. If empty service ("All Apps") is requested, strips all host system logs to return pure application logs.
// 4. Zero tenant laundering: host system logs are never stamped with a non-admin tenant's ID.
func filterSecurityLogEntries(ctx context.Context, entries []domainLogging.LogEntry, requestedApp string, queryTenantID string) []domainLogging.LogEntry {
	filtered := make([]domainLogging.LogEntry, 0, len(entries))
	isJournalctl := domainLogging.IsJournalctlAlias(requestedApp)
	isAllApps := strings.TrimSpace(requestedApp) == ""
	isAdmin := isPlatformAdminContext(ctx)

	for _, e := range entries {
		if strings.TrimSpace(e.Message) == "-- No entries --" {
			continue
		}

		svc := e.ContainerName
		if svc == "" {
			svc = e.PodName
		}
		if svc == "" && e.Attributes != nil {
			svc = e.Attributes["service"]
		}

		// Drop sensitive OS log sources
		if domainLogging.IsSensitiveOSLogSource(svc, "", "") ||
			domainLogging.IsSensitiveOSLogSource(e.ContainerName, "", "") ||
			domainLogging.IsSensitiveOSLogSource(e.PodName, "", "") {
			continue
		}

		isHostLog := domainLogging.IsHostSystemLog(e.Namespace, svc) ||
			domainLogging.IsHostSystemLog(e.Namespace, e.ContainerName) ||
			domainLogging.IsHostSystemLog(e.Namespace, e.PodName)

		if isJournalctl {
			if !isHostLog {
				continue
			}
		} else if isAllApps {
			if isHostLog {
				continue
			}
		}

		if isHostLog {
			if !isAdmin {
				e.TenantID = ""
			}
			if e.Namespace == "" {
				e.Namespace = domainLogging.HostNamespace
			}
		}

		filtered = append(filtered, e)
	}
	return filtered
}

// sanitizeCatalogServices cleans service list and node maps:
// 1. Strips sensitive OS sources and individual host system daemon unit names.
// 2. If caller is platform_admin, adds "journalctl" once per online node and once in global list.
func sanitizeCatalogServices(ctx context.Context, byNode map[string][]string, onlineNodeKeys []string) ([]string, map[string][]string) {
	admin := isPlatformAdminContext(ctx)
	cleanedByNode := make(map[string][]string, len(byNode))
	serviceSet := make(map[string]struct{})

	onlineSet := make(map[string]struct{}, len(onlineNodeKeys))
	for _, k := range onlineNodeKeys {
		onlineSet[k] = struct{}{}
	}

	for node, svcs := range byNode {
		nodeCleaned := make([]string, 0, len(svcs))
		for _, s := range svcs {
			s = strings.TrimSpace(s)
			if s == "" || domainLogging.IsSensitiveOSLogSource(s, "", "") || domainLogging.IsHostSystemLog("", s) {
				continue
			}
			if domainLogging.IsJournalctlAlias(s) {
				continue
			}
			nodeCleaned = append(nodeCleaned, s)
			serviceSet[s] = struct{}{}
		}

		// Only online hosts get "journalctl" when caller is platform_admin
		if admin {
			if _, isOnline := onlineSet[node]; isOnline {
				nodeCleaned = append(nodeCleaned, "journalctl")
				serviceSet["journalctl"] = struct{}{}
			}
		}
		sort.Strings(nodeCleaned)
		cleanedByNode[node] = nodeCleaned
	}

	resultServices := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		resultServices = append(resultServices, s)
	}
	sort.Strings(resultServices)

	return resultServices, cleanedByNode
}

// mapClusterResultsToLogEntries transforms agent search results into domain LogEntry models.
func mapClusterResultsToLogEntries(results []LogSearchResult, filter domainLogging.LogFilter) []domainLogging.LogEntry {
	entries := make([]domainLogging.LogEntry, 0, len(results))
	for _, res := range results {
		lvl := domainLogging.NormalizeLogLevel(res.Level, res.Message)
		ns := res.Namespace
		if ns == "" {
			ns = filter.Namespace
		}
		pod := res.PodName
		if pod == "" {
			pod = res.Service
		}
		cont := res.ContainerName
		if cont == "" {
			cont = res.Service
		}
		stream := res.Stream
		if stream == "" {
			stream = "stdout"
		}
		entries = append(entries, domainLogging.LogEntry{
			Timestamp:     res.Timestamp,
			TenantID:      filter.TenantID,
			ClusterID:     filter.ClusterID,
			Namespace:     ns,
			PodName:       pod,
			ContainerName: cont,
			Stream:        stream,
			LogLevel:      lvl,
			Message:       res.Message,
			Attributes: map[string]string{
				"node_id":   res.NodeID,
				"node_name": res.NodeName,
				"raw":       res.Raw,
				"namespace": ns,
				"pod":       pod,
				"container": cont,
				"stream":    stream,
			},
		})
	}
	return entries
}

// fetchPureAppNames discovers pure user application/container names on candidate hosts,
// filtering out host system daemons, journalctl, and sensitive OS log sources.
// Results are sorted and capped to max 12 unique names.
func (r *DistributedAgentLogRepo) fetchPureAppNames(ctx context.Context, hosts []docker.ComputeHost) []string {
	if r.logClient == nil || len(hosts) == 0 {
		return nil
	}

	var candidates []docker.ComputeHost
	for _, h := range hosts {
		endpoint := strings.TrimSpace(h.Endpoint)
		if endpoint == "" {
			continue
		}
		if strings.EqualFold(h.Status, "disconnected") || strings.EqualFold(h.Status, "disabled") ||
			strings.EqualFold(h.Status, "down") || strings.EqualFold(h.Status, "offline") ||
			strings.EqualFold(h.Status, "stopped") || strings.EqualFold(h.Status, "unhealthy") {
			continue
		}
		if r.logClient.IsHostUnreachable(endpoint) {
			continue
		}
		candidates = append(candidates, h)
	}
	if len(candidates) == 0 {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		apps = make(map[string]struct{})
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

			if len(services) == 0 {
				if recentLogs, sErr := r.logClient.SearchNodeLogs(timeoutCtx, host.Endpoint, token, SearchLogsRequest{Limit: 20}); sErr == nil {
					for _, l := range recentLogs {
						if s := strings.TrimSpace(l.Service); s != "" {
							services = append(services, s)
						}
					}
				}
			}

			mu.Lock()
			for _, s := range services {
				s = strings.TrimSpace(s)
				if s == "" || domainLogging.IsSensitiveOSLogSource(s, "", "") ||
					domainLogging.IsHostSystemLog("", s) || domainLogging.IsJournalctlAlias(s) {
					continue
				}
				apps[s] = struct{}{}
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(apps) == 0 {
		return nil
	}

	res := make([]string, 0, len(apps))
	for a := range apps {
		res = append(res, a)
	}
	sort.Strings(res)
	if len(res) > 12 {
		res = res[:12]
	}
	return res
}

// backfillPureAppLogs queries logs for specific pure app containers concurrently when
// an unfiltered "All Apps" query yielded fewer than limit entries due to noisy host daemons.
func (r *DistributedAgentLogRepo) backfillPureAppLogs(
	ctx context.Context,
	hosts []docker.ComputeHost,
	baseReq SearchLogsRequest,
	filter domainLogging.LogFilter,
	existingCount int,
) []domainLogging.LogEntry {
	if r.logClient == nil || len(hosts) == 0 {
		return nil
	}

	pureApps := r.fetchPureAppNames(ctx, hosts)
	if len(pureApps) == 0 {
		return nil
	}

	perAppLimit := 50
	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		backfilled []domainLogging.LogEntry
		sem        = make(chan struct{}, 4)
	)

	for _, appName := range pureApps {
		app := appName
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			appReq := SearchLogsRequest{
				App:    app,
				Query:  baseReq.Query,
				Level:  baseReq.Level,
				Since:  baseReq.Since,
				Until:  baseReq.Until,
				Stream: baseReq.Stream,
				Limit:  perAppLimit,
			}
			if filter.Namespace != "" {
				appReq.Namespace = filter.Namespace
			}

			results, err := r.logClient.SearchClusterLogs(ctx, hosts, appReq)
			if err != nil || len(results) == 0 {
				return
			}

			entries := mapClusterResultsToLogEntries(results, filter)
			entries = filterSecurityLogEntries(ctx, entries, app, filter.TenantID)
			if len(entries) > 0 {
				mu.Lock()
				backfilled = append(backfilled, entries...)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return backfilled
}

