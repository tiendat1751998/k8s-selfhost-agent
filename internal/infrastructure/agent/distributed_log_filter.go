package agent

import (
	"context"
	"sort"
	"strings"

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
